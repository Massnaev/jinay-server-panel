# Repository instructions

- Make one focused commit for each logical change.
- Never commit secrets, `.env`, auth caches, generated passwords, server addresses, private logs, or production data.
- Do not add an arbitrary shell or command-execution endpoint.
- Keep Docker and Codex App Server off the public network.
- Default every privileged capability to disabled until explicitly configured.
- Add tests for authentication, authorization and command validation changes.
- Update `ROADMAP.md`, `SECURITY.md`, or `docs/ARCHITECTURE.md` when a change alters scope or trust boundaries.
