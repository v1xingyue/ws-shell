#!/usr/bin/env node
import { mkdir, readFile, rm, stat, writeFile } from "node:fs/promises";
import { spawn } from "node:child_process";
import { stdin as input, stdout as output } from "node:process";
import { homedir } from "node:os";
import path from "node:path";
import * as p from "@clack/prompts";

const defaultWsShellImage = "ghcr.io/v1xingyue/ws-shell:v1.8.alpine";

const vmImages = {
  alpine: "alpine:3.23",
  ubuntu: "ubuntu:24.04",
  debian: "debian:13-slim",
};
const shells = ["/bin/bash", "/bin/zsh", "/bin/sh"];
const toolChoices = {
  nodejs: "Node.js + npm",
  codex: "OpenAI Codex CLI",
  "claude-code": "Claude Code",
};

const { command, args } = parseCommand(process.argv.slice(2));
const scriptRoot = path.resolve(import.meta.dirname);
const workspaceRoot = process.cwd();
const stateRoot = path.join(homedir(), ".vercel-vm-factory");
const defaultsPath = path.join(stateRoot, "defaults.json");
const legacyDefaultsPath = path.join(scriptRoot, ".defaults.json");
const codeDefaults = {
  "vm-image": "alpine",
  from: defaultWsShellImage,
  shell: "/bin/sh",
};
const packagedDefaults = {
  ...(await readDefaults(legacyDefaultsPath)),
  ...codeDefaults,
};
let defaults = {
  ...packagedDefaults,
  ...(await readDefaults(defaultsPath)),
};
const colorEnabled = output.isTTY && !process.env.NO_COLOR;
const color = {
  dim: (text) => paint(text, "2"),
  cyan: (text) => paint(text, "36"),
  green: (text) => paint(text, "32"),
  yellow: (text) => paint(text, "33"),
  red: (text) => paint(text, "31"),
  bold: (text) => paint(text, "1"),
};

try {
  if (args.help || args.h || command === "help") {
    printHelp();
    process.exit(0);
  }
  if (!["create", "doctor"].includes(command)) {
    throw new Error(`Unknown command: ${command}. Run vercel-vm-factory help`);
  }
  await main();
} catch (error) {
  const message = error instanceof Error ? error.message : String(error);
  if (message.includes("scope does not exist")) {
    console.error(
      `\nScope not found. Leave scope empty, or set the real CLI slug with --scope.`,
    );
    console.error(`If it was saved before, edit or delete: ${defaultsPath}`);
  }
  console.error(message);
  process.exit(1);
}

