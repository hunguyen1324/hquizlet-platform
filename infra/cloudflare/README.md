# HQuizlet Worker proxy

Worker cung cấp URL cố định `https://hquizlet.<account>.workers.dev`. Frontend được phục vụ bằng Workers Static Assets; chỉ `/api`, `/api/*`, `/files` và `/files/*` được proxy tới Quick Tunnel. SPA, assets và template không phụ thuộc tunnel.

## Cloudflare Workers Builds từ GitHub

1. Workers & Pages → Create application → Import a repository.
2. Chọn repo `hunguyen1324/hquizlet-platform`, branch `main`.
3. Worker name: `hquizlet` (khớp `wrangler.jsonc`).
4. Root directory: `infra/cloudflare`.
5. Build command: `npm run check && npm test && npm run build`.
6. Deploy command: `npm run deploy`.
7. Sau deploy, Settings → Variables and Secrets → thêm biến **runtime** Text `UPSTREAM_ORIGIN`, giá trị URL tunnel HTTPS đang hoạt động, không kèm đường dẫn.

`keep_vars: true` giữ biến dashboard khi deploy lại. Biến build không thay thế biến runtime. Không commit token hoặc secret.

### Cấu hình đang dùng root `/`

Giữ build variable `SKIP_DEPENDENCY_INSTALL=1`. Thay Build command bằng:

```bash
npm --prefix infra/cloudflare install && npm --prefix infra/cloudflare run check && npm --prefix infra/cloudflare test && npm --prefix infra/cloudflare run build
```

Giữ Deploy command:

```bash
cd infra/cloudflare && npx wrangler deploy
```

Build script cài dependency frontend bằng `npm ci`, ép `VITE_GATEWAY_URL=/api`, build vào `apps/web/dist` và copy `_headers` riêng cho Cloudflare. Không cấu hình gateway thành localhost hoặc URL tunnel trong frontend. Runtime `UPSTREAM_ORIGIN` và `ORIGIN_PROXY_SECRET` giữ nguyên.

Assets có hash được cache dài; HTML kiểm tra lại, API/file động vẫn no-store. SPA fallback phục vụ trang con. Static frontend là công khai; quyền truy cập dữ liệu vẫn kiểm tra ở backend.

### Xác minh giai đoạn 3

- Trang chủ, refresh trang con, JS/CSS và `/templates/flashcard_template.xlsx` tải được.
- Mở `/api/healthz/services` trực tiếp trong trình duyệt vẫn trả JSON, không phải SPA HTML.
- Đăng nhập, tạo bộ thẻ, học/quiz, upload/download hoạt động.
- Thử ở môi trường kiểm tra với upstream không kết nối: frontend vẫn tải, API báo lỗi. Không cố ý dừng tunnel đang phục vụ người dùng để thử.
- Dashboard xác nhận assets không gọi Worker script không cần thiết.
- URL file tuyệt đối hoặc presigned từ backend vẫn cần kiểm tra thực tế; không tự đổi chữ ký URL.

Rollback: chọn deployment proxy trước đó trên Cloudflare và khôi phục Build command cũ. Giữ frontend Nginx trên server làm phương án quay lại. Sau rollback, kiểm tra biến runtime và secret.

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
npm test
npm run build
npx wrangler deploy --dry-run
```

Để chạy local, tạo `.dev.vars` (đã ignore) với `UPSTREAM_ORIGIN=https://YOUR-TUNNEL.trycloudflare.com`, sau đó `npm run dev`.

Nguồn: https://developers.cloudflare.com/workers/ci-cd/builds/configuration/ và https://developers.cloudflare.com/workers/wrangler/configuration/
