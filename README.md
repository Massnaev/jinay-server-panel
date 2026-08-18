# Jinay Server Panel

Publicly developed, self-hosted control panel for Ubuntu 24.04 and 26.04 LTS servers. Jinay is built for operators who want clear health monitoring and carefully constrained server controls without living in a terminal.

> [!IMPORTANT]
> This repository is an early MVP. Keep it behind a VPN or an authenticated reverse proxy until the security checklist is complete. Never expose the agent, Docker socket, or Codex App Server directly to the internet.

## First milestone

- Responsive dark operations dashboard
- CPU, memory, disk, load and temperature telemetry with protected 1-hour and 24-hour history
- Physical CPU socket topology with per-socket load and automatic DRM/PCI GPU detection
- Physical disk inventory with models, media types, mount usage and available temperature sensors
- Live inbound and outbound network speed
- Local username/password authentication created from Ubuntu
- Docker inventory with opt-in start, stop and restart actions
- Health findings, audit history and recovery guidance
- Opt-in Eco, Balanced and Turbo CPU profiles with verification and automatic rollback
- Codex integration boundary designed around localhost/Unix sockets and approvals
- One-command Ubuntu installer with verified release checksums
- Optional daily updates from verified GitHub Releases

## Architecture

```text
Browser -> Tailscale HTTPS / reverse proxy -> local agent (loopback only)
                                             +-> static web interface
                                             +-> /proc and /sys telemetry
                                             +-> allowlisted Docker actions
                                             +-> typed Unix socket -> CPU power helper
                                             +-> audit log
                                             +-> Codex App Server bridge (future)
```

The browser never receives host credentials and never talks to Docker or Codex directly. The local agent does not expose a general shell endpoint.

Read [ARCHITECTURE.md](docs/ARCHITECTURE.md), [SECURITY.md](SECURITY.md), [ROADMAP.md](ROADMAP.md), and the [command reference](docs/COMMANDS.md) before deploying or contributing.
The exact power-profile behavior and limitations are documented in [POWER.md](docs/POWER.md).

## Development

Requirements: Node.js 22+ and Go 1.22+.

```bash
npm ci
npm run dev
```

The current web preview uses realistic demo telemetry. The Ubuntu agent is developed in `agent/` and will replace demo data when connected.

## Interface preview

### Server overview

![Server overview dashboard](docs/screenshots/overview-desktop.png)

### Docker containers

![Docker container management](docs/screenshots/containers-desktop.png)

### Mobile layout

![Mobile server overview](docs/screenshots/overview-mobile.png)

## Installation

The intended release flow is:

```bash
curl -fsSL https://raw.githubusercontent.com/Massnaev/jinay-server-panel/main/install.sh | sudo bash
```

The installer verifies the published SHA-256 release checksum, binds the agent to loopback, creates the first administrator, and prints the login, one-time password, and detected panel URL in the same terminal. Do not use an installer copied from an issue or chat message. Release archives are produced on Ubuntu with `scripts/build-release.sh`; each archive contains the tested Linux agent and prebuilt static interface. Node.js is not installed or run on the managed server.

CPU power control is privileged and therefore remains disabled by default. Enable the three fixed profiles explicitly:

```bash
curl -fsSL https://raw.githubusercontent.com/Massnaev/jinay-server-panel/main/install.sh | sudo env SERVERPANEL_POWER_CONTROL=on bash
```

Eco caps every CPU frequency policy at 65% of its hardware maximum and disables Intel Turbo Boost where supported. Balanced restores dynamic full-range scaling; Turbo requests maximum performance. Jinay snapshots and verifies all policies and rolls them back if any step fails. This changes CPU frequency behavior, not an exact watt limit.

When Tailscale is already installed, the installer attempts to enable private HTTPS automatically. Manual equivalent:

```bash
sudo tailscale serve --bg --yes http://127.0.0.1:9080
```

This does not enable Tailscale Funnel and does not publish the panel to the public internet. Keep the agent bound to loopback.

If another Tailscale user cannot open the address, see [Tailscale access and device sharing](docs/TAILSCALE.md). The server must be in the same tailnet or explicitly shared with that user; normal Tailscale connectivity alone does not grant access.

## Secrets

- Commit `.env.example`, never `.env`.
- Store runtime secrets in root-readable files or the OS credential store.
- Never commit API keys, OAuth tokens, `~/.codex/auth.json`, private certificates, generated passwords, or production logs.
- Treat Docker access and Codex credentials as root-equivalent capabilities.

## Contributing

Each logical change should be committed separately. Security-sensitive changes need tests and an explanation of the capability they add or widen.

## Donations

If Jinay is useful to you, you can support its development:

- **USDT (TRC-20):** `TUz11JVX41hTXBUWbbRazmBRWhxBGkytqT`

Always verify the network and address before sending funds. Cryptocurrency transfers cannot be reversed.

## License

A license will be selected before the first public release.
