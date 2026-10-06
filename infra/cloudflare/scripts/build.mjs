import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { copyFileSync } from "node:fs";

const webDir = fileURLToPath(new URL("../../../apps/web/", import.meta.url));
const npm = process.platform === "win32" ? "npm.cmd" : "npm";
for (const args of [["ci", "--include=dev"], ["run", "build"]]) {
  const result = spawnSync(npm, args, {
    cwd: webDir,
    stdio: "inherit",
    shell: process.platform === "win32",
    env: { ...process.env, VITE_GATEWAY_URL: "/api" },
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}
copyFileSync(
  new URL("../static/_headers", import.meta.url),
  new URL("../../../apps/web/dist/_headers", import.meta.url),
);
