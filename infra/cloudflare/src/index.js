function errorResponse(message, status) {
  return new Response(message, {
    status,
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "no-store",
    },
  });
}

export default {
  async fetch(request, env) {
    if (!env.UPSTREAM_ORIGIN) {
      return errorResponse("Chưa cấu hình UPSTREAM_ORIGIN trên Worker", 503);
    }

    let upstream;
    try {
      upstream = new URL(env.UPSTREAM_ORIGIN);
      if (upstream.protocol !== "https:" || upstream.username || upstream.password ||
          upstream.pathname !== "/" || upstream.search || upstream.hash) {
        throw new Error("Invalid upstream origin");
      }
    } catch {
      return errorResponse("UPSTREAM_ORIGIN phải là HTTPS origin, không kèm đường dẫn", 503);
    }

    const publicUrl = new URL(request.url);
    if (upstream.origin === publicUrl.origin) {
      return errorResponse("UPSTREAM_ORIGIN không được trỏ tới chính Worker", 503);
    }

    // Assign path separately so a path starting with // cannot override the host.
    const target = new URL(upstream.origin);
    target.pathname = publicUrl.pathname;
    target.search = publicUrl.search;

    const headers = new Headers(request.headers);
    headers.delete("host");
    headers.delete("x-origin-proxy-secret");
    if (env.ORIGIN_PROXY_SECRET) {
      headers.set("X-Origin-Proxy-Secret", env.ORIGIN_PROXY_SECRET);
    }

    const options = { method: request.method, headers, redirect: "manual" };
    if (request.method !== "GET" && request.method !== "HEAD") {
      options.body = request.body;
    }

    try {
      const response = await fetch(target, options);
      if (response.status === 101) return response;

      const responseHeaders = new Headers(response.headers);
      const location = responseHeaders.get("location");
      if (location) {
        const redirectUrl = new URL(location, target);
        if (redirectUrl.origin === upstream.origin) {
          redirectUrl.protocol = publicUrl.protocol;
          redirectUrl.host = publicUrl.host;
          responseHeaders.set("location", redirectUrl.href);
        }
      }

      responseHeaders.set("Cache-Control", "no-store");
      responseHeaders.set("X-Content-Type-Options", "nosniff");
      responseHeaders.set("Referrer-Policy", "strict-origin-when-cross-origin");
      return new Response(response.body, {
        status: response.status,
        statusText: response.statusText,
        headers: responseHeaders,
      });
    } catch {
      return errorResponse("Backend tạm thời không kết nối được", 502);
    }
  },
};
