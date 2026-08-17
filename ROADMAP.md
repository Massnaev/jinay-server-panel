# ServerPanel roadmap

## Phase 0 — secure foundation (current)

- [x] Repository, product architecture and security boundary
- [x] Dashboard design system and realistic preview data
- [x] Local agent with loopback-only HTTP API
- [x] CLI administrator creation and one-time initial credential
- [x] Session cookies, CSRF protection, login rate limiting and audit log
- [x] Ubuntu 24.04 systemd unit and reverse-proxy example
- [ ] Signed release checksums and idempotent installer

## Phase 1 — useful server operations

- [ ] Live CPU, RAM, load, disks, network and temperature history
- [ ] Docker inventory, logs and opt-in start/stop/restart
- [ ] Service health, panel errors and actionable recommendations
- [ ] Roles: administrator, operator and read-only viewer
- [ ] Backup and restore of panel configuration
- [ ] MFA/WebAuthn and recovery codes

## Phase 2 — power and hardware

- [ ] Detect supported power-profile and CPU governor interfaces
- [ ] Eco, balanced and turbo profiles with measured before/after impact
- [ ] Hardware-specific IPMI/Redfish integration
- [ ] Fan-control support only for detected and explicitly supported hardware
- [ ] Safety limits, automatic rollback and thermal shutdown protections

## Phase 3 — Codex and AI operations

- [ ] Install Codex CLI as an isolated service account
- [ ] ChatGPT/device-code login without exposing the credential cache
- [ ] Local Codex App Server bridge over Unix socket or authenticated loopback
- [ ] Per-tool allowlists, approvals, budgets and immutable audit trails
- [ ] Read-only diagnostics mode before any mutation capability
- [ ] Workspace sandboxing for product generation

## Phase 4 — provider ecosystem

- [ ] OpenRouter provider adapter and cost controls
- [ ] Google model/provider adapter after its exact supported product is selected
- [ ] Local/open-weight models
- [ ] Provider-neutral model routing, quotas and fallbacks
- [ ] IDE/workspace integrations where supported by vendor APIs

## Phase 5 — public project maturity

- [ ] Independent security review and threat-model refresh
- [ ] Stable versioned API and upgrade/migration path
- [ ] Signed packages and reproducible builds
- [ ] Localization and accessibility audit
- [ ] Verified donation addresses and project governance

## Explicit non-goals

- No browser-exposed root shell.
- No direct public Docker socket or Codex App Server.
- No automatic publishing of secrets or vendor tokens.
- No generic fan control without verified hardware support.
