import assert from "node:assert/strict";
import { test } from "node:test";
import worker from "../src/index.js";

test("frontend and templates work without upstream or network", async () => {
  for (const path of ["/", "/study-sets/42", "/assets/app.js", "/templates/flashcard_template.xlsx"]) {
    const response = await worker.fetch(new Request(`https://app.workers.dev${path}`), {
      ASSETS: { fetch: async (request) => new Response(new URL(request.url).pathname) },
    });
    assert.equal(await response.text(), path);
  }
});

test("API and files require upstream instead of returning SPA HTML", async () => {
  for (const path of ["/api", "/api/healthz/services", "/files", "/files/avatar.png"]) {
    assert.equal((await worker.fetch(new Request(`https://app.workers.dev${path}`), {})).status, 503);
  }
});

test("proxy preserves upload and auth, overwrites secret and rewrites redirects", async () => {
  const originalFetch = globalThis.fetch;
  try {
    globalThis.fetch = async (url, options) => {
      assert.equal(String(url), "https://backend.trycloudflare.com/api/upload?q=1");
      assert.equal(options.method, "POST");
      assert.equal(options.headers.get("Authorization"), "Bearer test");
      assert.equal(options.headers.get("X-Origin-Proxy-Secret"), "trusted");
      assert.equal(await new Response(options.body).text(), "upload");
      return new Response(null, { status: 302, headers: { Location: "/login" } });
    };
    const response = await worker.fetch(new Request("https://app.workers.dev/api/upload?q=1", {
      method: "POST", body: "upload",
      headers: { Authorization: "Bearer test", "X-Origin-Proxy-Secret": "untrusted" },
    }), { UPSTREAM_ORIGIN: "https://backend.trycloudflare.com", ORIGIN_PROXY_SECRET: "trusted" });
    assert.equal(response.headers.get("Location"), "https://app.workers.dev/login");
    assert.equal(response.headers.get("Cache-Control"), "no-store");
  } finally { globalThis.fetch = originalFetch; }
});
