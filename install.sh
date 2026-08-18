#!/usr/bin/env bash
set -euo pipefail

# Jinay release installer for Ubuntu 24.04 and 26.04 LTS.
# A release archive contains the agent, exported web assets, and systemd unit.
# Secrets are generated on the target host only.

repository="${SERVERPANEL_REPOSITORY:-Massnaev/jinay-server-panel}"
version="${SERVERPANEL_VERSION:-latest}"
install_root="/opt/serverpanel"
data_dir="/var/lib/serverpanel"
config_dir="/etc/serverpanel"
service_user="serverpanel"
tailscale_mode="${SERVERPANEL_TAILSCALE_SERVE:-auto}"
auto_update_mode="${SERVERPANEL_AUTO_UPDATE:-off}"
power_control_mode="${SERVERPANEL_POWER_CONTROL:-preserve}"

fail() {
  printf 'Jinay installer: %s\n' "$1" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "run this installer as root (use sudo)"
[[ "${repository}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || fail "invalid GitHub repository name"
[[ "${tailscale_mode}" == "auto" || "${tailscale_mode}" == "off" ]] || fail "SERVERPANEL_TAILSCALE_SERVE must be auto or off"
[[ "${auto_update_mode}" == "on" || "${auto_update_mode}" == "off" ]] || fail "SERVERPANEL_AUTO_UPDATE must be on or off"
[[ "${power_control_mode}" == "preserve" || "${power_control_mode}" == "on" || "${power_control_mode}" == "off" ]] || fail "SERVERPANEL_POWER_CONTROL must be preserve, on or off"
command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v sha256sum >/dev/null 2>&1 || fail "sha256sum is required"
command -v systemctl >/dev/null 2>&1 || fail "systemd is required"
command -v openssl >/dev/null 2>&1 || fail "openssl is required"

case "$(uname -m)" in
  x86_64) architecture="amd64" ;;
  aarch64|arm64) architecture="arm64" ;;
  *) fail "unsupported CPU architecture: $(uname -m)" ;;
esac

asset="serverpanel-linux-${architecture}.tar.gz"
if [[ "${version}" == "latest" ]]; then
  latest_url="$(curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
    --output /dev/null --write-out '%{url_effective}' "https://github.com/${repository}/releases/latest")"
  version="${latest_url##*/}"
  [[ "${version}" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.-]+)?$ ]] || fail "GitHub returned an invalid release tag"
  base_url="https://github.com/${repository}/releases/download/${version}"
  release_id="${version}"
else
  [[ "${version}" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.-]+)?$ ]] || fail "invalid release version"
  base_url="https://github.com/${repository}/releases/download/${version}"
  release_id="${version}"
fi

temporary_dir="$(mktemp -d)"
trap 'rm -rf -- "${temporary_dir}"' EXIT

printf 'Downloading Jinay %s for %s...\n' "${version}" "${architecture}"
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
  "${base_url}/${asset}" -o "${temporary_dir}/${asset}"
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
  "${base_url}/${asset}.sha256" -o "${temporary_dir}/${asset}.sha256"
(
  cd "${temporary_dir}"
  sha256sum --check --status "${asset}.sha256"
) || fail "release checksum verification failed"

install -d -m 0755 "${install_root}" "${install_root}/releases" "${config_dir}"
install -d -m 0700 "${data_dir}"
chmod 0755 "${install_root}" "${install_root}/releases" "${config_dir}"
chmod 0700 "${data_dir}"
release_dir="${install_root}/releases/${release_id}"
install -d -m 0755 "${release_dir}"
chmod 0755 "${release_dir}"
tar --extract --gzip --file "${temporary_dir}/${asset}" --directory "${release_dir}" --no-same-owner

[[ -x "${release_dir}/bin/serverpanel-agent" ]] || fail "release is missing bin/serverpanel-agent"
[[ -x "${release_dir}/bin/serverpanel-power-helper" ]] || fail "release is missing bin/serverpanel-power-helper"
[[ -f "${release_dir}/deploy/serverpanel-agent.service" ]] || fail "release is missing the agent systemd unit"
[[ -f "${release_dir}/deploy/serverpanel-power-helper.service" ]] || fail "release is missing the power helper systemd unit"
[[ -x "${release_dir}/deploy/jinay-update" ]] || fail "release is missing the verified updater"
[[ -f "${release_dir}/deploy/jinay-update.service" ]] || fail "release is missing the updater systemd unit"
[[ -f "${release_dir}/deploy/jinay-update.timer" ]] || fail "release is missing the updater timer"
[[ -r "${release_dir}/deploy/install.sh" ]] || fail "release is missing the trusted installer"
[[ -f "${release_dir}/web/index.html" ]] || fail "release is missing the exported web interface"

if ! id "${service_user}" >/dev/null 2>&1; then
  useradd --system --home-dir "${data_dir}" --shell /usr/sbin/nologin "${service_user}"
fi
chown "${service_user}:${service_user}" "${data_dir}"
chmod 0700 "${data_dir}"

if [[ ! -f "${config_dir}/serverpanel.env" ]]; then
  install -m 0600 /dev/null "${config_dir}/serverpanel.env"
  printf '%s\n' \
    'SERVERPANEL_LISTEN=127.0.0.1:9080' \
    'SERVERPANEL_DATA_DIR=/var/lib/serverpanel' \
    'SERVERPANEL_WEB_ROOT=/opt/serverpanel/current/web' \
    'SERVERPANEL_SECURE_COOKIES=true' \
    'SERVERPANEL_ENABLE_DOCKER_ACTIONS=false' \
    'SERVERPANEL_ENABLE_POWER_ACTIONS=false' \
    'SERVERPANEL_POWER_HELPER_SOCKET=/run/serverpanel-power/power.sock' \
    > "${config_dir}/serverpanel.env"
