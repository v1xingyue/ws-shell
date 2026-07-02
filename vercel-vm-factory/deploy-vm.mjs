#!/usr/bin/env node
import { mkdir, readFile, rm, stat, writeFile } from "node:fs/promises";
import { spawn } from "node:child_process";
import { createInterface } from "node:readline/promises";
import { stdin as input, stdout as output } from "node:process";
import { homedir } from "node:os";
import path from "node:path";

const defaultWsShellImage = "ghcr.io/v1xingyue/ws-shell:v1.8.alpine";

const vmImages = {
  alpine: "alpine:3.23",
  ubuntu: "ubuntu:24.04",
  debian: "debian:13-slim",
};

const { command, args } = parseCommand(process.argv.slice(2));
const scriptRoot = path.resolve(import.meta.dirname);
const workspaceRoot = process.cwd();
const stateRoot = path.join(homedir(), ".vercel-vm-factory");
const defaultsPath = path.join(stateRoot, "defaults.json");
const legacyDefaultsPath = path.join(scriptRoot, ".defaults.json");
const codeDefaults = { "vm-image": "alpine", from: defaultWsShellImage };
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
  const dockerfile = makeDockerfile({ vmImage, wsShellImage });

  await rm(appDir, { recursive: true, force: true });
  await mkdir(appDir, { recursive: true });
  await writeFile(path.join(appDir, "Dockerfile.vercel"), dockerfile);

  printSummary({
    "vm image": `${vmImageName} -> ${vmImage}`,
    project,
    scope: scope || "default",
    source: wsShellImage,
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

  await writeDefaults(defaultsPath, {
    ...defaults,
    "vm-image": vmImageName,
    scope: scope || undefined,
    project,
    from: wsShellImage,
    "auth-mode": authMode,
    "auth-user": authUsername,
    "auth-password": authPassword,
    "client-id": githubClientId,
    "client-secret": githubClientSecret,
    "github-userid": allowedUserIds,
  });

  const commonArgs = [];
  if (args.token) commonArgs.push("--token", args.token);
  if (scope) commonArgs.push("--scope", scope);

  if (!skipLink) {
    step("Linking Vercel project");
    await runNoUrl("vercel", [
      "link",
      "--yes",
      "--project",
      project,
      "--cwd",
      appDir,
      ...commonArgs,
    ]);
  } else {
    warn("skip-link enabled; using existing .vercel/project.json");
  }

  step("Setting project framework=container");
  await setContainerFramework(appDir, commonArgs);

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
  vercelArgs.push(...commonArgs);

  step("Deploying");
  const deploymentUrl = await run("vercel", vercelArgs);
  console.log(
    `\n${color.green("Deployment URL:")} ${color.bold(deploymentUrl)}`,
  );
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
  const rl = createInterface({ input, output });
  const answer = (
    await rl.question(
      `Vercel CLI is not installed. Install it with "${install.command} ${install.args.join(" ")}"? [y/N]: `,
    )
  )
    .trim()
    .toLowerCase();
  rl.close();

  if (answer !== "y" && answer !== "yes")
    throw new Error("Vercel CLI is required. Exiting.");

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

function makeDockerfile({ vmImage, wsShellImage }) {
  return `ARG WS_SHELL_IMAGE=${wsShellImage}
ARG VM_IMAGE=${vmImage}

FROM \${WS_SHELL_IMAGE} AS ws-shell
FROM \${VM_IMAGE} AS vm

# wsterm already embeds the web UI; runtime config comes from environment variables.
COPY --from=ws-shell /app/bin/wsterm /app/bin/wsterm

WORKDIR /app
ENV ENABLE_SSL=false
EXPOSE 80
CMD ["/app/bin/wsterm","-bind",":80","-fork","/bin/sh"]
`;
}

async function value(name, question, fallback) {
  const current = args[name] ?? fallback;
  if (args[name]) return args[name];
  if (!input.isTTY) return current || "";

  const rl = createInterface({ input, output });
  const answer = (
    await rl.question(`${question}${fallback ? ` [${fallback}]` : ""}: `)
  ).trim();
  rl.close();
  return answer || current || "";
}

async function secret(name, question, fallback) {
  if (args[name]) return args[name];
  if (!input.isTTY) return fallback || "";

  const rl = createInterface({ input, output });
  const placeholder = fallback ? mask(fallback) : "skip";
  const answer = (await rl.question(`${question} [${placeholder}]: `)).trim();
  rl.close();
  return answer || fallback || "";
}

async function optionalValue(name, question, fallback) {
  if (args[name] !== undefined) return args[name];
  if (!input.isTTY) return "";

  const rl = createInterface({ input, output });
  const suffix = fallback ? ` [${fallback}; Enter to skip]` : " [skip]";
  const answer = (await rl.question(`${question}${suffix}: `)).trim();
  rl.close();
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

  console.log(color.bold("Choose authentication"));
  console.log(`  ${color.cyan("1")}. basic username/password`);
  console.log(`  ${color.cyan("2")}. GitHub OAuth`);
  console.log(`  ${color.cyan("3")}. both`);
  console.log(`  ${color.cyan("4")}. none`);

  const rl = createInterface({ input, output });
  const answer = (await rl.question(`Authentication [${fallback}]: `)).trim();
  rl.close();

  if (!answer) return modes.has(fallback) ? fallback : "basic";
  if (modes.has(answer)) return answer;
  if (answer === "1") return "basic";
  if (answer === "2") return "github";
  if (answer === "3") return "both";
  if (answer === "4") return "none";
  throw new Error("Authentication must be basic, github, both, or none");
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
  console.log(color.bold("Choose VM image"));
  names.forEach((name, index) =>
    console.log(
      `  ${color.cyan(String(index + 1))}. ${name} ${color.dim(`(${vmImages[name]})`)}`,
    ),
  );
  console.log(`  ${color.cyan("4")}. custom image`);

  const rl = createInterface({ input, output });
  const answer = (await rl.question(`VM image [${fallback}]: `)).trim();
  rl.close();

  if (!answer) return fallback;
  if (vmImages[answer]) return answer;
  if (/^[1-3]$/.test(answer)) return names[Number(answer) - 1];
  if (answer === "4")
    return value(
      "custom-vm-image",
      "Custom VM image",
      defaults["custom-vm-image"],
    );
  return answer;
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

function run(command, commandArgs) {
  return new Promise((resolve, reject) => {
    let seenUrl = "";
    const child = spawn(command, commandArgs, {
      stdio: ["inherit", "pipe", "pipe"],
    });

    child.stdout.on("data", (chunk) => {
      const text = chunk.toString();
      output.write(text);
      seenUrl = findLastUrl(text) || seenUrl;
    });

    child.stderr.on("data", (chunk) => {
      const text = chunk.toString();
      output.write(text);
      seenUrl = findLastUrl(text) || seenUrl;
    });

    child.on("error", reject);
    child.on("close", (code) => {
      if (code === 0 && seenUrl) resolve(seenUrl);
      else if (code === 0)
        reject(new Error("vercel finished but no deployment url was found"));
      else reject(new Error(`vercel exited with code ${code}`));
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
    const child = spawn(command, commandArgs, { stdio: "inherit" });
    child.on("error", reject);
    child.on("close", (code) => {
      if (code === 0) resolve();
      else reject(new Error(`${command} exited with code ${code}`));
    });
  });
}

function findLastUrl(text) {
  return text.match(/https:\/\/[^\s]+\.vercel\.app/g)?.at(-1) ?? "";
}

function printHeader() {
  console.log(color.bold(color.cyan("Vercel VM Factory")));
  console.log(
    color.dim("Build a Container deployment from a tiny Dockerfile.vercel"),
  );
  console.log("");
}

function printSummary(items) {
  console.log(color.bold("Deployment plan"));
  for (const [key, value] of Object.entries(items)) {
    printKeyValue(key, value);
  }
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
