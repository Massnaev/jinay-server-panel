# Tailscale access

ServerPanel remains bound to `127.0.0.1:9080`. When Tailscale is installed, the installer attempts to configure private HTTPS access with Tailscale Serve and prints the resulting URL.

## A second user cannot open the panel

Being connected to Tailscale is not sufficient by itself. The user must be authorized to reach the server in one of these ways:

1. Add the user to the server owner's tailnet; or
2. Share the `node01` machine with that user's Tailscale account and ask them to accept the invitation.

The recipient must use the full address printed by the installer, for example `https://node01.example-tailnet.ts.net/`. Shared machines cannot be reached using only the short hostname. Tailnet access-control rules still apply to shared machines and Tailscale Serve.

On the recipient's device, verify connectivity before debugging ServerPanel:

```bash
tailscale ping node01.example-tailnet.ts.net
curl -I https://node01.example-tailnet.ts.net/
```

If `tailscale ping` fails, fix the invitation, device sharing, MagicDNS, or tailnet access policy first. If ping works but HTTPS fails, check `sudo tailscale serve status` on the server.

Do not enable Tailscale Funnel for an administrative MVP panel. Funnel makes the endpoint reachable from the public internet.

## Disable automatic Serve configuration

For a deployment using Caddy, Nginx, or another private edge:

```bash
curl -fsSL https://raw.githubusercontent.com/Massnaev/serverpanel/main/install.sh \
  | sudo SERVERPANEL_TAILSCALE_SERVE=off bash
```
