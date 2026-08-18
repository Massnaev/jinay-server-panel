# CPU power profiles

Jinay controls processor power indirectly through the Linux CPUFreq interface. It does not claim to set an exact watt limit: the real consumption still depends on both CPUs, workload, voltage, motherboard, memory, disks, GPU, power-supply efficiency and cooling.

## What the profiles change

| Profile | Governor | Maximum CPU frequency | Intel Turbo Boost | Intended use |
| --- | --- | --- | --- | --- |
| Eco | `schedutil` (safe dynamic fallback if unavailable) | 65% of the hardware maximum on every CPU policy | Disabled when `intel_pstate/no_turbo` exists | Lower idle/background consumption and heat |
| Balanced | `schedutil` (safe dynamic fallback if unavailable) | Full hardware range | Allowed | Everyday operation |
| Turbo | `performance` | Full hardware range | Allowed | Short, urgent heavy workloads |

On a multi-socket server, Jinay enumerates every `/sys/devices/system/cpu/cpufreq/policy*` directory. It therefore applies and verifies the limit across both Xeons rather than treating the machine as one processor.

Higher frequency usually requires higher voltage and therefore more power. Linux documents the CPUFreq governors and writable minimum/maximum policy limits in the [CPU frequency and voltage scaling guide](https://www.kernel.org/doc/html/latest/admin-guide/pm/cpufreq.html). Intel documents the `no_turbo` control and passive `intel_cpufreq` mode in the [intel_pstate guide](https://www.kernel.org/doc/html/latest/admin-guide/pm/intel_pstate.html).

## Safety model

- Power actions are disabled by default.
- The web agent continues to run as the unprivileged `serverpanel` user.
- A separate root helper accepts only `eco`, `balanced`, or `turbo` over a group-restricted Unix socket. It has no TCP listener and no command or shell endpoint.
- The API requires an authenticated administrator, a valid CSRF token, and a login less than 15 minutes old.
- Before changing anything, the helper snapshots the governor and limits of every policy plus the Turbo setting. It verifies all written values and restores the snapshot if any write or verification fails.
- The selected profile is stored root-only and reapplied when the helper starts after a reboot.
- Every accepted or rejected browser request is written to the Jinay audit log.

## Installation

Enable power control explicitly during installation:

```bash
curl -fsSL https://raw.githubusercontent.com/Massnaev/jinay-server-panel/main/install.sh | sudo env SERVERPANEL_POWER_CONTROL=on bash
```

To enable it on an existing installation, rerun the verified installer bundled with the installed release:

```bash
sudo env SERVERPANEL_POWER_CONTROL=on /opt/serverpanel/current/deploy/install.sh
```

The installer preserves the current power-control choice on later updates unless `SERVERPANEL_POWER_CONTROL=on` or `off` is supplied explicitly.

## Verification and recovery

```bash
sudo systemctl status serverpanel-power-helper.service
sudo journalctl -u serverpanel-power-helper.service -n 100 --no-pager
sudo -u serverpanel env SERVERPANEL_ENABLE_POWER_ACTIONS=true \
  SERVERPANEL_POWER_HELPER_SOCKET=/run/serverpanel-power/power.sock \
  /opt/serverpanel/current/bin/serverpanel-agent power apply --profile eco
```

Use the panel for normal switching. The CLI is intended for local verification and recovery and accepts the same three fixed profile names.

Disable the capability and stop the helper:

```bash
sudo env SERVERPANEL_POWER_CONTROL=off /opt/serverpanel/current/deploy/install.sh
```

## Fans

Jinay currently reads fan RPM and reports whether Linux exposes PWM files, but it does not write fan speeds. On many servers the board or BMC automatically slows fans as CPU temperature falls, so Eco can reduce fan noise indirectly. Direct fan control will require a recognized BMC/IPMI or motherboard-specific interface, minimum safe RPM limits, temperature failsafes and a tested automatic rollback. A generic PWM write would risk overheating hardware and is intentionally excluded.
