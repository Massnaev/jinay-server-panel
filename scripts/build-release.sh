#!/usr/bin/env bash
set -euo pipefail

# Run on Ubuntu 24.04 CI so native Node dependencies match the target host.
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${project_root}/release"
temporary_dir="$(mktemp -d)"
release_version="${SERVERPANEL_RELEASE_VERSION:-0.1.0-dev}"
trap 'rm -rf -- "${temporary_dir}"' EXIT

case "$(uname -m)" in
  x86_64) architecture="amd64" ;;
  aarch64|arm64) architecture="arm64" ;;
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
mkdir -p "${stage}/bin" "${stage}/web" "${stage}/deploy"

(
  cd "${project_root}/agent"
  CGO_ENABLED=0 GOOS=linux GOARCH="${architecture}" \
    go build -trimpath -ldflags="-s -w -X main.version=${release_version}" \
    -o "${stage}/bin/serverpanel-agent" ./cmd/serverpanel
)

install -m 0644 deploy/serverpanel-agent.service "${stage}/deploy/serverpanel-agent.service"
cp -R dist/client/. "${stage}/web/"

mkdir -p "${output_dir}"
asset="serverpanel-linux-${architecture}.tar.gz"
tar --create --gzip --file "${output_dir}/${asset}" --directory "${stage}" .
(
  cd "${output_dir}"
  sha256sum "${asset}" > "${asset}.sha256"
)
printf 'Created %s and checksum.\n' "${output_dir}/${asset}"
