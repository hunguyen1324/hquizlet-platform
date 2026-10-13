# Nghiệm thu Dashboard / IAM — 2026-10-13

Hoàn thành code trong phạm vi Phase 1–4 và hardening Phase 6. Phase 5 (tổ chức/trường học, lời mời, dashboard tổ chức, role tùy chỉnh) được hoãn theo quyết định của chủ dự án. Chưa triển khai production.

## Kết quả kiểm chứng

| Kiểm tra | Kết quả |
|---|---|
| `go test` authz + auth/class/study/gateway/payment/quiz | Đạt; class đã chạy lại sau bản sửa scope giao bài cuối |
| Web Vitest | 20/20 tests đạt |
| Web TypeScript + Vite production build | Đạt; còn cảnh báo bundle lớn |
| Redocly lint OpenAPI | 0 lỗi, 123 cảnh báo (license, operationId và các quy tắc khuyến nghị) |
| `phase10-e2e.sh` | PASS trên project `hq-iam-e2e-1791900937-2019`; container/network/volume test đã tự dọn |
| Dashboard API load | 48 requests, concurrency 4; p50 12,6 ms, p95 45,3 ms, max 57,1 ms |
| Backfill dry-run | 0 assignment thiếu; Teacher không bị đổi thành Student sau restart |
| EXPLAIN ANALYZE, 50.000 phiên trên bảng tạm | Phiên gần nhất: 12,387 → 0,125 ms; tổng hợp ngày: 0,704 → 0,282 ms |

Tải API dùng dữ liệu tổng hợp nhỏ của E2E. Benchmark SQL 50.000 phiên là kiểm tra riêng trên bảng tạm, không phải đo toàn bộ API với 50.000 phiên hoặc cam kết tốc độ production. Chỉ mục bổ sung: `learning_sessions(user_id,created_at DESC)`, `learning_sessions(created_at)` và chỉ mục outbox chưa publish.

## Các luồng E2E đã chứng minh

- Đăng ký theo Teacher/Student; học sinh và giáo viên không vào API admin, không tự cấp admin; gateway bỏ header giả mạo.
- Học sinh không tạo lớp, host live hoặc đặt giá bán; host live có scope lớp đúng.
- Dashboard học sinh chỉ trả tiến độ của mình; người ngoài lớp bị chặn. Deadline và quyền theo lớp dùng dữ liệu server.
- Học sinh học flashcard của giáo viên lưu được tiến độ của chính mình. Sự kiện hoàn thành được tiêu thụ qua NATS.
- Protected quiz chấm điểm server, ghi một learning session; nộp lại không nhân đôi tiến độ hoặc sự kiện.
- Admin sửa nội dung khác chủ sở hữu và giao/gỡ bài trên lớp của người khác; giáo viên ngoài lớp bị chặn qua test policy.
- Cộng ví từ chối mật khẩu thiếu/sai, ghi ledger và audit khi thành công; PlatformSupport không cộng ví.
- Gán admin yêu cầu mật khẩu; thu hồi admin chặn ngay API quản trị auth và cộng ví dù gateway còn cache identity.
- Console đọc metrics, người dùng/role/status, nội dung, lớp, hệ thống/import queue. Policy role được trả từ chính thư viện authz để xem read-only.
- Xem nội dung quản trị được audit; ẩn nội dung chặn API đọc thông thường; khôi phục hoạt động. Activity có metadata NULL vẫn đọc được.
- Khóa tài khoản từ chối login/session; session bị thu hồi không được phục hồi qua restart. Audit có đủ gán/thu hồi role, khóa user, cộng ví, xem và kiểm duyệt nội dung.

## Chạy lại

```bash
go test ./packages/authz/... ./services/auth/... ./services/class/... ./services/study/... ./services/gateway/... ./services/payment/... ./services/quiz/...
npm --prefix apps/web test
npm --prefix apps/web run build
npx --yes @redocly/cli@2 lint packages/api-contracts/openapi.yaml
bash infra/scripts/phase10-e2e.sh
```

Windows cần Git Bash và có thể đặt `IAM_E2E_PYTHON` tới Python 3 thực tế. Script dùng cổng loopback 18080 và stack riêng; không kết nối DB production. Log local trong `tmp/iam-e2e-final.log`, `tmp/iam-go-tests.log`, `tmp/openapi-lint.log`, `tmp/dashboard-explain.log`. Workflow `dashboard-iam-gate.yml` chạy kiểm tra cho PR liên quan.

## Trước production

- Chạy backfill dry-run trên bản sao dữ liệu thật, kiểm tra phân bố role và migration; E2E tổng hợp không thay thế bước này.
- QA trực quan desktop/mobile và điều hướng trên trình duyệt. Môi trường hiện không có trình duyệt kết nối nên chưa thực hiện bước này.
- Làm theo [runbook IAM](deployment/iam-operations.md), gồm bootstrap admin, thu hồi, audit và cấu hình khóa ký.
- Repo có [workflow deploy](../.github/workflows/deploy.yml) chạy khi push `main`. Có thể commit/push nhánh làm việc để review; push `main` là bước kích hoạt pipeline triển khai.

Working tree còn các thay đổi/xóa tài liệu ngoài phạm vi IAM đã có từ trước. Chọn file khi commit để không gộp nhầm các thay đổi đó. Không có thao tác commit, push hoặc deploy tự động trong lượt làm việc này.
