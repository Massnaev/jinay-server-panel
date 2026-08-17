#!/usr/bin/env bash
set -euo pipefail

# Run on Ubuntu 24.04 CI so native Node dependencies match the target host.
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${project_root}/release"
temporary_dir="$(mktemp -d)"
node_version="${SERVERPANEL_NODE_VERSION:-24.18.0}"
trap 'rm -rf -- "${temporary_dir}"' EXIT

case "$(uname -m)" in
  x86_64) architecture="amd64"; node_architecture="x64" ;;
  aarch64|arm64) architecture="arm64"; node_architecture="arm64" ;;
  *) printf 'Unsupported architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
esac

command -v node >/dev/null 2>&1
command -v npm >/dev/null 2>&1
command -v go >/dev/null 2>&1

cd "${project_root}"
npm ci --ignore-scripts --no-audit --no-fund
npm run lint
npm test

stage="${temporary_dir}/serverpanel"
mkdir -p "${stage}/bin" "${stage}/web" "${stage}/deploy" "${stage}/runtime"

(
  cd "${project_root}/agent"
  CGO_ENABLED=0 GOOS=linux GOARCH="${architecture}" \
    go build -trimpath -ldflags='-s -w' -o "${stage}/bin/serverpanel-agent" ./cmd/serverpanel
)

install -m 0755 deploy/serverpanel-web "${stage}/bin/serverpanel-web"
install -m 0644 deploy/serverpanel-agent.service "${stage}/deploy/serverpanel-agent.service"
install -m 0644 deploy/serverpanel-web.service "${stage}/deploy/serverpanel-web.service"
cp -R dist node_modules package.json "${stage}/web/"

node_archive="node-v${node_version}-linux-${node_architecture}.tar.xz"
node_base_url="https://nodejs.org/dist/v${node_version}"
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
  "${node_base_url}/${node_archive}" -o "${temporary_dir}/${node_archive}"
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
  "${node_base_url}/SHASUMS256.txt" -o "${temporary_dir}/SHASUMS256.txt"
grep "  ${node_archive}$" "${temporary_dir}/SHASUMS256.txt" > "${temporary_dir}/node.sha256"
(
  cd "${temporary_dir}"
  sha256sum --check --status node.sha256
) || { printf 'Node.js runtime checksum verification failed.\n' >&2; exit 1; }
tar --extract --xz --file "${temporary_dir}/${node_archive}" --directory "${stage}/runtime" --strip-components=1

mkdir -p "${output_dir}"
asset="serverpanel-linux-${architecture}.tar.gz"
tar --create --gzip --file "${output_dir}/${asset}" --directory "${stage}" .
(
  cd "${output_dir}"
  sha256sum "${asset}" > "${asset}.sha256"
)
printf 'Created %s and checksum.\n' "${output_dir}/${asset}"