async function main() {
  printHeader();
  defaults = await syncDefaults();
  await ensureVercelInstalled();
  await ensureVercelLogin();

  if (args.doctor || command === "doctor") {
    await doctor();
    return;
  }

  const vmImageName = await chooseVmImage(
    args["vm-image"] ?? args.base ?? defaults["vm-image"] ?? "alpine",
  );
  const vmImage = vmImages[vmImageName] ?? vmImageName;
  const scope = await optionalValue(
    "scope",
    "Vercel team/scope",
    defaults.scope,
  );
  const project = await value(
    "project",
    "Vercel project name",
    defaults.project ?? `ws-shell-${vmImageName}`,
  );
  const wsShellImage =
    args.from ??
    process.env.WS_SHELL_IMAGE ??
    defaults.from ??
    defaultWsShellImage;
  const shell = await chooseShell(args.shell ?? defaults.shell ?? "/bin/sh");
  const tools = await chooseTools(args.tools ?? defaults.tools ?? "");
  const prod = args.prod !== "false";
  const dryRun = Boolean(args["dry-run"]);
  const skipLink = Boolean(args["skip-link"]);
  const authMode = await chooseAuthMode(defaultAuthMode());

  const oauthRedirectUrl =
    args["redirect-url"] ??
    process.env.OAUTH_REDIRECT_URL ??
    defaults["redirect-url"] ??
    `https://${project}.vercel.app/auth/github/callback`;

  const appDir = path.join(
    workspaceRoot,
    ".vercel-vm-factory",
    ".generated",
    project,
  );
  const dockerfile = makeDockerfile({
    shell,
    tools,
    vmImage,
    vmImageName,
    wsShellImage,
  });

  await rm(appDir, { recursive: true, force: true });
  await mkdir(appDir, { recursive: true });
  await writeFile(path.join(appDir, "Dockerfile.vercel"), dockerfile);

  printSummary({
    "vm image": `${vmImageName} -> ${vmImage}`,
    project,
    scope: scope || "default",
    source: wsShellImage,
    shell,
    tools: tools || "none",
    auth: authMode,
    ...(usesGitHubAuth(authMode) ? { callback: oauthRedirectUrl } : {}),
    dockerfile: path.join(appDir, "Dockerfile.vercel"),
    target: prod ? "production" : "preview",
  });

  if (dryRun) {
    ok("dry run: skipped vercel deploy");
    return;
  }

  if (usesGitHubAuth(authMode)) printOAuthGuide(oauthRedirectUrl);

  const authUsername = usesBasicAuth(authMode)
    ? await secret(
        "auth-user",
        "Basic auth username",
        process.env.AUTH_USERNAME ?? defaults["auth-user"],
      )
    : "";
  const authPassword = usesBasicAuth(authMode)
    ? await secret(
        "auth-password",
        "Basic auth password",
        process.env.AUTH_PASSWORD ?? defaults["auth-password"],
      )
    : "";
  const githubClientId = usesGitHubAuth(authMode)
    ? await secret(
        "client-id",
        "GitHub client id",
        process.env.GITHUB_CLIENT_ID ?? defaults["client-id"],
      )
    : "";
  const githubClientSecret = usesGitHubAuth(authMode)
    ? await secret(
        "client-secret",
        "GitHub client secret",
        process.env.GITHUB_CLIENT_SECRET ?? defaults["client-secret"],
      )
    : "";
  const allowedUserIds = usesGitHubAuth(authMode)
    ? await secret(
        "github-userid",
        "Allowed GitHub numeric user id(s)",
        process.env.ALLOWED_USER_IDS ?? defaults["github-userid"],
      )
    : "";

  let activeScope = scope;
  const commonArgs = () => makeCommonArgs(activeScope);

  if (!skipLink) {
    step("Linking Vercel project");
    await withScopeFallback(activeScope, async () =>
      runNoUrl("vercel", [
        "link",
        "--yes",
        "--project",
        project,
        "--cwd",
        appDir,
        ...commonArgs(),
      ]),
    );
  } else {
    warn("skip-link enabled; using existing .vercel/project.json");
  }

  step("Setting project framework=container");
  await withScopeFallback(activeScope, async () =>
    setContainerFramework(appDir, commonArgs()),
  );

  const vercelArgs = ["deploy", appDir, "--yes", "--logs"];

  if (authUsername) vercelArgs.push("--env", `AUTH_USERNAME=${authUsername}`);
  if (authPassword) vercelArgs.push("--env", `AUTH_PASSWORD=${authPassword}`);
  if (usesGitHubAuth(authMode))
    vercelArgs.push("--env", `OAUTH_REDIRECT_URL=${oauthRedirectUrl}`);
  if (githubClientId)
    vercelArgs.push("--env", `GITHUB_CLIENT_ID=${githubClientId}`);
  if (githubClientSecret)
    vercelArgs.push("--env", `GITHUB_CLIENT_SECRET=${githubClientSecret}`);
  if (allowedUserIds)
    vercelArgs.push("--env", `ALLOWED_USER_IDS=${allowedUserIds}`);
  if (prod) vercelArgs.push("--prod");

  step("Deploying");
  const deploymentUrl = await withScopeFallback(activeScope, async () =>
    run("vercel", [...vercelArgs, ...commonArgs()]),
  );
  await writeDefaults(defaultsPath, {
    ...defaults,
    "vm-image": vmImageName,
    scope: activeScope || undefined,
    project,
    from: wsShellImage,
    shell,
    tools,
    "auth-mode": authMode,
    "auth-user": authUsername,
    "auth-password": authPassword,
    "client-id": githubClientId,
    "client-secret": githubClientSecret,
    "github-userid": allowedUserIds,
  });
  console.log(
    `\n${color.green("Deployment URL:")} ${color.bold(deploymentUrl)}`,
  );

  async function withScopeFallback(scopeValue, action) {
    try {
      return await action();
    } catch (error) {
      if (!scopeValue || !isMissingScope(error)) throw error;
      warn(`scope "${scopeValue}" not found; retrying with default Vercel scope`);
      activeScope = "";
      return action();
    }
  }
}

