# Vận hành IAM và kiểm thử dashboard

## Kiểm thử trên máy phát triển

Từ repo root, chạy bằng Bash (Git Bash trên Windows):

```bash
bash infra/scripts/phase10-e2e.sh
```

Script tạo project `hq-iam-e2e-<timestamp>-<pid>` với network và volume riêng, dùng `infra/docker/docker-compose.iam-e2e.yml`. Gateway test chỉ bind `127.0.0.1:18080`; PostgreSQL/Redis/NATS/MinIO không publish port. Cấu hình test dùng địa chỉ và thông tin đăng nhập local cố định, không đọc `DATABASE_URL` trong `.env` để kết nối DB thật. Nếu Windows trỏ `python3` tới Microsoft Store, đặt `IAM_E2E_PYTHON` thành đường dẫn Python 3 thực tế trước khi chạy. Cần để trống cổng 18080 và cài Docker Compose hỗ trợ `up --wait`, curl, Python 3.

Khi kết thúc (thành công hoặc lỗi), script chỉ xoá container/network/volume của project test vừa tạo. Khi lỗi, script in log trước khi dọn. Nếu tiến trình bị kill cưỡng bức, dùng đúng tên project in ở đầu log để dọn thủ công; không dùng tên project của stack phát triển.

Workflow `dashboard-iam-gate.yml` chạy test Go, contract test/build web và E2E trên pull request có thay đổi liên quan, hoặc qua `workflow_dispatch`.

## Cấp admin đầu tiên

Đăng ký tài khoản bình thường trước, xác minh đúng ID/email bằng truy vấn chỉ đọc vào DB auth. Người vận hành có quyền DB có thể bootstrap assignment `PlatformAdmin` ở scope `platform` cho ID đã xác minh. Thực hiện trong transaction và ghi `audit_log` với `iam:AssignRole`, resource `hq:user/<ID>`, result `allow`, detail chứa nguồn `bootstrap` và lý do.

Sau bootstrap, đăng xuất/đăng nhập lại rồi kiểm tra `GET /v1/me/permissions`: phải có `PlatformAdmin` và `dashboard:ViewAdmin`. Việc gán/thu hồi tiếp theo dùng API admin `/v1/admin/assignments` để ghi audit. Không cho người dùng tự chọn admin trong đăng ký.

## Thu hồi và vô hiệu hoá

- Lấy đúng assignment ID bằng `GET /v1/admin/assignments?userId=<ID>`, rồi `DELETE /v1/admin/assignments/<assignmentID>`.
- Thu hồi assignment PlatformAdmin cuối cùng đồng thời hạ role admin legacy và ghi audit trong cùng transaction; migration khởi động lại không khôi phục grant đã thu hồi.
- Để khoá tài khoản, dùng `POST /v1/admin/users/<ID>/disable`. Auth từ chối đăng nhập mới và từ chối xác thực các session của tài khoản disabled. Thao tác disable thu hồi các session trong cùng transaction với audit; bật lại không phục hồi session cũ. Gateway có thể còn cache identity trong thời gian TTL (hiện 60 giây); cần kiểm tra lại sau khi cache hết hạn. Admin không được tự vô hiệu hoá chính mình.
- Kiểm tra quyền thực tế bằng token của tài khoản bị thu hồi sau thời gian cache, không chỉ dựa vào danh sách assignment.

## Đọc audit

`GET /v1/admin/audit?limit=50&offset=0` trả `entries`; dùng `actorId` và `action` để lọc. Theo dõi `iam:AssignRole`, `iam:RevokeRole`, `user:Disable` và các entry `result='deny'`. Kết quả gán/thu hồi thành công cần đối chiếu target trong `resource` và assignment trong detail.

Gán PlatformAdmin và cộng tiền ví yêu cầu `X-Admin-Password`. Mật khẩu được kiểm tra tại auth; cộng tiền kiểm tra lại quyền hiện tại. Ledger ví và audit được ghi trong cùng transaction. API admin auth xác thực quyền hiện tại mỗi yêu cầu. Audit nội dung riêng tư dùng `study-set:Read`; ẩn/khôi phục dùng `study-set:Publish` và bắt buộc lý do. Không ghi mật khẩu vào audit hoặc log.

