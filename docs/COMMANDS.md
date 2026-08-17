# Jinay command reference

Jinay keeps its original `/opt/serverpanel`, `/etc/serverpanel`, `/var/lib/serverpanel` paths and `serverpanel-agent.service` name for upgrade compatibility. These are implementation details; the product and repository are named Jinay Server Panel.

## Install

Default installation keeps automatic updates disabled:

```bash
curl -fsSL https://raw.githubusercontent.com/Massnaev/jinay-server-panel/main/install.sh | sudo bash
```

Enable release auto-updates during installation:

```bash
curl -fsSL https://raw.githubusercontent.com/Massnaev/jinay-server-panel/main/install.sh | sudo env SERVERPANEL_AUTO_UPDATE=on bash
```

The installer prints the panel URL and, on first installation only, the generated `admin` password. Store that password immediately; Jinay stores only its verifier and cannot display it again.

## Status and logs

```bash
sudo systemctl status serverpanel-agent.service
sudo journalctl -u serverpanel-agent.service -n 100 --no-pager
curl http://127.0.0.1:9080/healthz
/opt/serverpanel/current/bin/serverpanel-agent version
```

## Accounts

List accounts:

```bash
sudo -u serverpanel env SERVERPANEL_DATA_DIR=/var/lib/serverpanel /opt/serverpanel/current/bin/serverpanel-agent user list
```

Create an administrator without placing the password in shell history:

```bash
read -rsp "New Jinay password: " JINAY_NEW_PASSWORD
printf '%s' "$JINAY_NEW_PASSWORD" | sudo -u serverpanel env SERVERPANEL_DATA_DIR=/var/lib/serverpanel /opt/serverpanel/current/bin/serverpanel-agent user add --username new-admin --role admin --password-stdin
unset JINAY_NEW_PASSWORD
sudo systemctl restart serverpanel-agent.service
```

Valid roles are `admin`, `operator`, and `viewer`.

## Automatic updates

Enable the daily check:

```bash
sudo systemctl enable --now jinay-update.timer
```

Disable it:

```bash
sudo systemctl disable --now jinay-update.timer
```

Inspect the schedule and previous updater log:

```bash
systemctl list-timers jinay-update.timer
sudo journalctl -u jinay-update.service -n 100 --no-pager
```

Run one update check now:

```bash
sudo systemctl start jinay-update.service
```

The updater follows published GitHub Releases only. It does not install commits from `main` and does not create releases.

Development versions such as `main-abcdef0` are never replaced automatically, which prevents an older stable release from overwriting a newer test snapshot. Install the next stable release manually once to return that server to the stable update channel.

## Manual update and rollback

Run the trusted installer bundled with the currently installed release:

```bash
sudo /opt/serverpanel/current/deploy/install.sh
```

List installed release directories before a rollback:

```bash
sudo find /opt/serverpanel/releases -mindepth 1 -maxdepth 1 -type d -printf '%f\n'
```

Rollback requires choosing one exact directory from that list, then switching the symlink and restarting the agent:

```bash
sudo ln -sfn /opt/serverpanel/releases/EXACT_VERSION /opt/serverpanel/current
sudo systemctl restart serverpanel-agent.service
```

Never paste a server address, generated password, API key, private log, or token into a GitHub issue.