async function setContainerFramework(appDir, commonArgs) {
  const projectFile = path.join(appDir, ".vercel", "project.json");
  const projectConfig = JSON.parse(await readFile(projectFile, "utf8"));
  if (!projectConfig.projectId)
    throw new Error(`Missing projectId in ${projectFile}`);

  await runNoUrl("vercel", [
    "api",
    `/v10/projects/${projectConfig.projectId}`,
    "-X",
    "PATCH",
    "-F",
    "framework=container",
    ...commonArgs,
  ]);
}

async function ensureVercelLogin() {
  step("Checking Vercel login");
  try {
    const whoami = await runCapture("vercel", ["whoami"]);
    ok(`logged in as ${whoami.trim()}`);
  } catch {
    warn("Vercel CLI is not logged in. Starting vercel login...");
    await runNoUrl("vercel", ["login"]);
  }
}

async function ensureVercelInstalled() {
  step("Checking Vercel CLI");
  try {
    const version = await runCapture("vercel", ["--version"]);
    ok(version.split("\n").filter(Boolean).at(-1) || "vercel installed");
  } catch {
    await installVercelCli();
    const version = await runCapture("vercel", ["--version"]);
    ok(version.split("\n").filter(Boolean).at(-1) || "vercel installed");
  }
}

async function installVercelCli() {
  if (!input.isTTY)
    throw new Error(
      "Vercel CLI is not installed. Run this in a terminal to install it.",
    );

  const install = await choosePackageInstall();
  const answer = await promptResult(
    p.confirm({
      message: `Vercel CLI is not installed. Install with ${install.command} ${install.args.join(" ")}?`,
      initialValue: false,
    }),
  );

  if (!answer) throw new Error("Vercel CLI is required. Exiting.");

  step(`Installing Vercel CLI with ${install.command}`);
  await runNoUrl(install.command, install.args);
}

async function choosePackageInstall() {
  const installs = [
    { command: "pnpm", args: ["add", "-g", "vercel"] },
    { command: "npm", args: ["install", "-g", "vercel"] },
    { command: "yarn", args: ["global", "add", "vercel"] },
    { command: "bun", args: ["add", "-g", "vercel"] },
  ];

  for (const install of installs) {
    try {
      await runCapture(install.command, ["--version"]);
      return install;
    } catch {
      // Try the next package manager.
    }
  }

  throw new Error(
    "No package manager found. Install pnpm, npm, yarn, or bun first.",
  );
}

