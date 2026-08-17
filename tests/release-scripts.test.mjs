import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("installer keeps the MVP services private and privileged actions disabled", async () => {
  const installer = await readFile(new URL("../install.sh", import.meta.url), "utf8");
  assert.match(installer, /Massnaev\/serverpanel/);
  assert.match(installer, /SERVERPANEL_LISTEN=127\.0\.0\.1:9080/);
  assert.match(installer, /SERVERPANEL_ENABLE_DOCKER_ACTIONS=false/);
  assert.match(installer, /web\/index\.html/);
  assert.match(installer, /sha256sum --check --status/);
  assert.match(installer, /install -d -m 0755 "\$\{install_root\}"/);
  assert.match(installer, /install -d -m 0700 "\$\{data_dir\}"/);
  assert.match(installer, /chmod 0755 "\$\{install_root\}" "\$\{install_root\}\/releases"/);
  assert.match(installer, /tailscale serve --bg --yes http:\/\/127\.0\.0\.1:9080/);
  assert.match(installer, /Panel URL:/);
  assert.match(installer, /Initial account \(shown once\):/);
  assert.match(installer, /password: %s/);
  assert.doesNotMatch(installer, /tailscale funnel/);
  assert.doesNotMatch(installer, /OWNER\/serverpanel/);
});

test("release exports a static interface and keeps Node off the server", async () => {
  const [build, agentUnit] = await Promise.all([
    readFile(new URL("../scripts/build-release.sh", import.meta.url), "utf8"),
    readFile(new URL("../deploy/serverpanel-agent.service", import.meta.url), "utf8"),
  ]);
  assert.match(build, /-X main\.version=\$\{release_version\}/);
  assert.match(build, /go test \.\/\.\.\./);
  assert.match(build, /cp -R dist\/client\/\./);
  assert.doesNotMatch(build, /nodejs\.org|runtime\/bin\/node|node_modules.*stage/);
  assert.match(agentUnit, /MemoryDenyWriteExecute=true/);
});
