const AGENT_ORIGIN = "http://127.0.0.1:9080";
const MAX_REQUEST_BYTES = 64 * 1024;
const UPSTREAM_TIMEOUT_MS = 35_000;

const staticRoutes = new Map([
  ["GET session", "/api/session"],
  ["GET metrics", "/api/metrics"],
  ["GET containers", "/api/containers"],
  ["GET diagnostics", "/api/diagnostics"],
  ["GET audit", "/api/audit"],
  ["POST auth/login", "/api/auth/login"],
  ["POST auth/logout", "/api/auth/logout"],
]);

const containerActionPattern = /^containers\/([a-zA-Z0-9][a-zA-Z0-9_.-]{0,127})\/(start|stop|restart)$/;

/**
 * Resolve only the API operations implemented by the local agent.
 * Returning null is intentional: this is an allowlist, not a general proxy.
 *
 * @param {string} method
 * @param {string[]} segments
 */
export function resolveAgentPath(method, segments) {
  const normalizedMethod = method.toUpperCase();
  const path = segments.join("/");
  const staticPath = staticRoutes.get(`${normalizedMethod} ${path}`);
  if (staticPath) return staticPath;

  if (normalizedMethod !== "POST") return null;
  const action = containerActionPattern.exec(path);
  if (!action) return null;
  return `/api/containers/${encodeURIComponent(action[1])}/${action[2]}`;
}

/** @param {Request} request */
function upstreamHeaders(request) {
  const headers = new Headers({ Accept: "application/json" });
  for (const name of ["content-type", "cookie", "x-csrf-token"]) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  return headers;
}

/** @param {Response} response */
function browserHeaders(response) {
  const headers = new Headers();
  for (const name of ["content-type", "cache-control", "set-cookie"]) {
    const value = response.headers.get(name);
    if (value) headers.set(name, value);
  }
  headers.set("Cache-Control", "no-store");
  headers.set("X-Content-Type-Options", "nosniff");
  return headers;
}

function jsonError(status, message) {
  return Response.json({ error: message }, {
    status,
    headers: { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" },
  });
}

/**
 * @param {Request} request
 * @param {string[]} segments
 * @param {typeof fetch} fetchImpl
 */
export async function proxyAgentRequest(request, segments, fetchImpl = fetch) {
  const agentPath = resolveAgentPath(request.method, segments);
  if (!agentPath) return jsonError(404, "Unknown ServerPanel API operation.");

  let body;
  if (request.method !== "GET" && request.method !== "HEAD") {
    const declaredLength = Number(request.headers.get("content-length") || "0");
    if (Number.isFinite(declaredLength) && declaredLength > MAX_REQUEST_BYTES) {
      return jsonError(413, "Request body is too large.");
    }
    const bytes = await request.arrayBuffer();
    if (bytes.byteLength > MAX_REQUEST_BYTES) return jsonError(413, "Request body is too large.");
    if (bytes.byteLength) body = bytes;
  }

  try {
    const upstream = await fetchImpl(`${AGENT_ORIGIN}${agentPath}`, {
      method: request.method,
      headers: upstreamHeaders(request),
      body,
      redirect: "manual",
      signal: AbortSignal.timeout(UPSTREAM_TIMEOUT_MS),
    });
    return new Response(upstream.body, {
      status: upstream.status,
      headers: browserHeaders(upstream),
    });
  } catch {
    return jsonError(503, "The local ServerPanel agent is unavailable.");
  }
}