fi
if ! grep -q '^SERVERPANEL_WEB_ROOT=' "${config_dir}/serverpanel.env"; then
  printf '%s\n' 'SERVERPANEL_WEB_ROOT=/opt/serverpanel/current/web' >> "${config_dir}/serverpanel.env"
fi
if ! grep -q '^SERVERPANEL_ENABLE_POWER_ACTIONS=' "${config_dir}/serverpanel.env"; then
  printf '%s\n' 'SERVERPANEL_ENABLE_POWER_ACTIONS=false' >> "${config_dir}/serverpanel.env"
fi
if ! grep -q '^SERVERPANEL_POWER_HELPER_SOCKET=' "${config_dir}/serverpanel.env"; then
  printf '%s\n' 'SERVERPANEL_POWER_HELPER_SOCKET=/run/serverpanel-power/power.sock' >> "${config_dir}/serverpanel.env"
fi
if [[ "${power_control_mode}" == "on" ]]; then
  sed -i 's/^SERVERPANEL_ENABLE_POWER_ACTIONS=.*/SERVERPANEL_ENABLE_POWER_ACTIONS=true/' "${config_dir}/serverpanel.env"
elif [[ "${power_control_mode}" == "off" ]]; then
  sed -i 's/^SERVERPANEL_ENABLE_POWER_ACTIONS=.*/SERVERPANEL_ENABLE_POWER_ACTIONS=false/' "${config_dir}/serverpanel.env"
fi

ln -sfn "${release_dir}" "${install_root}/current"
install -m 0644 "${release_dir}/deploy/serverpanel-agent.service" /etc/systemd/system/serverpanel-agent.service
install -m 0644 "${release_dir}/deploy/serverpanel-power-helper.service" /etc/systemd/system/serverpanel-power-helper.service
install -m 0644 "${release_dir}/deploy/jinay-update.service" /etc/systemd/system/jinay-update.service
install -m 0644 "${release_dir}/deploy/jinay-update.timer" /etc/systemd/system/jinay-update.timer
if systemctl list-unit-files serverpanel-web.service --no-legend 2>/dev/null | grep -q '^serverpanel-web.service'; then
  systemctl disable --now serverpanel-web.service >/dev/null 2>&1 || true
fi

initial_password=""
if [[ ! -s "${data_dir}/users.json" ]]; then
  initial_password="$(openssl rand -base64 24 | tr -d '\n')"
  printf '%s' "${initial_password}" | runuser -u "${service_user}" -- \
    env SERVERPANEL_DATA_DIR="${data_dir}" \
    "${install_root}/current/bin/serverpanel-agent" user add --username admin --role admin --password-stdin
fi

systemctl daemon-reload
if grep -q '^SERVERPANEL_ENABLE_POWER_ACTIONS=true$' "${config_dir}/serverpanel.env"; then
  systemctl enable --now serverpanel-power-helper.service
  systemctl restart serverpanel-power-helper.service
else
  systemctl disable --now serverpanel-power-helper.service >/dev/null 2>&1 || true
fi
systemctl enable --now serverpanel-agent.service
systemctl restart serverpanel-agent.service
if [[ "${auto_update_mode}" == "on" ]]; then
  systemctl enable --now jinay-update.timer
fi

panel_url="http://127.0.0.1:9080"
tailscale_message=""
if [[ "${tailscale_mode}" == "auto" ]] && command -v tailscale >/dev/null 2>&1; then
  if tailscale_output="$(tailscale serve --bg --yes http://127.0.0.1:9080 2>&1)"; then
    detected_url="$(tailscale serve status 2>/dev/null | sed -n 's#^\(https://[^[:space:]]*\).*#\1#p' | head -n 1)"
    if [[ -n "${detected_url}" ]]; then
      panel_url="${detected_url%/}/"
    fi
  else
    tailscale_message="${tailscale_output}"
  fi
fi

printf '\nJinay installation completed.\n'
if [[ -n "${initial_password}" ]]; then
  printf '\nInitial account (shown once):\n'
  printf '  login: admin\n'
  printf '  password: %s\n' "${initial_password}"
  printf 'Store it in a password manager and replace it after first sign-in.\n'
else
  printf '\nExisting accounts were preserved; passwords cannot be recovered from their hashes.\n'
fi
if systemctl is-enabled --quiet jinay-update.timer 2>/dev/null; then
  printf '\nAutomatic release updates: enabled\n'
else
  printf '\nAutomatic release updates: disabled\n'
  printf 'Enable later: sudo systemctl enable --now jinay-update.timer\n'
fi
if systemctl is-active --quiet serverpanel-power-helper.service 2>/dev/null; then
  printf '\nCPU power profiles: enabled (Eco, Balanced, Turbo)\n'
else
  printf '\nCPU power profiles: disabled\n'
  printf 'Enable later: sudo env SERVERPANEL_POWER_CONTROL=on /opt/serverpanel/current/deploy/install.sh\n'
fi
printf '\nPanel URL:\n  %s\n' "${panel_url}"
if [[ -n "${tailscale_message}" ]]; then
  printf '\nTailscale Serve needs attention:\n%s\n' "${tailscale_message}"
fi
if [[ "${panel_url}" == http://127.0.0.1:* ]]; then
  printf '\nSSH tunnel fallback:\n'
  printf '  ssh -L 3000:127.0.0.1:9080 user@server\n'
  printf 'Then open http://127.0.0.1:3000\n'
else
  printf 'This HTTPS address is private to authorized Tailscale users.\n'
fi