E2E kiểm tra cô lập dashboard lớp, sự kiện hoàn thành qua NATS, quyền live/giá bán, xác nhận mật khẩu, ledger/audit, metrics, lọc người dùng và kiểm duyệt. Phase 5 (tổ chức/trường học) được hoãn theo quyết định của chủ dự án. Backfill production phải dry-run trên bản sao dữ liệu thật trước khi áp dụng; bộ E2E chỉ dùng dữ liệu tổng hợp.

## Bootstrap bằng SQL (chỉ cho người vận hành DB)

Đặt `admin_id` thành ID đã xác minh. Mẫu dưới đây không được chạy tự động trong bộ kiểm thử hoặc trên production:

```sql
\set admin_id 123
BEGIN;
SELECT id,email,disabled FROM users WHERE id=:'admin_id'::bigint FOR UPDATE;
WITH new_grant AS (
  INSERT INTO iam_assignments(principal_type,principal_id,role_key,scope_type,scope_id,created_by)
  SELECT 'user',id,'PlatformAdmin','platform',NULL,id FROM users
  WHERE id=:'admin_id'::bigint AND NOT disabled
  ON CONFLICT(principal_type,principal_id,role_key,scope_type,scope_id) DO NOTHING
  RETURNING id,principal_id
)
INSERT INTO audit_log(actor_id,action,resource,result,detail)
SELECT principal_id,'iam:AssignRole','hq:user/'||principal_id,'allow',
       jsonb_build_object('source','bootstrap','roleKey','PlatformAdmin','assignmentId',id)
FROM new_grant;
COMMIT;
```

Nếu không có dòng phù hợp hoặc đã có grant, mẫu không tạo thêm audit cấp quyền thành công. Sau bootstrap dùng phiên đăng nhập mới để tránh cache gateway. Không cập nhật `users.role` thành admin cho tài khoản mới.

## Chữ ký và hiệu lực quyền

- `INTERNAL_SIGNING_KEY` phải giống nhau giữa gateway và các service; production phải đặt khóa riêng. Dashboard lớp gọi progress bằng HMAC-SHA256 gắn với method, đường dẫn, query và timestamp, hiệu lực 30 giây. Không đưa khóa này vào frontend. `/internal/progress/summary` không được publish qua gateway.
- Grant đã hết hạn không khôi phục PlatformAdmin qua role legacy. Thu hồi admin cập nhật quyền tại auth ngay cho API quản trị auth; cộng ví kiểm tra lại quyền sau xác nhận mật khẩu. Các API khác có thể còn identity cache gateway tối đa 60 giây.
- Giới hạn admin hiện tại: 120 lượt đọc và 20 lượt ghi mỗi phút theo user, trên mỗi gateway process. Khi scale nhiều gateway cần rate-limit tập trung ở reverse proxy hoặc Redis.

## Kiểm thử tải và backfill

`infra/scripts/dashboard-load.py` dùng `IAM_LOAD_TOKEN` và `IAM_LOAD_CLASS_ID`, gửi 48 yêu cầu với concurrency 4, yêu cầu p95 dưới 1.500 ms. E2E gọi tự động với token test. Không truyền token trên command line hoặc in token trong báo cáo.

`infra/scripts/dashboard-explain.sql` tạo bản sao bảng tạm 50.000 phiên, chạy EXPLAIN ANALYZE trước/sau chỉ mục rồi rollback. Chạy trên PostgreSQL của project E2E riêng. Không phải benchmark production.

Dry-run hỗ trợ `COMPOSE_FILE` và `COMPOSE_PROJECT_NAME` để chọn đúng stack test, hoặc `DATABASE_URL` khi có psql. Không dùng `--apply` trên production trước khi đã xem báo cáo trên bản sao dữ liệu thật. Student chỉ được backfill khi tài khoản chưa có platform assignment, nên giữ nguyên Teacher và các role đã quản lý.
