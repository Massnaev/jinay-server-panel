import { mkdir, writeFile } from "node:fs/promises";

const workerUrl = new URL("../dist/server/index.js", import.meta.url);
const { default: worker } = await import(workerUrl.href);
const response = await worker.fetch(
  new Request("http://localhost/", { headers: { accept: "text/html" } }),
  { ASSETS: { fetch: async () => new Response("Not found", { status: 404 }) } },
  { waitUntil() {}, passThroughOnException() {} },
);

if (!response.ok) throw new Error(`Static export failed with HTTP ${response.status}`);
const html = (await response.text()).replace(
  /url\([^)]*?\.vinext\/fonts\/([^)]*)\)/g,
  "url(/assets/_vinext_fonts/$1)",
);
const outputDirectory = new URL("../dist/client/", import.meta.url);
await mkdir(outputDirectory, { recursive: true });
await writeFile(new URL("index.html", outputDirectory), html, "utf8");
console.log("Exported dist/client/index.html");
