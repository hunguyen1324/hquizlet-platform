# Kế hoạch triển khai Cloudflare Free cho HQuizlet

Ngày lập: 06/10/2026.

## Mục tiêu

- Có URL cố định khi chưa mua tên miền.
- Hosting frontend trên Cloudflare để giảm tải server.
- Giữ backend và dữ liệu trên server nat-mini hiện tại.
- Thêm chống bot, theo dõi quota và quy trình phục hồi.
- Sử dụng gói Free trong phạm vi hạn mức được công bố.

URL người dùng cố định nhưng backend vẫn phụ thuộc Quick Tunnel. Khi tunnel mất kết nối, giao diện có thể tải được nhưng API tạm ngừng. Worker không khắc phục được lỗi kết nối tunnel.

## 1. Kiến trúc

```text
Người dùng
    |
    v
https://hquizlet.<tài-khoản>.workers.dev
    |
    +-- Giao diện, JS, CSS, template --> Workers Static Assets
    |
    +-- API và file động --> Worker proxy
                                |
                                v
                           Quick Tunnel
                                |
                                v
                       Nginx trên nat-mini
                                |
                                v
                     Gateway và các service
```

Chọn một Worker kèm Static Assets để frontend và API cùng hostname, giảm cấu hình CORS và số dự án cần quản lý.

Cloudflare cung cấp hostname `workers.dev`. Request phục vụ trực tiếp static assets miễn phí và không giới hạn; request gọi Worker script tính vào quota.

