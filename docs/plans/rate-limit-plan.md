# Kế hoạch thêm Rate Limit cho hquizlet-platform

> Trạng thái: kế hoạch, chưa sửa code. Dựa trên code ở commit `59a9158` (main).

## 1. Hiện trạng: đã có gì, chưa có gì

| Lớp | Hiện trạng | Vị trí |
|---|---|---|
| **Quiz bảo vệ (theo user)** | **Có.** Đếm theo user trong Postgres (`protected_quiz_limits`): `summary` 30/phút, `start` 20/giờ, `play`/`peek` 120/phút, `audio` 300/phút, `upload` 20/giờ. Trả lỗi `ErrQuizRateLimit`. | `services/study/internal/service/protected_quiz.go`, `repository/quiz_session.go` |
| **Thanh toán** | **Có, một phần.** Giới hạn số đơn nạp đang chờ (cửa sổ 10 phút). | `services/payment/internal/service/order_svc.go` |
| **Quiz live – join** | **Chưa.** Hàm `rateLimitProxy` chỉ xoá header danh tính, dòng chú thích ghi "Basic rate limit could be added here". Endpoint công khai, ai cũng gọi được (có thể dò mã phòng). | `services/gateway/main.go` (`rateLimitProxy`) |
| **Đăng nhập / đăng ký / quên mật khẩu** | **Chưa thấy.** `/v1/auth/*` được proxy thẳng, không có giới hạn ở gateway; chưa thấy trong service auth. Dễ bị dò mật khẩu / spam. | `services/gateway/main.go` (`authProxy`), `services/auth` |
| **Gateway nói chung** | **Chưa.** Không có giới hạn theo IP hay theo user cho các route còn lại. | `services/gateway/main.go` |
| **nginx** | **Chưa.** Không có `limit_req`/`limit_conn`. | `infra/nginx/nginx.conf` |
| **Cloudflare / Turnstile** | Chỉ có trong tài liệu `CLOUDFLARE-FREE-PLAN.md` (giai đoạn 5), chưa triển khai. | — |

**Kết luận:** quiz đã có rate limit theo user; **phần mặt tiền (đăng nhập, join live, IP, nginx) thì chưa**. Đây là chỗ rủi ro nhất.

## 2. Vấn đề phải xử lý trước: lấy đúng IP khách

Gateway đang đặt `X-Forwarded-For = r.RemoteAddr` (dòng ~423 và ~600). Khi chạy sau nginx/Cloudflare, `RemoteAddr` là IP của nginx/tunnel, **mọi người dùng sẽ bị tính chung một IP** nếu limit theo IP.

Việc cần làm:
1. nginx: cấu hình `set_real_ip_from` (dải IP tin cậy) + `real_ip_header` (`CF-Connecting-IP` nếu qua Cloudflare, nếu không thì `X-Forwarded-For`).
2. Gateway: chỉ tin `X-Forwarded-For`/`X-Real-IP` khi request đến từ proxy tin cậy (cấu hình `TRUSTED_PROXIES`); lấy IP đầu tiên ngoài dải tin cậy. Không tin header do client tự gửi.
3. Test: giả `RemoteAddr` là proxy + header giả → phải ra đúng IP; request trực tiếp kèm header giả → bị bỏ qua.

## 3. Thiết kế

Ba lớp, mỗi lớp một nhiệm vụ:

1. **nginx (thô, rẻ):** chặn lũ request từ một IP trước khi chạm app. `limit_req_zone` theo `$binary_remote_addr` (sau real_ip), `limit_conn`. Có `burst`, trả `429`.
2. **Gateway (chính, có ngữ cảnh):** middleware rate limit theo **IP** (route công khai) và theo **user ID** (route đã xác thực), cấu hình theo từng nhóm route.
3. **Service (nghiệp vụ):** giữ nguyên các giới hạn hiện có (quiz, đơn nạp). Không nhân đôi logic.

**Thuật toán:** token bucket. Một nhà (1 gateway) → bộ nhớ trong tiến trình (`golang.org/x/time/rate` hoặc tự viết, kèm dọn key cũ). Nhiều replica → Redis (đã có sẵn trong stack) dùng script Lua `INCR + EXPIRE` hoặc sliding window; key luôn có tiền tố nhóm + IP/user ID.

**Khi Redis lỗi:** route nhạy cảm (đăng nhập, join) → **fail-closed** hạn chế (dùng bucket cục bộ thay thế); route thường → fail-open + log cảnh báo.

**Phản hồi khi vượt ngưỡng:** HTTP `429`, header `Retry-After`, thân JSON theo định dạng lỗi hiện có của gateway (`writeGatewayError`, mã `RATE_LIMITED`). Frontend hiển thị "Thao tác quá nhanh, thử lại sau N giây".

### Bảng ngưỡng đề xuất (điểm khởi đầu, chỉnh sau khi đo)

