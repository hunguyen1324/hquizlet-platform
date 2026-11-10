import { readFile } from "node:fs/promises";
import { gzipSync } from "node:zlib";

const dist = new URL("../dist/", import.meta.url);
const manifest = JSON.parse(await readFile(new URL(".vite/manifest.json", dist), "utf8"));
const entry = Object.keys(manifest).find(key => manifest[key].isEntry);
if (!entry) throw new Error("No application entry in build manifest");
const visited = new Set(), files = new Set();
function visit(key) {
  if (visited.has(key)) return;
  visited.add(key);
  const chunk = manifest[key];
  if (!chunk) throw new Error(`Missing manifest chunk: ${key}`);
  files.add(chunk.file);
  for (const file of chunk.css ?? []) files.add(file);
  // Only static imports load at startup; lazy routes are budgeted separately.
  for (const dependency of chunk.imports ?? []) visit(dependency);
}
visit(entry);
const totals = { js: 0, css: 0 };
for (const file of files) {
  const type = file.endsWith(".js") ? "js" : file.endsWith(".css") ? "css" : null;
  if (type) totals[type] += gzipSync(await readFile(new URL(file, dist))).length;
}
const budgets = { js: 110 * 1024, css: 32 * 1024 };
for (const type of ["js", "css"]) {
  console.log(`Startup ${type.toUpperCase()}: ${(totals[type] / 1024).toFixed(1)} KiB gzip / ${budgets[type] / 1024} KiB budget`);
  if (totals[type] > budgets[type]) process.exitCode = 1;
}
