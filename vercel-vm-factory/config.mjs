export const defaultWsShellImage = "ghcr.io/v1xingyue/ws-shell:v1.9.alpine";

export const vmImages = {
  alpine: "alpine:3.23.5",
  ubuntu: "ubuntu:26.10",
  debian: "debian:13.5",
};

export const shells = ["/bin/bash", "/bin/zsh", "/bin/sh"];

export const toolChoices = {
  nodejs: "Node.js + npm",
  codex: "OpenAI Codex CLI",
  "claude-code": "Claude Code",
};

export const projectNameSuffixes = [
  "juddy",
  "nova",
  "orbit",
  "pixel",
  "spark",
  "ripple",
  "atlas",
  "echo",
];

export const codeDefaults = {
  "vm-image": "alpine",
  from: defaultWsShellImage,
  shell: "/bin/sh",
};