Nguồn: [workers.dev](https://developers.cloudflare.com/workers/configuration/routing/workers-dev/), [Static Assets](https://developers.cloudflare.com/workers/static-assets/billing-and-limitations/).

## 2. Phạm vi miễn phí

| Thành phần | Mục đích | Thời điểm |
|---|---|---|
| Workers Free | URL cố định và proxy API | Giai đoạn đầu |
| Workers Static Assets | Hosting frontend và template | Sau khi proxy ổn định |
| HTTPS của hostname Cloudflare | Truy cập qua HTTPS | Khi deploy Worker |
| Turnstile Free | Chống bot đăng ký, đăng nhập | Sau khi URL ổn định |
| Dashboard metrics và log | Theo dõi quota, lỗi, độ trễ | Ngay từ đầu |
| Cache trình duyệt | Tăng tốc file tĩnh có hash | Khi tách frontend |

Chưa chuyển database, Redis hoặc file storage. Đánh giá riêng các dịch vụ đó khi có nhu cầu thực tế.

Các tính năng quản lý theo domain như DNS, WAF Rules và Cache Rules cho zone sẽ đánh giá khi có tên miền; không mặc định cấu hình được toàn bộ cho hostname `workers.dev`.

## 3. Giai đoạn 1: ổn định tunnel

### Công việc

- Xác nhận tunnel dùng `--protocol http2 --edge-ip-version 4`.
- Kiểm tra DNS server và kết nối outbound tới Cloudflare.
- Xác nhận log có `Registered tunnel connection`.
- Kiểm tra health nội bộ và qua URL public.
- Kiểm tra sau một lần restart container.

HTTP/2 cần outbound TCP cổng `7844`. Nếu mạng chặn cổng này, đổi giao thức chưa đủ.

### Điều kiện hoàn thành

- Các service nội bộ đều `ok`.
- URL tunnel trả health thành công nhiều lần.
- Restart tạo được phiên mới và lấy được URL mới.

### Giới hạn

Quick Tunnel dành cho thử nghiệm, không có cam kết uptime; giới hạn 200 request đang xử lý đồng thời và không hỗ trợ SSE. Worker phía trước không loại bỏ các giới hạn này.

Nguồn: [Troubleshooting](https://developers.cloudflare.com/tunnel/troubleshooting/), [Quick Tunnels](https://developers.cloudflare.com/tunnel/get-started/quick-tunnels/).

## 4. Giai đoạn 2: URL cố định bằng Worker

Ban đầu proxy app hiện tại để kiểm tra luồng trước khi tách frontend.

### Cấu hình dự kiến

| Biến | Nội dung |
|---|---|
| `UPSTREAM_ORIGIN` | URL Quick Tunnel đang hoạt động |
| `PUBLIC_ORIGIN` | URL cố định của app |
| `ORIGIN_PROXY_SECRET` | Secret chung giữa Worker và Nginx |

### Yêu cầu proxy

- Lấy upstream từ cấu hình; không cho người gọi tự chọn đích.
- Giữ method, query string, body, Authorization và cookie cần thiết.
- Stream upload/download, tránh đọc toàn bộ file vào bộ nhớ.
- Kiểm tra redirect để tránh trả người dùng về hostname tunnel.
- Trả lỗi rõ ràng khi backend mất kết nối.
- Không tự retry thao tác ghi dữ liệu hoặc thanh toán.
- Không cache API trong giai đoạn đầu.
- Không log token, mật khẩu hoặc body nhạy cảm.

Nginx kiểm tra secret trên các đường được bảo vệ. Worker ghi đè header secret trước khi chuyển tiếp để người biết URL tunnel không dễ bỏ qua kiểm soát tại Worker. Thiết kế đường health và đường quản trị riêng, tránh làm hỏng kiểm tra nội bộ.

### Điều kiện hoàn thành

- Đăng nhập, đăng xuất, tạo bộ thẻ, học và quiz qua URL cố định.
- Upload/download hoạt động.
- Đổi upstream nhưng URL người dùng giữ nguyên.
- Truy cập trực tiếp đường bảo vệ qua tunnel thiếu secret bị từ chối.

## 5. Giai đoạn 3: frontend trên Static Assets

### Công việc

- Xác định lệnh build và output thực tế của `apps/web`.
- Deploy bản build lên Worker Static Assets.
- Cấu hình SPA fallback để refresh trang con không bị 404.
- Chỉ gọi Worker cho API và đường file động đã xác minh.
- Đưa template, JS, CSS và ảnh giao diện vào Static Assets.
- Kiểm tra API trong `apps/web/src/lib/api/client.ts` và ánh xạ Nginx.
- Kiểm tra URL file/ảnh backend trả về, tránh chứa tunnel cũ.

### Cache

| Nội dung | Chính sách |
|---|---|
| JS/CSS có hash | Cache dài, immutable |
| HTML | Kiểm tra lại để nhận bản mới |
| API tài khoản, dữ liệu riêng tư, thanh toán | private, no-store |
| File riêng tư | Kiểm tra quyền, tránh cache chung |
| Nội dung công khai | Chỉ cache sau khi kiểm tra quyền và cập nhật |

### Điều kiện hoàn thành

- Tắt tunnel thử nghiệm: giao diện vẫn tải và báo backend không khả dụng.
- Refresh trang chi tiết không bị 404.
- Deploy mới không để người dùng mắc ở bundle cũ.
- Static assets không gọi Worker script không cần thiết.

Nguồn: [Cache-Control](https://developers.cloudflare.com/cache/concepts/cache-control/).

## 6. Giai đoạn 4: tự cập nhật URL tunnel

Làm thủ công trước, sau đó tự động hóa.

### Quy trình

1. Theo dõi phiên container tunnel hiện tại.
2. Lấy URL từ log của phiên đó, tránh URL cũ.
3. Xác minh URL mới trả đúng health của app.
4. Cập nhật upstream qua API Cloudflare hoặc công cụ deploy.
5. Kiểm tra health qua URL cố định.
6. Lưu URL đã xác minh và kết quả cập nhật.

### Yêu cầu

- Token chỉ có quyền cần thiết trên Worker liên quan.
- Lưu token trên server trong file giới hạn quyền đọc.
- Khóa chống hai tác vụ cập nhật đồng thời.
- Chỉ cập nhật khi URL thay đổi.
- Retry có khoảng nghỉ; tránh restart liên tục.
- Không làm mất secret, cấu hình hoặc asset deployment khi cập nhật.

### Điều kiện hoàn thành

Restart tunnel và URL cố định tự phục hồi. Chấp nhận gián đoạn trong thời gian tạo và xác minh tunnel mới.

## 7. Giai đoạn 5: Turnstile

Ưu tiên đăng ký, đăng nhập và quên mật khẩu nếu có.

### Công việc

- Tạo widget cho hostname cố định.
- Site key đặt ở frontend; secret key đặt ở backend.
- Frontend gửi token cùng request.
- Backend xác minh Siteverify trước khi thực hiện hành động.
- Xử lý token thiếu, sai, hết hạn hoặc đã dùng.
- Giữ rate limit backend như lớp kiểm soát bổ sung.

### Điều kiện hoàn thành

Request bỏ qua widget không thể thực hiện hành động yêu cầu Turnstile.

Turnstile Free có tối đa 20 widget, không giới hạn lượt challenge. Xác minh phía server là bắt buộc.

Nguồn: [Turnstile plans](https://developers.cloudflare.com/turnstile/plans/), [Integration](https://developers.cloudflare.com/turnstile/get-started/).

## 8. Giai đoạn 6: quota, bảo mật và theo dõi

Workers Free hiện có 100.000 request/ngày và 10 ms CPU mỗi invocation. Static assets được phục vụ trực tiếp không tiêu quota request Worker script. Kiểm tra lại hạn mức trước khi triển khai vì dịch vụ có thể thay đổi.

Ví dụ dự toán: 500 người/ngày × 100 API request/người = 50.000 request/ngày. Cần đo thực tế vì polling, retry và tải danh sách có thể tăng số lượng.

### Công việc

- Theo dõi request/ngày, 429, 5xx và độ trễ.
- Giảm polling và request trùng; phân trang danh sách lớn.
- Health check khoảng 5 phút/lần.
- Thêm X-Content-Type-Options, Referrer-Policy và chống nhúng trang.
- Thử CSP ở chế độ báo cáo trước khi chặn để tránh ảnh hưởng Turnstile.
- Ghi request ID để đối chiếu Worker và backend.
- Tiếp tục backup database trên server.

Nguồn: [Workers limits](https://developers.cloudflare.com/workers/platform/limits/).

## 9. Kiểm tra trước khi chuyển URL chính

| Nhóm | Trường hợp |
|---|---|
| Tài khoản | Đăng ký, đăng nhập, refresh token, đăng xuất |
| Học tập | Tạo/sửa bộ thẻ, học, quiz, lưu kết quả |
| Quyền | Tài khoản khác không đọc được dữ liệu riêng tư |
| File | Upload, download, ảnh, import template |
| Thanh toán | Luồng thử nghiệm, webhook nếu sử dụng |
| Kết nối | Tunnel chết, backend chậm, DNS lỗi |
| Deploy | Frontend mới, Worker mới, đổi upstream |
| Chống bot | Token thiếu, sai, hết hạn, dùng lại |

Kiểm tra thanh toán và thao tác ghi để tránh thực hiện hai lần khi request được gửi lại.

## 10. Rollback

- Giữ app Docker và dữ liệu trên server trong quá trình triển khai.
- Sao lưu Nginx và Compose trước khi sửa.
- Giữ deployment Worker/frontend trước để rollback.
- Tự cập nhật lỗi: chuyển sang cập nhật upstream thủ công.
- Turnstile lỗi: rollback phiên bản tích hợp.
- Khi quay lại tunnel trực tiếp, điều chỉnh kiểm tra secret có chủ đích.

## 11. Lịch dự kiến và đầu ra

Ước tính 3–5 buổi, phụ thuộc lỗi mạng server.

| Buổi | Kết quả |
|---|---|
| 1 | Tunnel ổn định, Worker có URL cố định |
| 2 | Frontend trên Static Assets |
| 3 | Tự cập nhật URL tunnel |
| 4 | Turnstile và kiểm soát truy cập |
| 5 | Kiểm tra, quota, hướng dẫn vận hành |

### Đầu ra cần bàn giao

- Mã Worker proxy và cấu hình Static Assets.
- Cấu hình tunnel và Nginx đã xác minh.
- Script cập nhật upstream cùng cơ chế chạy định kỳ/theo dõi.
- Tích hợp Turnstile frontend/backend.
- Tài liệu deploy, rollback và xử lý sự cố.
- Kết quả kiểm tra chức năng và số liệu quota ban đầu.

### Điều kiện bắt đầu

- Tunnel HTTP/2 kết nối được.
- Có tài khoản Cloudflare Free.
- Xác nhận hostname Worker muốn dùng và quyền triển khai tài khoản.
- Token/secret được nhập qua môi trường triển khai, không commit vào Git.

Bước đầu tiên: xác nhận tunnel HTTP/2 hoạt động, sau đó tạo Worker. Chưa cần mua domain hoặc chuyển dữ liệu khỏi server.
