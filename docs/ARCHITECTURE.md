# Architecture

## Components

### Static web application

The React interface is exported to static HTML, CSS and JavaScript during the trusted release build. The Go agent serves those immutable files and the typed API from one loopback origin. No Node.js application server runs on the managed host. The browser never receives Docker socket access, host SSH keys, Codex tokens, or provider API keys.

### Local agent

The agent is the only component allowed to inspect host telemetry or request an operational action. It listens on loopback or a Unix socket, validates the session and CSRF token, checks the caller role, validates typed parameters, executes a fixed command, and appends an audit event.

Read-only hardware telemetry includes swap, CPU frequency policy, ACPI platform-profile availability, hwmon temperatures, fan RPM, and the presence of PWM interfaces. Detection never implies authorization to write. Power-profile and fan mutation routes do not exist until a platform-specific helper, thermal limits, confirmation, audit, and automatic rollback are implemented and validated on the target hardware.

### Same-origin API

The browser calls the agent through the same origin, so there is no generic application proxy. The Go router exposes only the documented API and a public-file allowlist (`/`, `/assets/*`, and `/favicon.svg`). An external Caddy, Nginx, or Tailscale HTTPS edge terminates TLS and forwards traffic to the loopback agent.

### Optional release updater

The opt-in `jinay-update.timer` checks GitHub once per day with randomized delay. It compares the latest published release tag with the version embedded in the running agent. When they differ, a root-only service invokes the installer bundled in the currently verified local release; that installer downloads the versioned archive and verifies its published SHA-256 before switching the `current` symlink. Commits on `main` are never installed automatically.

This is a privileged host boundary. The updater has no HTTP route and receives no browser-supplied command, URL, repository or version. Automatic update remains disabled by default; signed artifacts and stronger provenance verification remain roadmap work.

### Codex bridge (future)

Codex runs under a dedicated Linux account and communicates through its App Server over a Unix socket or authenticated loopback. Jinay translates a small, typed set of UI intents into App Server calls. The browser cannot choose arbitrary tool permissions or bypass approval policy.

OpenAI authentication is owned by Codex using `codex login` or device-code login on the server. The credential store remains readable only by the Codex service account. Panel authentication remains separate.

## Trust boundaries

| Boundary | Untrusted side | Trusted side | Required control |
| --- | --- | --- | --- |
| Internet to agent | Browser/network | TLS edge | TLS, rate limits, allowlist/VPN |
| Browser session to agent | Request headers/body | Loopback agent | Typed routes, 64 KiB JSON limit, session, CSRF and roles |
| Browser session to action | User input | Agent operation | Session, role, CSRF, reauth |
| Agent to host | Typed action | OS/Docker | Allowlist, no shell, timeout |
| Release updater to host | Published GitHub release | Root filesystem and systemd | Opt-in timer, fixed local script, version validation, SHA-256 |
| Panel to Codex | Prompt/tool request | Codex workspace | Sandbox, approvals, budgets |
| Codex to secrets | Agent process | Credential store | Dedicated account, file modes |

## Initial API surface

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/session`
- `GET /api/metrics`
- `GET /api/containers`
- `POST /api/containers/{id}/{start|stop|restart}`
- `GET /api/diagnostics`
- `GET /api/audit`

Power, fan and AI mutation endpoints are deliberately excluded until their platform-specific safety controls exist.

## Data model

The MVP stores user password verifiers, roles, sessions and audit events locally. Passwords are never stored. The first production persistence implementation must support atomic writes, strict file permissions, backups, migrations and bounded audit retention.
