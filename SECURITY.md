# Security policy

Jinay is a privileged administration surface. A flaw can affect the entire host, every container, and any credentials stored on that host.

## Security invariants

1. The local agent binds to `127.0.0.1` or a Unix socket by default.
2. The public edge terminates TLS and sends strict security headers.
3. Authentication and authorization are enforced on the server for every protected request.
4. State-changing requests require a valid session, role, CSRF token and audit event.
5. The agent exposes typed, allowlisted operations; it never accepts arbitrary shell text.
6. Docker actions are disabled by default because Docker control is effectively root-equivalent.
7. Codex credentials, API keys and generated passwords never enter browser storage or Git.
8. Codex App Server is never exposed directly to a shared or public network.
9. Destructive operations require confirmation and recent reauthentication.
10. Hardware controls are unavailable until the exact platform is detected and tested.
11. The Go router serves only typed API routes and an explicit static-file path allowlist.
12. Automatic Tailscale setup uses Serve only; the installer never enables public Funnel exposure.
13. Automatic updates are opt-in, follow published releases only, verify the release SHA-256, and invoke the installer bundled in the already verified local release.

The production host serves prebuilt static assets; Node.js, Vite and Vinext are build-time dependencies only. The Go agent retains `MemoryDenyWriteExecute=true`.

The optional `jinay-update.timer` runs as root because installing a system release requires writing under `/opt` and restarting the agent. Its service has a fixed executable path and repository, accepts no browser input, and does not pipe a newly downloaded script into a shell. It is disabled unless the operator explicitly enables it, and it refuses to replace `main-*` or development snapshots automatically. A compromised GitHub release remains a supply-chain risk until signed artifacts are implemented.

## Deployment baseline

- Ubuntu 24.04 or 26.04 LTS with current security updates
- Firewall allowing only SSH and the HTTPS reverse proxy
- Panel reachable through a VPN, mesh network, or IP allowlist during MVP
- SSH keys only; disable password SSH login after recovery access is verified
- Separate unprivileged service account for the panel agent
- Root-readable secret files with mode `0600`
- Automated encrypted backups stored off-host
- Login throttling, session expiry and MFA before broader internet exposure

## Reporting a vulnerability

Do not open a public issue containing an exploit, secret, credential, server address, or private log. Until a private security contact is configured, report suspected issues privately to the repository owner.

Include the affected version, impact, reproduction steps, and any safe mitigation. Never test against a server you do not own or have explicit permission to assess.

## Supported versions

No production-supported release exists yet. Security fixes currently target the latest `main` branch.