async function doctor() {
  console.log("");
  console.log(color.bold("Saved defaults"));
  printKeyValue("defaults file", defaultsPath);
  printKeyValue("vm image", defaults["vm-image"] || "not set");
  printKeyValue("project", defaults.project || "not set");
  printKeyValue("scope", defaults.scope || "not set");
  printKeyValue("source image", defaults.from || defaultWsShellImage);
  printKeyValue("shell", defaults.shell || "/bin/sh");
  printKeyValue("tools", defaults.tools || "none");
  printKeyValue("auth mode", defaults["auth-mode"] || "not set");
  printKeyValue(
    "auth user",
    defaults["auth-user"] ? mask(defaults["auth-user"]) : "not set",
  );
  printKeyValue(
    "auth password",
    defaults["auth-password"] ? mask(defaults["auth-password"]) : "not set",
  );
  printKeyValue(
    "client id",
    defaults["client-id"] ? mask(defaults["client-id"]) : "not set",
  );
  printKeyValue(
    "client secret",
    defaults["client-secret"] ? mask(defaults["client-secret"]) : "not set",
  );
  printKeyValue(
    "github userid",
    defaults["github-userid"] ? mask(defaults["github-userid"]) : "not set",
  );
}

function makeDockerfile({ shell, tools, vmImage, vmImageName, wsShellImage }) {
  const shellInstall = makeShellInstall({ shell, vmImageName });
  const ohMyZshInstall =
    path.basename(shell) === "zsh"
      ? `RUN RUNZSH=no CHSH=no KEEP_ZSHRC=no sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended \\
    && printf '%s\\n' \\
      'export ZSH="$HOME/.oh-my-zsh"' \\
      'ZSH_THEME="robbyrussell"' \\
      'plugins=(git)' \\
      'source "$ZSH/oh-my-zsh.sh"' \\
      > /root/.zshrc`
      : "";
  const toolInstall = makeToolInstall({ tools, vmImageName });
  const shellSetup = [shellInstall, ohMyZshInstall, toolInstall]
    .filter(Boolean)
    .join("\n");
  return `ARG WS_SHELL_IMAGE=${wsShellImage}
ARG VM_IMAGE=${vmImage}

FROM \${WS_SHELL_IMAGE} AS ws-shell
FROM \${VM_IMAGE} AS vm
${shellSetup ? `\n${shellSetup}\n` : ""}
# wsterm already embeds the web UI; runtime config comes from environment variables.
COPY --from=ws-shell /app/bin/wsterm /app/bin/wsterm

WORKDIR /app
ENV ENABLE_SSL=false
ENV HOME=/root
ENV SHELL=${shell}
EXPOSE 80
CMD ["/app/bin/wsterm","-bind",":80","-fork","${shell}"]
`;
}

function makeToolInstall({ tools, vmImageName }) {
  const selected = new Set(parseTools(tools));
  if (selected.has("codex") || selected.has("claude-code"))
    selected.add("nodejs");
  if (!selected.size) return "";

  const packages = [];
  if (selected.has("nodejs")) packages.push("nodejs", "npm");
  const npmPackages = [];
  if (selected.has("codex")) npmPackages.push("@openai/codex");
  if (selected.has("claude-code")) npmPackages.push("@anthropic-ai/claude-code");

  const installNode =
    packages.length && vmImageName === "alpine"
      ? `RUN apk add --no-cache ${packages.join(" ")}`
      : packages.length && (vmImageName === "ubuntu" || vmImageName === "debian")
        ? `RUN apt-get update \\
    && apt-get install -y --no-install-recommends ${packages.join(" ")} \\
    && rm -rf /var/lib/apt/lists/*`
        : packages.length
          ? `RUN if command -v apk >/dev/null 2>&1; then \\
      apk add --no-cache ${packages.join(" ")}; \\
    elif command -v apt-get >/dev/null 2>&1; then \\
      apt-get update \\
      && apt-get install -y --no-install-recommends ${packages.join(" ")} \\
      && rm -rf /var/lib/apt/lists/*; \\
    else \\
      echo "Cannot install nodejs/npm: unsupported VM image package manager" >&2; \\
      exit 1; \\
    fi`
          : "";

  const installCli = npmPackages.length
    ? `RUN npm install -g ${npmPackages.join(" ")}`
    : "";
  return [installNode, installCli].filter(Boolean).join("\n");
}

