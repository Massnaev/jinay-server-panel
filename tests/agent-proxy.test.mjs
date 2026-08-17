import assert from "node:assert/strict";
import test from "node:test";

import { proxyAgentRequest, resolveAgentPath } from "../lib/agent-proxy.js";

test("allows only the typed agent API surface", () => {
  assert.equal(resolveAgentPath("GET", ["metrics"]), "/api/metrics");
  assert.equal(resolveAgentPath("POST", ["auth", "login"]), "/api/auth/login");
  assert.equal(resolveAgentPath("POST", ["containers", "abc-123", "restart"]), "/api/containers/abc-123/restart");
  assert.equal(resolveAgentPath("DELETE", ["containers", "abc-123"]), null);
  assert.equal(resolveAgentPath("POST", ["containers", "abc-123", "exec"]), null);
  assert.equal(resolveAgentPath("GET", ["http:", "example.com"]), null);
  assert.equal(resolveAgentPath("POST", ["containers", "..", "restart"]), null);
});

test("forwards only cookie, content type and CSRF headers to loopback", async () => {
  let capturedUrl = "";
  let capturedInit;
  const request = new Request("https://panel.example/api/auth/logout", {
    method: "POST",
    headers: {
      Cookie: "sp_session=secret",
      "X-CSRF-Token": "csrf",
      Authorization: "do-not-forward",
      "X-Forwarded-For": "203.0.113.8",
    },
  });
  const response = await proxyAgentRequest(request, ["auth", "logout"], async (url, init) => {
    capturedUrl = String(url);
    capturedInit = init;
    return new Response(null, { status: 204, headers: { "Set-Cookie": "sp_session=; Max-Age=0" } });
  });

  assert.equal(capturedUrl, "http://127.0.0.1:9080/api/auth/logout");
  assert.equal(capturedInit.headers.get("cookie"), "sp_session=secret");
  assert.equal(capturedInit.headers.get("x-csrf-token"), "csrf");
  assert.equal(capturedInit.headers.get("authorization"), null);
  assert.equal(capturedInit.headers.get("x-forwarded-for"), null);
  assert.equal(response.status, 204);
  assert.equal(response.headers.get("cache-control"), "no-store");
});

test("rejects unknown routes and oversized bodies before fetch", async () => {
  let calls = 0;
  const neverFetch = async () => {
    calls += 1;
    return new Response();
  };

  const unknown = await proxyAgentRequest(
    new Request("https://panel.example/api/shell", { method: "POST" }),
    ["shell"],
    neverFetch,
  );
  assert.equal(unknown.status, 404);

  const oversized = await proxyAgentRequest(
    new Request("https://panel.example/api/auth/login", {
      method: "POST",
      headers: { "Content-Length": String(65 * 1024) },
      body: "{}",
    }),
    ["auth", "login"],
    neverFetch,
  );
  assert.equal(oversized.status, 413);
  assert.equal(calls, 0);
});

test("returns a bounded error when the loopback agent is unavailable", async () => {
  const response = await proxyAgentRequest(
    new Request("https://panel.example/api/session"),
    ["session"],
    async () => { throw new Error("private upstream detail"); },
  );
  assert.equal(response.status, 503);
  assert.deepEqual(await response.json(), { error: "The local ServerPanel agent is unavailable." });
});
