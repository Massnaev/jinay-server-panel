import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

async function render() {
  const workerUrl = new URL("../dist/server/index.js", import.meta.url);
  workerUrl.searchParams.set("test", `${process.pid}-${Date.now()}`);
  const { default: worker } = await import(workerUrl.href);
  return worker.fetch(
    new Request("http://localhost/", { headers: { accept: "text/html" } }),
    { ASSETS: { fetch: async () => new Response("Not found", { status: 404 }) } },
    { waitUntil() {}, passThroughOnException() {} },
  );
}

test("server-renders the protected ServerPanel shell", async () => {
  const response = await render();
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);
  const html = await response.text();
  assert.match(html, /<html lang="ru"/i);
  assert.match(html, /<title>ServerPanel/);
  assert.match(html, /Проверяем защищённую сессию/);
  assert.doesNotMatch(html, /codex-preview|Your site is taking shape|react-loading-skeleton/i);
});

test("keeps the MVP security and accessibility boundaries visible in source", async () => {
  const [page, css, layout, packageJson] = await Promise.all([
    readFile(new URL("../app/page.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/globals.css", import.meta.url), "utf8"),
    readFile(new URL("../app/layout.tsx", import.meta.url), "utf8"),
    readFile(new URL("../package.json", import.meta.url), "utf8"),
  ]);
  assert.match(page, /X-CSRF-Token/);
  assert.match(page, /credentials: "include"/);
  assert.match(page, /Подтверждение операции/);
  assert.match(page, /Режим предпросмотра/);
  assert.doesNotMatch(page, /exec\(|spawn\(|\/bin\/sh|sudo /);
  assert.match(css, /focus-visible/);
  assert.match(css, /prefers-reduced-motion:\s*reduce/);
  assert.match(layout, /title: "ServerPanel/);
  assert.doesNotMatch(packageJson, /react-loading-skeleton/);
});