function makeShellInstall({ shell, vmImageName }) {
  const packageName = path.basename(shell);
  if (packageName === "sh") return "";
  const packages = packageName === "zsh" ? "zsh curl git" : packageName;

  if (vmImageName === "alpine")
    return `RUN apk add --no-cache ${packages}`;

  if (vmImageName === "ubuntu" || vmImageName === "debian")
    return `RUN apt-get update \\
    && apt-get install -y --no-install-recommends ${packages} \\
    && rm -rf /var/lib/apt/lists/*`;

  return `RUN if command -v ${shell} >/dev/null 2>&1; then \\
      true; \\
    elif command -v apk >/dev/null 2>&1; then \\
      apk add --no-cache ${packages}; \\
    elif command -v apt-get >/dev/null 2>&1; then \\
      apt-get update \\
      && apt-get install -y --no-install-recommends ${packages} \\
      && rm -rf /var/lib/apt/lists/*; \\
    else \\
      echo "Cannot install ${packageName}: unsupported VM image package manager" >&2; \\
      exit 1; \\
    fi`;
}

async function value(name, question, fallback) {
  const current = args[name] ?? fallback;
  if (args[name]) return args[name];
  if (!input.isTTY) return current || "";

  const answer = await askText(question, fallback);
  return answer || current || "";
}

async function secret(name, question, fallback) {
  if (args[name]) return args[name];
  if (!input.isTTY) return fallback || "";

  const placeholder = fallback ? mask(fallback) : "skip";
  const answer = await askSecret(question, placeholder);
  return answer || fallback || "";
}

async function optionalValue(name, question, fallback) {
  if (args[name] !== undefined) return args[name];
  if (!input.isTTY) return "";

  const answer = await askText(
    question,
    fallback ? `${fallback}; Enter to skip` : "skip",
  );
  return answer;
}

function defaultAuthMode() {
  if (args["auth-mode"]) return args["auth-mode"];
  const hasBasic = Boolean(
    args["auth-user"] ||
    args["auth-password"] ||
    process.env.AUTH_USERNAME ||
    process.env.AUTH_PASSWORD,
  );
  const hasGitHub = Boolean(
    args["client-id"] ||
    args["client-secret"] ||
    args["github-userid"] ||
    process.env.GITHUB_CLIENT_ID ||
    process.env.GITHUB_CLIENT_SECRET,
  );
  if (hasBasic && hasGitHub) return "both";
  if (hasBasic) return "basic";
  if (hasGitHub) return "github";
  return defaults["auth-mode"] || (input.isTTY ? "basic" : "none");
}

async function chooseAuthMode(fallback) {
  const modes = new Set(["basic", "github", "both", "none"]);
  if (args["auth-mode"]) {
    if (!modes.has(args["auth-mode"]))
      throw new Error("--auth-mode must be basic, github, both, or none");
    return args["auth-mode"];
  }
  if (!input.isTTY) return modes.has(fallback) ? fallback : "basic";

  return promptResult(
    p.select({
      message: "Authentication",
      initialValue: modes.has(fallback) ? fallback : "basic",
      options: [
        { value: "basic", label: "Basic", hint: "username/password" },
        { value: "github", label: "GitHub OAuth" },
        { value: "both", label: "Basic + GitHub OAuth" },
        { value: "none", label: "None", hint: "no app auth" },
      ],
    }),
  );
}

function usesBasicAuth(mode) {
  return mode === "basic" || mode === "both";
}

function usesGitHubAuth(mode) {
  return mode === "github" || mode === "both";
}

