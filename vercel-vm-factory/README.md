# Vercel VM Factory

Create a tiny Vercel Container deployment: copy `wsterm` from `ghcr.io/v1xingyue/ws-shell:v1.8.alpine` into a selected VM image, then deploy with Vercel CLI.

```bash
npx vercel-vm-factory create \
  --vm-image ubuntu \
  --shell /bin/bash \
  --tools nodejs,codex \
  --project ws-shell-ubuntu \
  --auth-mode basic \
  --auth-user admin \
  --auth-password change-me
```

GitHub OAuth is optional:

```bash
npx vercel-vm-factory create \
  --auth-mode github \
  --client-id YOUR_GITHUB_CLIENT_ID \
  --client-secret YOUR_GITHUB_CLIENT_SECRET \
  --github-userid 12345678
```

Run without flags for prompts:

```bash
npx vercel-vm-factory create
```

The prompt walks through VM image, project, shell, optional preinstalled tools, and authentication. For list prompts, enter either names or numbers; tool choices can be comma-separated, for example `1,3` or `nodejs,claude-code`.

Check local setup:

```bash
npx vercel-vm-factory doctor
```

The script checks `vercel --version` and `vercel whoami`; if you are not logged in, it runs `vercel login`.

Use `--help` to show all flags.

Common flags:

- `--vm-image alpine|ubuntu|debian|IMAGE`
- `--shell /bin/bash|/bin/zsh|/bin/sh`
- `--tools nodejs,codex,claude-code`
- `--project NAME`
- `--scope TEAM_SLUG`
- `--auth-mode basic|github|both|none`
- `--dry-run`

Entered auth values are reused from `~/.vercel-vm-factory/defaults.json`; press Enter to keep the placeholder value or skip an empty one.

The generated project contains only `Dockerfile.vercel`.

CLI mapping:

- Vercel Team -> `--scope TEAM_SLUG` when needed; omit it to use the CLI default scope
- Project Name -> `--project x-shell`
- Application Preset -> patched through Vercel API as `framework=container`
- Root Directory -> generated project directory

VM image presets:

- `alpine` -> `alpine:3.23`
- `ubuntu` -> `ubuntu:24.04`
- `debian` -> `debian:13-slim`

Shell options:

- `/bin/bash`
- `/bin/zsh`
- `/bin/sh`

Choosing bash or zsh adds the matching package to the generated Dockerfile when the VM image does not already include it. Choosing zsh also installs oh-my-zsh.

Generated shell setup examples:

- Alpine + `/bin/zsh`: installs `zsh curl git`, then installs oh-my-zsh unattended.
- Ubuntu/Debian + `/bin/bash`: installs `bash` with `apt-get`.
- `/bin/sh`: no extra shell package is installed.

Preinstall tools:

- `nodejs`
- `codex`
- `claude-code`

Choosing codex or claude-code also installs Node.js/npm.

Generated tool setup examples:

- `--tools nodejs`: installs `nodejs npm`
- `--tools codex`: installs `nodejs npm`, then `npm install -g @openai/codex`
- `--tools claude-code`: installs `nodejs npm`, then `npm install -g @anthropic-ai/claude-code`
- `--tools codex,claude-code`: installs both CLIs

Custom VM image:

```bash
npx vercel-vm-factory create --vm-image fedora:42 --project ws-shell-fedora
```

Before deploying, set the GitHub OAuth callback URL to:

```text
https://PROJECT.vercel.app/auth/github/callback
```

For example, if `--project x-shell`, set:

```text
https://x-shell.vercel.app/auth/github/callback
```

GitHub OAuth fields:

- Auth mode -> `--auth-mode basic|github|both|none`
- Username -> `--auth-user`
- Password -> `--auth-password`
- Client ID -> `--client-id`
- Client Secret -> `--client-secret`
- Numeric GitHub user ID -> `--github-userid`

Get your numeric user ID:

```text
https://api.github.com/users/YOUR_LOGIN
```

The callback URL must match exactly. If you deploy under another project name or custom domain, update the GitHub OAuth App or pass `--redirect-url`.

Use `--dry-run` to generate files without deploying.
