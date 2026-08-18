import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("installer keeps the MVP services private and privileged actions disabled", async () => {
  const installer = await readFile(new URL("../install.sh", import.meta.url), "utf8");
  assert.match(installer, /Massnaev\/jinay-server-panel/);
  assert.match(installer, /SERVERPANEL_LISTEN=127\.0\.0\.1:9080/);
  assert.match(installer, /SERVERPANEL_ENABLE_DOCKER_ACTIONS=false/);
  assert.match(installer, /SERVERPANEL_ENABLE_POWER_ACTIONS=false/);
  assert.match(installer, /SERVERPANEL_POWER_CONTROL/);
  assert.match(installer, /systemctl enable --now serverpanel-power-helper\.service/);
  assert.match(installer, /web\/index\.html/);
  assert.match(installer, /sha256sum --check --status/);
  assert.match(installer, /install -d -m 0755 "\$\{install_root\}"/);
  assert.match(installer, /install -d -m 0700 "\$\{data_dir\}"/);
  assert.match(installer, /chmod 0755 "\$\{install_root\}" "\$\{install_root\}\/releases"/);
  assert.match(installer, /tailscale serve --bg --yes http:\/\/127\.0\.0\.1:9080/);
  assert.match(installer, /Panel URL:/);
  assert.match(installer, /Initial account \(shown once\):/);
  assert.match(installer, /password: %s/);
  assert.match(installer, /SERVERPANEL_AUTO_UPDATE/);
  assert.match(installer, /systemctl enable --now jinay-update\.timer/);
  assert.match(installer, /write-out '%\{url_effective\}'/);
  assert.doesNotMatch(installer, /tailscale funnel/);
  assert.doesNotMatch(installer, /OWNER\/serverpanel/);
});

test("release exports a static interface and keeps Node off the server", async () => {
  const [build, agentUnit, powerUnit, updateUnit, updateTimer, updater] = await Promise.all([
    readFile(new URL("../scripts/build-release.sh", import.meta.url), "utf8"),
    readFile(new URL("../deploy/serverpanel-agent.service", import.meta.url), "utf8"),
    readFile(new URL("../deploy/serverpanel-power-helper.service", import.meta.url), "utf8"),
    readFile(new URL("../deploy/jinay-update.service", import.meta.url), "utf8"),
    readFile(new URL("../deploy/jinay-update.timer", import.meta.url), "utf8"),
    readFile(new URL("../deploy/jinay-update", import.meta.url), "utf8"),
  ]);
  assert.match(build, /-X main\.version=\$\{release_version\}/);
  assert.match(build, /go test \.\/\.\.\./);
  assert.match(build, /cp -R dist\/client\/\./);
  assert.match(build, /serverpanel-power-helper/);
  assert.match(build, /install -m 0755 install\.sh/);
  assert.doesNotMatch(build, /nodejs\.org|runtime\/bin\/node|node_modules.*stage/);
  assert.match(agentUnit, /MemoryDenyWriteExecute=true/);
  assert.match(powerUnit, /RestrictAddressFamilies=AF_UNIX/);
  assert.match(powerUnit, /^CapabilityBoundingSet=$/m);
  assert.match(powerUnit, /ReadWritePaths=-\/sys\/devices\/system\/cpu\/cpufreq/);
  assert.doesNotMatch(powerUnit, /AF_INET|docker\.sock|\/bin\/sh/);
  assert.match(updateUnit, /ExecStart=\/opt\/serverpanel\/current\/deploy\/jinay-update/);
  assert.match(updateTimer, /RandomizedDelaySec=2h/);
  assert.match(updateTimer, /Persistent=true/);
  assert.match(updater, /current_version=.*agent.*version/);
  assert.match(updater, /SERVERPANEL_VERSION="\$\{latest_version\}"/);
  assert.match(updater, /current_version.*== main-/);
  assert.match(updater, /development snapshot/);
  assert.match(updater, /deploy\/install\.sh/);
  assert.doesNotMatch(updater, /curl.*\|.*bash/);
});