async function chooseVmImage(fallback) {
  if (args["vm-image"]) return args["vm-image"];
  if (args.base) return args.base;
  if (!input.isTTY) return fallback;

  const names = Object.keys(vmImages);
  const answer = await promptResult(
    p.select({
      message: "VM image",
      initialValue: vmImages[fallback] ? fallback : "custom",
      options: names
        .map((name) => ({ value: name, label: name, hint: vmImages[name] }))
        .concat([
          { value: "custom", label: "Custom", hint: "enter a full image name" },
        ]),
    }),
  );

  if (answer === "custom")
    return value(
      "custom-vm-image",
      "Custom VM image",
      vmImages[fallback] ? defaults["custom-vm-image"] : fallback,
    );
  return answer;
}

async function chooseShell(fallback) {
  if (args.shell) return args.shell;
  if (!input.isTTY) return fallback;

  return promptResult(
    p.select({
      message: "Shell",
      initialValue: shells.includes(fallback) ? fallback : "/bin/sh",
      options: shells.map((shell, index) => ({
        value: shell,
        label: shell,
        hint: index === shells.length - 1 ? "default" : undefined,
      })),
    }),
  );
}

async function chooseTools(fallback) {
  if (args.tools !== undefined) return parseTools(args.tools).join(",");
  if (!input.isTTY) return parseTools(fallback).join(",");

  const names = Object.keys(toolChoices);
  const selected = await promptResult(
    p.multiselect({
      message: "Preinstall tools",
      required: false,
      initialValues: parseTools(fallback),
      options: names.map((name) => ({
        value: name,
        label: name,
        hint: toolChoices[name],
      })),
    }),
  );

  return selected.join(",");
}

function parseTools(value) {
  const names = Object.keys(toolChoices);
  const selected = String(value || "")
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
    .flatMap((item) => {
      if (item === "0" || item === "none") return [];
      if (/^[1-3]$/.test(item)) return [names[Number(item) - 1]];
      return [item];
    })
    .filter((item) => names.includes(item));
  return [...new Set(selected)];
}

async function readDefaults(file) {
  try {
    return JSON.parse(await readFile(file, "utf8"));
  } catch {
    return {};
  }
}

async function syncDefaults() {
  const current = await readDefaults(defaultsPath);
  const codeMtime = Math.max(
    await readMtime(import.meta.filename),
    await readMtime(legacyDefaultsPath),
  );
  const homeMtime = await readMtime(defaultsPath);
  const latest =
    homeMtime > codeMtime
      ? { ...packagedDefaults, ...current }
      : { ...current, ...packagedDefaults };

  if (
    JSON.stringify(cleanDefaults(current)) !==
    JSON.stringify(cleanDefaults(latest))
  )
    await writeDefaults(defaultsPath, latest);
  return cleanDefaults(latest);
}

async function readMtime(file) {
  try {
    return (await stat(file)).mtimeMs;
  } catch {
    return 0;
  }
}

async function writeDefaults(file, data) {
  const clean = cleanDefaults(data);
  await mkdir(path.dirname(file), { recursive: true });
  await writeFile(file, `${JSON.stringify(clean, null, 2)}\n`);
}

function cleanDefaults(data) {
  const migrated = {
    ...data,
    "vm-image": data["vm-image"] ?? data.base,
    "custom-vm-image": data["custom-vm-image"] ?? data["custom-base"],
  };
  delete migrated.base;
  delete migrated["custom-base"];

  const clean = Object.fromEntries(
    Object.entries(migrated).filter(([, value]) => value),
  );
  return Object.fromEntries(
    Object.entries(clean).sort(([a], [b]) => a.localeCompare(b)),
  );
}

function mask(value) {
  if (value.length <= 8) return "********";
  return `${value.slice(0, 4)}...${value.slice(-4)}`;
}

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (!arg.startsWith("--")) continue;

    const [rawKey, rawValue] = arg.slice(2).split("=", 2);
    out[rawKey] =
      rawValue ?? (argv[i + 1]?.startsWith("--") ? true : argv[i + 1]) ?? true;
    if (rawValue === undefined && argv[i + 1] && !argv[i + 1].startsWith("--"))
      i += 1;
  }
  return out;
}