| Nhóm route | Khoá | Ngưỡng | Ghi chú |
|---|---|---|---|
| `POST /v1/auth/login` | IP **và** tài khoản (email) | 10/phút/IP; 5 lần sai/15 phút/tài khoản | Khoá mềm tạm thời + tăng dần thời gian chờ |
| `POST /v1/auth/register`, quên/đặt lại mật khẩu | IP | 5/giờ | Sau này thêm Turnstile |
| `POST /v1/auth/refresh` | user | 30/phút | |
| `POST /v1/live-sessions/{code}/join` | IP + mã phòng | 20/phút/IP; 60 lần sai mã/giờ/IP | Chống dò mã phòng |
| Live: gửi đáp án, player-state | participant | 60/phút | |
| API đã đăng nhập chung | user | 300/phút, burst 60 | Mặc định rộng |
| Upload (file, video) | user | theo quota riêng (đã có `upload` 20/giờ cho quiz) | |
| Quiz bảo vệ | user | **giữ nguyên** | Xem tối ưu ở kế hoạch hiệu năng (chuyển sang Redis/RAM) |

## 4. Các bước thực hiện (mỗi bước một PR)

**RL-0 – Chuẩn bị (0,5 ngày)**
- Ghi lại lưu lượng thật (log gateway: route, IP, user, mã trạng thái) trong vài ngày để chọn ngưỡng.
- Thêm biến cấu hình: `TRUSTED_PROXIES`, `RATE_LIMIT_ENABLED`, `RATE_LIMIT_REDIS_URL`.

**RL-1 – IP thật (0,5–1 ngày)** — mục 2. *Xong khi:* test IP pass; log gateway hiển thị IP khách đúng.

**RL-2 – Middleware rate limit ở gateway (1–2 ngày)**
- File mới `services/gateway/ratelimit.go`: interface `Limiter` (`Allow(key, rule) (ok bool, retryAfter)`), cài đặt in-memory trước, Redis sau.
- Bọc `mux` bằng middleware tra bảng luật theo method + đường dẫn.
- Có chế độ "chỉ ghi log, không chặn" (shadow mode) để chạy thử trên production.
- *Xong khi:* test đơn vị (vượt ngưỡng → 429 + `Retry-After`; hết cửa sổ → cho qua; key khác nhau không ảnh hưởng nhau; dọn bộ nhớ key cũ).

**RL-3 – Áp cho đăng nhập & đăng ký (1 ngày)**
- Luật theo IP ở gateway + đếm số lần sai theo tài khoản trong service `auth` (gateway không biết đăng nhập đúng/sai).
- Phản hồi không tiết lộ tài khoản có tồn tại hay không.
- *Xong khi:* 11 lần đăng nhập/phút từ một IP → 429; 5 lần sai liên tiếp một tài khoản → bị chờ.

**RL-4 – Áp cho live join (0,5–1 ngày)**
- Thay chỗ ghi chú trong `rateLimitProxy` bằng giới hạn IP + mã phòng, đếm lần dò sai.
- *Xong khi:* quét thử 100 mã phòng từ một IP bị chặn sau ngưỡng.

**RL-5 – nginx (0,5 ngày)**
- `limit_req_zone`/`limit_conn_zone`, áp cho `/api/v1/auth/` và `/api/` (burst hợp lý), loại trừ SSE (`/events`) và stream media khỏi `limit_req`.
- Trả `429` (`limit_req_status 429`).

**RL-6 – Redis cho nhiều replica (1 ngày, chỉ khi cần)**
- Cài đặt `Limiter` bằng Redis (Lua atomic), fail-open/closed theo nhóm.

**RL-7 – Quan sát & tài liệu (0,5 ngày)**
- Số đếm `rate_limited_total{route,rule}` ở endpoint metrics hoặc log có cấu trúc; cảnh báo khi tăng đột biến.
- Cập nhật `README`/`docs/security`: bảng ngưỡng, cách chỉnh, cách tạm tắt (`RATE_LIMIT_ENABLED=false`).

**RL-8 – Turnstile cho đăng ký/đăng nhập (xem `CLOUDFLARE-FREE-PLAN.md` mục 7)** — làm sau, bổ sung chứ không thay rate limit.

## 5. Rủi ro & lưu ý

- **Nhiều người dùng chung một IP** (trường học, quán net, NAT): luật IP phải rộng; luật theo tài khoản/user mới là chặn chính. Với live join trong lớp học, 20/phút/IP có thể thấp — nên đo rồi chỉnh.
- **Không chặn nhầm SSE/stream**: loại khỏi `limit_req` của nginx; giới hạn số kết nối đồng thời thay vì số request.
- **Khoá tài khoản có thể bị lợi dụng** để khoá người khác: dùng chờ tăng dần theo (tài khoản + IP), không khoá cứng vĩnh viễn.
- **Header IP giả:** chỉ tin khi đến từ proxy tin cậy (mục 2).
- **Bộ nhớ:** limiter in-memory phải giới hạn số key và dọn định kỳ để không bị tấn công làm tràn RAM.

## 6. Câu hỏi cần bạn xác nhận

1. Hệ thống chạy **một** gateway hay nhiều replica? (Quyết định in-memory hay Redis.)
2. Có đặt sau Cloudflare không (để lấy IP từ `CF-Connecting-IP`)?
3. Live join chủ yếu dùng trong lớp học chung mạng? Cần ngưỡng IP rộng hơn không?
4. Dịch vụ `auth` hiện có đếm đăng nhập sai chưa? (Mình chưa thấy; nếu bạn biết chỗ khác đã xử lý, cho mình biết.)
