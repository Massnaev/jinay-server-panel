#!/usr/bin/env bash
set -euo pipefail

# ServerPanel release installer for Ubuntu 24.04 and 26.04 LTS.
# A release archive contains the agent, exported web assets, and systemd unit.
# Secrets are generated on the target host only.

repository="${SERVERPANEL_REPOSITORY:-Massnaev/serverpanel}"
version="${SERVERPANEL_VERSION:-latest}"
install_root="/opt/serverpanel"
data_dir="/var/lib/serverpanel"
config_dir="/etc/serverpanel"
service_user="serverpanel"

fail() {
  printf 'ServerPanel installer: %s\n' "$1" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "run this installer as root (use sudo)"
[[ "${repository}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || fail "invalid GitHub repository name"
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
  base_url="https://github.com/${repository}/releases/latest/download"
  release_id="latest-$(date -u +%Y%m%d%H%M%S)"
else
  [[ "${version}" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.-]+)?$ ]] || fail "invalid release version"
  base_url="https://github.com/${repository}/releases/download/${version}"
  release_id="${version}"
fi

temporary_dir="$(mktemp -d)"
trap 'rm -rf -- "${temporary_dir}"' EXIT

printf 'Downloading ServerPanel %s for %s...\n' "${version}" "${architecture}"
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
[[ -f "${release_dir}/deploy/serverpanel-agent.service" ]] || fail "release is missing the agent systemd unit"
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
    > "${config_dir}/serverpanel.env"
fi
if ! grep -q '^SERVERPANEL_WEB_ROOT=' "${config_dir}/serverpanel.env"; then
  printf '%s\n' 'SERVERPANEL_WEB_ROOT=/opt/serverpanel/current/web' >> "${config_dir}/serverpanel.env"
fi

ln -sfn "${release_dir}" "${install_root}/current"
install -m 0644 "${release_dir}/deploy/serverpanel-agent.service" /etc/systemd/system/serverpanel-agent.service
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
systemctl enable --now serverpanel-agent.service
systemctl restart serverpanel-agent.service

printf '\nServerPanel is listening locally on http://127.0.0.1:9080\n'
printf 'Use an SSH tunnel until HTTPS and the domain are configured:\n'
printf '  ssh -L 3000:127.0.0.1:9080 user@server\n'
if [[ -n "${initial_password}" ]]; then
  printf '\nInitial account (shown once):\n'
  printf '  login: admin\n'
  printf '  password: %s\n' "${initial_password}"
  printf 'Store it in a password manager and replace it after first sign-in.\n'
fi
