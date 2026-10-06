# HQuizlet Worker proxy

Worker cung cấp URL cố định `https://hquizlet.<account>.workers.dev` và proxy tới Quick Tunnel. Đây là giai đoạn proxy, chưa hosting frontend bằng Static Assets.

## Cloudflare Workers Builds từ GitHub

1. Workers & Pages → Create application → Import a repository.
2. Chọn repo `hunguyen1324/hquizlet-platform`, branch `main`.
3. Worker name: `hquizlet` (khớp `wrangler.jsonc`).
4. Root directory: `infra/cloudflare`.
5. Build command: `npm run check`.
6. Deploy command: `npm run deploy`.
7. Sau deploy, Settings → Variables and Secrets → thêm biến **runtime** Text `UPSTREAM_ORIGIN`, giá trị URL tunnel HTTPS đang hoạt động, không kèm đường dẫn.

`keep_vars: true` giữ biến dashboard khi deploy lại. Biến build không thay thế biến runtime. Không commit token hoặc secret.

## Kiểm tra

```bash
curl -i --max-time 30 https://hquizlet.<account>.workers.dev/api/healthz/services
```

Thử đăng nhập, tạo bộ thẻ, học, quiz và upload/download trên URL mới.

- 503: thiếu/sai cấu hình upstream hoặc trỏ vào chính Worker.
- 502: không fetch được backend; kiểm tra tunnel và DNS.
- Tunnel đổi URL: sửa `UPSTREAM_ORIGIN` và lưu/deploy thay đổi dashboard.

Worker giữ method, body, Authorization, cookie và stream response; không tự retry hoặc cache. Redirect về origin tunnel được chuyển sang hostname Worker. URL tuyệt đối trong HTML/JSON và cookie có Domain cố định cần kiểm tra riêng. WebSocket cần kiểm tra thực tế trên Cloudflare.

Secret `ORIGIN_PROXY_SECRET` là tùy chọn; chỉ thêm khi Nginx đã được cấu hình kiểm tra header `X-Origin-Proxy-Secret`. Worker này chưa tự cấu hình Nginx, Turnstile hoặc cập nhật URL tunnel.

## Local

```bash
npm install
npm run check
npx wrangler deploy --dry-run
```

Để chạy local, tạo `.dev.vars` (đã ignore) với `UPSTREAM_ORIGIN=https://YOUR-TUNNEL.trycloudflare.com`, sau đó `npm run dev`.

Nguồn: https://developers.cloudflare.com/workers/ci-cd/builds/configuration/ và https://developers.cloudflare.com/workers/wrangler/configuration/