function parseCommand(argv) {
  const known = new Set(["create", "doctor", "help"]);
  const first = argv[0];
  if (first && !first.startsWith("--")) {
    return {
      command: known.has(first) ? first : first,
      args: parseArgs(argv.slice(1)),
    };
  }
  return { command: "create", args: parseArgs(argv) };
}

function makeCommonArgs(scope) {
  const commonArgs = [];
  if (args.token) commonArgs.push("--token", args.token);
  if (scope) commonArgs.push("--scope", scope);
  return commonArgs;
}

function isMissingScope(error) {
  return String(error?.message || error).includes("scope does not exist");
}

function run(command, commandArgs) {
  return new Promise((resolve, reject) => {
    let seenUrl = "";
    let text = "";
    const child = spawn(command, commandArgs, {
      stdio: ["inherit", "pipe", "pipe"],
    });

    child.stdout.on("data", (chunk) => {
      const chunkText = chunk.toString();
      text += chunkText;
      output.write(chunkText);
      seenUrl = findLastUrl(chunkText) || seenUrl;
    });

    child.stderr.on("data", (chunk) => {
      const chunkText = chunk.toString();
      text += chunkText;
      output.write(chunkText);
      seenUrl = findLastUrl(chunkText) || seenUrl;
    });

    child.on("error", reject);
    child.on("close", (code) => {
      if (code === 0 && seenUrl) resolve(seenUrl);
      else if (code === 0)
        reject(new Error("vercel finished but no deployment url was found"));
      else reject(new Error(text.trim() || `vercel exited with code ${code}`));
    });
  });
}

function runCapture(command, commandArgs) {
  return new Promise((resolve, reject) => {
    let text = "";
    const child = spawn(command, commandArgs, {
      stdio: ["ignore", "pipe", "pipe"],
    });
    child.stdout.on("data", (chunk) => {
      text += chunk.toString();
    });
    child.stderr.on("data", (chunk) => {
      text += chunk.toString();
    });
    child.on("error", reject);
    child.on("close", (code) => {
      if (code === 0) resolve(text.trim());
      else
        reject(new Error(text.trim() || `${command} exited with code ${code}`));
    });
  });
}

function runNoUrl(command, commandArgs) {
  return new Promise((resolve, reject) => {
    let text = "";
    const child = spawn(command, commandArgs, {
      stdio: ["inherit", "pipe", "pipe"],
    });
    child.stdout.on("data", (chunk) => {
      const chunkText = chunk.toString();
      text += chunkText;
      output.write(chunkText);
    });
    child.stderr.on("data", (chunk) => {
      const chunkText = chunk.toString();
      text += chunkText;
      output.write(chunkText);
    });
    child.on("error", reject);
    child.on("close", (code) => {
      if (code === 0) resolve();
      else reject(new Error(text.trim() || `${command} exited with code ${code}`));
    });
  });
}

function findLastUrl(text) {
  return text.match(/https:\/\/[^\s]+\.vercel\.app/g)?.at(-1) ?? "";
}

function printHeader() {
  if (input.isTTY) {
    p.intro("Vercel VM Factory");
    return;
  }
  console.log("");
  console.log(color.cyan(" __     ____  __   _____           _                   "));
  console.log(color.cyan(" \\ \\   / /  \\/  | |  ___|_ _  ___| |_ ___  _ __ _   _ "));
  console.log(color.cyan("  \\ \\ / /| |\\/| | | |_ / _` |/ __| __/ _ \\| '__| | | |"));
  console.log(color.cyan("   \\ V / | |  | | |  _| (_| | (__| || (_) | |  | |_| |"));
  console.log(color.cyan("    \\_/  |_|  |_| |_|  \\__,_|\\___|\\__\\___/|_|   \\__, |"));
  console.log(color.cyan("                                                 |___/ "));
  console.log(color.dim("        Vercel Container VM deployment helper"));
  console.log("");
}

