import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("installer keeps the MVP services private and privileged actions disabled", async () => {
  const installer = await readFile(new URL("../install.sh", import.meta.url), "utf8");
  assert.match(installer, /Massnaev\/serverpanel/);
  assert.match(installer, /SERVERPANEL_LISTEN=127\.0\.0\.1:9080/);
  assert.match(installer, /SERVERPANEL_ENABLE_DOCKER_ACTIONS=false/);
  assert.match(installer, /runtime\/bin\/node/);
  assert.match(installer, /sha256sum --check --status/);
  assert.match(installer, /install -d -m 0755 "\$\{install_root\}"/);
  assert.match(installer, /install -d -m 0700 "\$\{data_dir\}"/);
  assert.match(installer, /chmod 0755 "\$\{install_root\}" "\$\{install_root\}\/releases"/);
  assert.doesNotMatch(installer, /OWNER\/serverpanel/);
});

test("release verifies and bundles its pinned Node runtime", async () => {
  const [build, launcher] = await Promise.all([
    readFile(new URL("../scripts/build-release.sh", import.meta.url), "utf8"),
    readFile(new URL("../deploy/serverpanel-web", import.meta.url), "utf8"),
  ]);
  assert.match(build, /node_version="\$\{SERVERPANEL_NODE_VERSION:-24\.18\.0\}"/);
  assert.match(build, /-X main\.version=\$\{release_version\}/);
  assert.match(build, /SHASUMS256\.txt/);
  assert.match(build, /sha256sum --check --status node\.sha256/);
  assert.match(launcher, /runtime\/bin\/node/);
  assert.match(launcher, /--jitless/);
});
