# Rate limit – thiết kế và vận hành

Tài liệu này mô tả phần rate limit đã triển khai theo
[`docs/plans/rate-limit-plan.md`](../plans/rate-limit-plan.md) (RL-1 → RL-5, RL-7).
Chưa làm: Redis cho nhiều replica (RL-6) và Turnstile (RL-8) — xem mục
[Chưa triển khai](#chưa-triển-khai).

## 1. Ba lớp

| Lớp | Ở đâu | Nhiệm vụ |
|---|---|---|
| nginx | `infra/nginx/nginx.conf` | Chặn lũ request rẻ tiền trước khi vào app (`limit_req`, `limit_conn`), trả `429`. |
| Gateway | `services/gateway/ratelimit.go` | Ngưỡng chính, có ngữ cảnh: theo IP cho route công khai, theo user cho route đã xác thực, theo (IP + mã phòng) cho live join. |
| Service | `services/study` (quiz bảo vệ), `services/payment` (đơn nạp), `services/auth` (đăng nhập sai) | Nghiệp vụ: đếm lỗi theo tài khoản, quota upload, … |

Thuật toán ở gateway là **token bucket** trong bộ nhớ tiến trình (`memoryLimiter`),
mỗi khoá có bucket riêng theo `rule.Name`, nên cùng một IP vẫn có ngân sách độc lập
cho từng nhóm route. Bộ nhớ bị chặn trên (`RATE_LIMIT_MAX_KEYS`, mặc định 100.000
khoá) và được dọn định kỳ: bucket idle > 10 phút bị xoá, khi bị flood khoá lạ thì
loại bớt 10% số khoá cũ nhất.

Đếm "sai mã phòng" dùng `failureTracker` (cửa sổ cố định): sau 60 lần trả `404` cho
endpoint join, IP đó bị khoá hết cửa sổ 1 giờ.

## 2. Lấy đúng IP khách (RL-1)

`services/gateway/clientip.go` chỉ tin `X-Forwarded-For` / `X-Real-IP` /
`CF-Connecting-IP` khi request đến từ dải tin cậy (`TRUSTED_PROXIES`). Chuỗi
forwarded được duyệt từ **phải sang trái**, bỏ qua các hop tin cậy, lấy địa chỉ
không tin cậy đầu tiên — nên giá trị do client tự gửi ở đầu chuỗi bị bỏ qua.

- Mặc định (khi `TRUSTED_PROXIES` trống): loopback, `10/8`, `172.16/12`,
  `192.168/16`, `169.254/16`, IPv6 ULA `fc00::/7`, link-local `fe80::/10`.
  Đủ cho nginx/Docker trên cùng máy; **đặt rõ** khi hạ tầng khác đi.
- Gateway thay `X-Forwarded-For` gửi upstream bằng IP đã phân giải (bỏ giá trị
  client gửi) và xoá `X-Real-IP`, nên service phía sau nhận đúng IP khách.
- Đứng sau Cloudflare: đặt `TRUSTED_PROXIES` bằng dải IP Cloudflare
  (<https://www.cloudflare.com/ips/>) và bật khối `real_ip_*` đã comment trong
  `nginx.conf`. Khi nginx nhận request trực tiếp từ Internet thì **không** bật
  `real_ip`, vì client có thể tự khai `X-Forwarded-For`.

## 3. Ngưỡng đang áp dụng

### Gateway – route công khai (khoá theo IP)

| Route | Rule | Ngưỡng |
|---|---|---|
| `POST /v1/auth/login` | `auth.login.ip` | 10/phút |
| `POST /v1/auth/register` | `auth.register.ip` | 5/giờ |
| `POST /v1/auth/reauth` | `auth.reauth.ip` | 10/phút |
| `POST /v1/live-sessions/{code}/join` | `live.join.ip` | 20/phút |
| `POST /v1/live-sessions/{code}/join` | `live.join.code` | 10/phút cho mỗi (IP + mã) |
| join trả `404` | `live.join.badcode` | 60 lần sai/giờ → khoá 1 giờ |
| route khác, **có** Bearer | `api.user` theo fingerprint của token | 300/phút, burst 60 |
| route khác, không Bearer | `api.ip` | 900/phút, burst 120 |

Request có Bearer được tính theo **credential** (băm SHA-256 của token, không lưu
token thô), không theo IP, nên một lớp học/NAT dùng chung IP không bị chặn oan.
Các route đã verify token còn có thêm ngân sách theo user id ở proxy (`api.user`),
ngưỡng như nhau nên hành vi không đổi.
`/healthz`, `/healthz/services`, `/metrics` và SSE (`GET …/events`) được miễn.

### Gateway – đã xác thực (khoá theo user / participant)

| Nhóm | Rule | Ngưỡng |
|---|---|---|
| API thường (sau khi verify token) | `api.user` | 300/phút, burst 60 |
| Admin console – đọc (`GET`) | `admin.read.user` | 120/phút |
| Admin console – ghi | `admin.write.user` | 20/phút |
| Live answers / player-state | `live.player.ip` | 60/phút cho mỗi (IP + phòng) |

### Auth service – chống dò mật khẩu (RL-3)

`services/auth/internal/http/login_guard.go` đếm **số lần đăng nhập sai**:

| Khoá | Ngưỡng | Thời gian chờ |
|---|---|---|
| mỗi (email + IP) | 5 lần / 15 phút | 30s, 60s, 2 phút, … tối đa 15 phút |
| mỗi email (mọi IP) | 20 lần / 15 phút | 5 phút, tăng dần, tối đa 15 phút |

Đăng nhập thành công xoá ngân sách của tài khoản. Câu trả lời `429` giống nhau cho
email tồn tại và không tồn tại, nên không lộ tài khoản.

### nginx (RL-5)

| Vị trí | Cấu hình |
|---|---|
| `/api/v1/auth/` | `limit_req zone=api_auth burst=20 nodelay` (5r/s/IP) |
| `/api/` | `limit_req zone=api_general burst=60 nodelay` (30r/s/IP) |
| SSE `/api/v1/live-sessions/*/events` | `limit_conn api_conn 100` (số kết nối/IP), không `limit_req` |
| video upload/stream/HLS | không `limit_req`, không `limit_conn` (stream dài) |

`limit_req_status 429; limit_conn_status 429;` và `error_page 429` trả JSON cùng
định dạng lỗi của gateway kèm `Retry-After: 60`.

Ngưỡng nginx cố ý rộng hơn gateway 2–3 lần: nginx chỉ giảm tải, gateway mới là nơi
quyết định đúng/sai theo route.

## 4. Phản hồi khi vượt ngưỡng

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 30
Content-Type: application/json

{"code":"RATE_LIMITED","message":"too many requests, retry in 30s","requestId":"…","details":{"retryAfter":30}}
```

Frontend (`apps/web/src/lib/api/client.ts`, `features/live/liveApi.ts`) đổi `429`
thành câu tiếng Việt: *“Thao tác quá nhanh, vui lòng thử lại sau N giây.”*

## 5. Cấu hình

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `RATE_LIMIT_ENABLED` | `true` | `false` = tắt toàn bộ limiter ở gateway (nginx vẫn chặn). |
| `RATE_LIMIT_SHADOW` | `false` | `true` = chỉ ghi log/metric, **không** trả 429. Dùng để chạy thử trên production. |
| `RATE_LIMIT_MAX_KEYS` | `100000` | Số khoá tối đa trong bộ nhớ. |
| `TRUSTED_PROXIES` | rỗng → dải private | Danh sách CIDR được tin header chuyển tiếp. |

Chỉnh ngưỡng: sửa các biến `rule*` ở đầu phần “Rules” trong
`services/gateway/ratelimit.go` (và `badCodeLimit` / `badCodeWindow`), hoặc các
hằng số `login*` trong `services/auth/internal/http/login_guard.go`.

Quy trình chỉnh an toàn:

1. Bật `RATE_LIMIT_SHADOW=true` trên production vài ngày.
2. Xem `rate_limited_total{mode="shadow"}` để biết rule nào sẽ chặn ai.
3. Chỉnh ngưỡng, rồi tắt shadow.

## 6. Quan sát (RL-7)

- `GET /metrics` trên gateway (chỉ trong mạng nội bộ, không qua nginx):
  ```
  rate_limited_total{route="POST /v1/auth/login",rule="auth.login.ip",mode="blocked"} 12
  rate_limited_total{route="POST /v1/auth/login",rule="auth.login.ip",mode="shadow"} 3
  ```
- Log có cấu trúc:
  `[gateway] rate_limited route=… rule=… key=ip:1.2.3.4 retry_after=30s`
  (`rate_limit_shadow` khi ở shadow mode).
- Gợi ý cảnh báo: `sum(rate(rate_limited_total{mode="blocked"}[5m]))` tăng đột biến;
  riêng `rule="auth.login.ip"` tăng mạnh là dấu hiệu dò mật khẩu.

## 7. Kiểm thử

```bash
cd services/gateway && go test ./...   # limiter, IP thật, join lockout, metrics
cd services/auth && go test ./...      # backoff đăng nhập sai
```

Kiểm tra nginx (không cần dựng cả stack):

```bash
docker run --rm -v "$PWD/infra/nginx/nginx.conf:/etc/nginx/nginx.conf:ro" nginx:alpine nginx -t
```

## 8. Lưu ý khi vận hành

- **NAT / lớp học**: luật IP phải rộng; luật theo user/tài khoản mới là chặn chính.
  Nếu 40 học sinh chung một IP bị chặn, tăng `ruleLiveJoinIP` hoặc `api.ip`.
- **SSE**: không đặt `limit_req` cho `/events` (mỗi lần reconnect là một request);
  dùng `limit_conn`.
- **Khoá tài khoản**: chỉ chờ tăng dần theo (email + IP), không khoá vĩnh viễn, và
  ngưỡng theo email đủ cao (20/15 phút) để không ai khoá hộ người khác.
- **Sai múi giờ/đồng hồ**: bucket dựa trên `time.Now()` của tiến trình.
- **Header IP giả**: chỉ có hiệu lực nếu `TRUSTED_PROXIES` bị đặt quá rộng (ví dụ
  tin cả `0.0.0.0/0`). Giữ danh sách hẹp.

## 9. Chưa triển khai

- **RL-6 – Redis cho nhiều replica**: `Limiter` đã là interface nên chỉ cần thêm
  implementation dùng Lua `INCR + EXPIRE` và biến `RATE_LIMIT_REDIS_URL`. Hiện mỗi
  Compose chỉ chạy **một** gateway nên bộ nhớ trong tiến trình là đủ; nếu scale
  ngang mà chưa làm RL-6, mỗi replica sẽ có ngân sách riêng (ngưỡng thực tế × số
  replica).
- **RL-8 – Turnstile** cho đăng ký/đăng nhập (xem `CLOUDFLARE-FREE-PLAN.md`).
- **RL-0 – đo lưu lượng thật** để tinh chỉnh ngưỡng: chưa thu thập.