function printSummary(items) {
  const entries = Object.entries(items);
  const keyWidth = Math.max(...entries.map(([key]) => key.length), 1);
  const valueWidth = Math.max(...entries.map(([, value]) => String(value).length), 1);
  const width = keyWidth + valueWidth + 5;

  console.log(color.bold("Deployment plan"));
  console.log(color.cyan(`+${"-".repeat(width)}+`));
  for (const [key, value] of Object.entries(items)) {
    const line = `${key.padEnd(keyWidth)}  ${String(value).padEnd(valueWidth)}`;
    console.log(`${color.cyan("|")} ${line} ${color.cyan("|")}`);
  }
  console.log(color.cyan(`+${"-".repeat(width)}+`));
  console.log("");
}

function printOAuthGuide(callbackUrl) {
  console.log(color.bold("GitHub OAuth"));
  console.log(
    `${color.yellow("!")} Set this callback URL in your GitHub OAuth App before deploying:`,
  );
  console.log(`  ${color.bold(callbackUrl)}`);
  console.log(
    color.dim(
      "  GitHub: Settings -> Developer settings -> OAuth Apps -> Authorization callback URL",
    ),
  );
  console.log(
    color.dim(
      "  User ID: open https://api.github.com/users/YOUR_LOGIN and copy the numeric id",
    ),
  );
  console.log("");
}

function printKeyValue(key, value) {
  console.log(`${color.dim(`${key.padEnd(14)} `)}${value}`);
}

async function askText(question, fallback = "") {
  return String(
    await promptResult(
      p.text({
        message: question,
        placeholder: fallback || undefined,
      }),
    ),
  ).trim();
}

async function askSecret(question, fallback = "") {
  return String(
    await promptResult(
      p.password({
        message: question,
        mask: "*",
        placeholder: fallback || undefined,
      }),
    ),
  ).trim();
}

async function promptResult(resultPromise) {
  const result = await resultPromise;
  if (p.isCancel(result)) {
    p.cancel("Operation cancelled.");
    process.exit(0);
  }
  return result;
}

function step(text) {
  console.log(`${color.cyan("->")} ${text}`);
}

function ok(text) {
  console.log(`${color.green("OK")} ${text}`);
}

function warn(text) {
  console.log(`${color.yellow("!")} ${text}`);
}

function paint(text, code) {
  return colorEnabled ? `\u001b[${code}m${text}\u001b[0m` : text;
}

function printHelp() {
  printHeader();
  console.log(`Usage:
  vercel-vm-factory create
  vercel-vm-factory create --vm-image ubuntu --project x-shell
  vercel-vm-factory doctor
  npx vercel-vm-factory create

Options:
  --vm-image NAME      alpine, ubuntu, debian, or a custom VM image
  --base NAME          Alias for --vm-image
  --project NAME       Vercel project name
  --scope SLUG         Optional Vercel team/user scope slug
  --from IMAGE         Source image for /app/bin/wsterm
  --shell PATH         /bin/bash, /bin/zsh, or /bin/sh
  --tools LIST         nodejs,codex,claude-code
  --auth-mode MODE     basic, github, both, or none
  --auth-user VALUE    Username/password auth user
  --auth-password VAL  Username/password auth password
  --client-id VALUE    GitHub OAuth client id
  --client-secret VAL  GitHub OAuth client secret
  --github-userid VAL  Allowed GitHub numeric user id(s)
  --redirect-url URL   OAuth redirect URL
  --prod=false         Deploy preview instead of production
  --skip-link          Reuse existing generated .vercel/project.json
  --dry-run            Generate Dockerfile.vercel only
  --doctor             Check Vercel CLI/login and saved defaults
  --help               Show this help

GitHub OAuth:
  Callback URL must match the deployed project:
  https://PROJECT.vercel.app/auth/github/callback
`);
}
