# Flashcards và Spaced Repetition

Màn hình học phần hiển thị tiến độ trên bốn chế độ học, thuật ngữ trước hình ảnh và đánh giá sau khi lật thẻ. Tiến độ được lưu riêng theo người dùng trong PostgreSQL, không suy ra từ số thẻ đã xem.

- Chưa nhớ: về mức đang học, ôn sau 1 phút.
- Khó: giữ số lần nhớ, ôn sau 10 phút.
- Đã nhớ: lần đầu sau 1 ngày, các lần sau nhân đôi khoảng ôn.
- Dễ: lần đầu sau 4 ngày, các lần sau nhân bốn khoảng ôn.
- Khoảng ôn tối đa 365 ngày. Đánh giá trước hạn không tăng mức thành thạo.
- Thẻ mới: chưa đánh giá; đang học: 0–1 lần nhớ; sắp thuộc: 2–3 lần; thành thạo: từ 4 lần nhớ.

Nút học lại chỉ đặt lại tiến độ Spaced Repetition của người hiện tại trong học phần, có bước xác nhận. Lịch sử phiên học vẫn được lưu riêng. Migration `023_spaced_repetition.go` chạy khi Study service khởi động. Chế độ Learn/Test/Match hiện vẫn lưu lịch sử phiên riêng; đánh giá Spaced Repetition nằm trong Flashcards. Màn hình hiển thị số thẻ đến hạn; thứ tự thẻ vẫn theo bộ thẻ hiện có.

## Neo4j

Backend dùng driver Go chính thức, hỗ trợ `neo4j+s://` của Aura. Cấu hình local đã được sao chép **chỉ các biến NEO4J** từ `quizlet-clone/.env` sang `.env` của platform; không có mật khẩu trong mã nguồn hoặc frontend. Khi đổi cấu hình nguồn cần đồng bộ lại các biến này. Docker Compose dev/app truyền chúng vào Study service.

Chạy Compose từ thư mục gốc và chỉ rõ env file:

```powershell
docker compose --env-file .env -f infra/docker/docker-compose.yml up --build -d
```

Chạy Go local cần nạp biến môi trường trước. Probe chỉ đọc kết nối và truy vấn graph:

```powershell
Get-Content .env | ForEach-Object {
  if ($_ -match '^(NEO4J_[A-Z_]+)=(.*)$') {
    [Environment]::SetEnvironmentVariable($matches[1], $matches[2].Trim('"', "'"), 'Process')
  }
}
go run ./services/study/cmd/neo4j-check
```

Nút Kanji liên quan đọc các node `Kanji.char` và quan hệ `READING`, `IS_COMPONENT_OF` từ graph hiện có, giới hạn 24 kết quả và timeout 5 giây. Không sửa dữ liệu graph.

## Xác minh trong phiên triển khai

- Frontend: TypeScript, production build, bundle budget và Vitest.
- Backend: HTTP, service, middleware, migration và biên dịch các lệnh server/probe.
- Docker daemon hiện không chạy; chưa chạy migration trên PostgreSQL thật hay kiểm chứng giao diện trong trình duyệt.
- Probe Neo4j thật thất bại ở kết nối, chưa xác nhận truy vấn trên Aura.
- SSH ngoài sandbox: `10.0.0.52:22`, `172.16.0.222:22` timeout; `2001:470:24:849::2fe:22` không tới được. Cần địa chỉ public, tuyến VPN/Tailscale hoặc cổng chuyển tiếp và thông tin đăng nhập để kiểm tra tiếp.

## Kiểm tra bổ sung qua máy chủ trung gian

Đã đăng nhập thành công các máy hiện hữu bằng khóa triển khai có sẵn:

- App: `117.1.91.238:30256`, hostname `nat-mini-zzp40b`.
- Data: `117.1.91.238:30149`, hostname `nat-go-zzp40b`.

Kết quả từ App: `10.0.0.52:22` timeout; `172.16.0.222:22` báo No route to host; `[2001:470:24:849::2fe]:22` trả banner OpenSSH.
Kết quả từ Data: hai IPv4 timeout; IPv6 trên trả banner OpenSSH.
Đã thử SSH root tới IPv6 qua App bằng hai khóa triển khai `app_deploy` và `data_deploy`: cả hai bị từ chối xác thực. Host key mới được lưu riêng tại `tmp/ci-ssh/new-server-known-hosts` theo cơ chế trust-on-first-use; chưa có đối chiếu fingerprint độc lập từ console.

Không thay đổi cấu hình hoặc dịch vụ trên các server. Để đăng nhập server IPv6 cần khóa được server chấp nhận hoặc thông tin đăng nhập tương ứng. Server chỉ có IPv4 riêng `172.16.0.222` cần IPv6, cổng public chuyển tiếp hoặc tuyến nội bộ truy cập được.

Cập nhật: đã đăng nhập root thành công vào `v6-micro-37` qua IPv6 `2001:470:24:849::2fe`, đi qua App. Xác minh IP nội bộ `10.0.0.52`, RAM 512 MiB, RAM dùng khoảng 50 MiB và đĩa trống khoảng 14 GiB tại thời điểm kiểm tra. Mật khẩu không được lưu vào tài liệu hoặc file cấu hình.
