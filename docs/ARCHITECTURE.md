# Architecture

## Components

### Web application

The web application owns presentation, navigation and the browser session boundary. It talks only to the ServerPanel API through the same HTTPS origin. It never receives Docker socket access, host SSH keys, Codex tokens, or provider API keys.

### Local agent

The agent is the only component allowed to inspect host telemetry or request an operational action. It listens on loopback or a Unix socket, validates the session and CSRF token, checks the caller role, validates typed parameters, executes a fixed command, and appends an audit event.

### Reverse proxy

Caddy or Nginx terminates TLS, applies request limits, and proxies only the required paths. The agent port is not opened by the firewall.

### Codex bridge (future)

Codex runs under a dedicated Linux account and communicates through its App Server over a Unix socket or authenticated loopback. ServerPanel translates a small, typed set of UI intents into App Server calls. The browser cannot choose arbitrary tool permissions or bypass approval policy.

OpenAI authentication is owned by Codex using `codex login` or device-code login on the server. The credential store remains readable only by the Codex service account. Panel authentication remains separate.

## Trust boundaries

| Boundary | Untrusted side | Trusted side | Required control |
| --- | --- | --- | --- |
| Internet to proxy | Browser/network | TLS edge | TLS, rate limits, allowlist/VPN |
| Proxy to app | Request headers/body | Web app | Header sanitation, size limits |
| Browser session to action | User input | Agent operation | Session, role, CSRF, reauth |
| Agent to host | Typed action | OS/Docker | Allowlist, no shell, timeout |
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
