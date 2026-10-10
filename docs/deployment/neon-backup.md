# PostgreSQL chính → Neon backup

Giữ PostgreSQL trên server Data là database chính của ứng dụng. DATABASE_URL và các service App không đổi. Neon nhận bản sao định kỳ; không phải replica realtime và không tự chuyển ứng dụng sang Neon khi Data hỏng. R2 chỉ backup file MinIO.

## Thiết kế an toàn

Worker neon-backup chạy độc lập, dùng pg_dump từ nguồn local và pg_restore sang Neon. Image Alpine chỉ cài PostgreSQL client, không khởi động PostgreSQL server thứ hai.

Mỗi lần tạo database mới hquizlet_backup_<UTC timestamp>_<random> trên Neon. neondb trong URL chỉ dùng làm điểm kết nối quản trị; không restore đè nội dung của neondb. Restore chạy single transaction, no-owner/no-acl, kiểm tra truy vấn rồi ANALYZE; sau đó mới cập nhật latest_database trong volume state. Restore lỗi giữ bản thành công trước và dọn database mới nếu có thể.

Mặc định giữ 2 database snapshot thành công. Chỉ xóa bản cũ nằm trong history của worker, có prefix hợp lệ và ownership marker khớp; không xóa database có sẵn. Có thể tạm cần chỗ cho 3 bản (2 bản cũ + bản đang tạo). Không xóa volume neon-backup-state: nó chứa history và marker quản lý. Nếu đổi endpoint/role/database quản trị, dùng volume mới và quản lý bản cũ riêng.

Backup không ghi dữ liệu vào nguồn; worker giới hạn 0.5 CPU/256 MiB, restore một luồng và không nằm trong request người dùng. Full dump vẫn tạo tải đọc trên PostgreSQL và cần chỗ trống cho file dump nén. Hai worker R2 và Neon không phải snapshot nguyên tử cùng thời điểm. Dùng file key bất biến và giữ các phiên bản file phục hồi.

## Cấu hình .env trên server Data

```dotenv
NEON_BACKUP_ENABLED=true
NEON_BACKUP_URL="postgresql://neondb_owner:npg_Z5w6kjRzoDeK@ep-sweet-meadow-b5y18mr3-pooler.c-7.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require"
NEON_BACKUP_INTERVAL_SECONDS=86400
NEON_BACKUP_RETRY_SECONDS=1800
NEON_BACKUP_KEEP=2
NEON_BACKUP_TIMEOUT_SECONDS=7200
```

POSTGRES_URL cũng được nhận làm alias nếu NEON_BACKUP_URL trống; không dùng URL Neon thay cho DATABASE_URL của App. URL phải dùng direct endpoint (không có -pooler) và TLS. Không đưa password vào Git/log. .env local đã có cấu hình riêng; .env trên Data cần nạp riêng vì CI hiện chỉ triển khai App khi push.

Role Neon cần quyền CREATE DATABASE và sở hữu database mới. PostgreSQL nguồn hiện là 16, client 16; Neon đích phải cùng hoặc mới hơn và hỗ trợ extension/schema nguồn. Kiểm tra dung lượng gói Neon đủ cho số snapshot giữ lại. Backup mặc định mỗi 24 giờ, retry lỗi mỗi 30 phút; chưa có WAL/PITR cho nguồn local.

## Chuyển code và khởi động

Trên PowerShell Windows:

```powershell
cd D:\quizlet\hquizlet-platform
scp -P 30149 infra/docker/neon-backup.Dockerfile infra/docker/docker-compose.neon-backup.yml root@vn-hn.cloudcode.io.vn:/opt/hquizlet/infra/docker/
scp -P 30149 infra/scripts/neon-backup.py root@vn-hn.cloudcode.io.vn:/opt/hquizlet/infra/scripts/
```

Trên Data, mở nano /opt/hquizlet/.env thêm biến trên rồi chạy:

```bash
cd /opt/hquizlet
chmod 600 .env
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.neon-backup.yml build neon-backup
# Kiểm tra nguồn/Neon và quyền tạo DB; không tạo hay xóa database:
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.neon-backup.yml run --rm --no-deps neon-backup check
# Chỉ chạy sau khi check thành công:
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.neon-backup.yml up -d --no-deps neon-backup
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.neon-backup.yml logs --tail 50 neon-backup
```

Log **Neon database backup completed: hquizlet_backup_...** xác nhận bản sao được restore và cập nhật history. Healthcheck kiểm tra last_success; cần nối monitoring để nhận cảnh báo. Không tự tạo URL lỗi trong log; lỗi tool được che để bảo vệ password.

## Khôi phục

Trong Neon Console xem database hquizlet_backup_... mới nhất hoặc đọc latest_database từ state.json trong volume worker. Để kiểm tra, kết nối Neon bằng URL đã thay tên database sang bản snapshot, truy vấn các bảng và một vài bản ghi quan trọng.

Khi sự cố thực sự xảy ra: dump từ snapshot Neon bằng client tương thích phiên bản Neon, restore vào database local mới, kiểm tra bảng/file/quyền, rồi mới đổi DATABASE_URL của App theo quy trình riêng. Không tự ghi đè primary đang chạy. Password, ownership/ACL nguồn và cấu hình server không nằm trong snapshot; cần giữ cấu hình phục hồi riêng.

Các test trong repo xác minh: URL TLS/direct, snapshot mới, thất bại giữ bản cũ, retention không xóa database khác, cleanup dump và không lộ secrets. Đã kiểm tra Neon thật bằng truy vấn chỉ đọc: kết nối thành công và role có quyền CREATE DATABASE. Chưa thực hiện restore dữ liệu production lên Neon hoặc build container trên server trong phiên này.

Nguồn chính thức: [Neon Manage databases](https://neon.com/docs/manage/databases), [Neon migration bằng pg_dump/pg_restore](https://neon.com/blog/optimizing-dev-environments-in-aws-rds-with-neon-postgres-part-ii-using-github-actions-to-mirror-rds-in-neon).

## Khi Docker bridge không ra được Neon nhưng host kết nối được

Chỉ trên Linux Data server, thêm overlay `infra/docker/docker-compose.neon-backup.host.yml`. Worker dùng host network; PostgreSQL nguồn dùng `DATA_BIND_IP` và `POSTGRES_PORT` của server, tự bỏ dấu ngoặc IPv6. Có thể đặt `NEON_BACKUP_SOURCE_HOST` riêng nếu địa chỉ bind khác. Không đổi ports/firewall hay mạng của App. Script được mount read-only từ server nên không cần build lại image để cập nhật.

Chuyển overlay và script mới từ Windows:

```powershell
scp -P 30149 infra/docker/docker-compose.neon-backup.host.yml root@vn-hn.cloudcode.io.vn:/opt/hquizlet/infra/docker/
scp -P 30149 infra/scripts/neon-backup.py root@vn-hn.cloudcode.io.vn:/opt/hquizlet/infra/scripts/
```

Trên Data, dùng cùng ba file Compose cho mọi lần check/up/logs sau này:

```bash
cd /opt/hquizlet
neon() {
  docker compose --env-file .env \
    -f infra/docker/docker-compose.data.yml \
    -f infra/docker/docker-compose.neon-backup.yml \
    -f infra/docker/docker-compose.neon-backup.host.yml "$@"
}
if neon run --rm --no-deps neon-backup check; then
  neon up -d --no-deps neon-backup
  neon logs --tail 50 neon-backup
fi
```

Hàm `neon` chỉ là cách viết ngắn lệnh Compose trong terminal hiện tại. Thành công ở bước TCP chưa đủ: check phải xác nhận cả PostgreSQL nguồn, TLS/auth Neon và quyền CREATEDB trước khi chạy worker. Host overlay không được tự áp dụng cho stack local Windows hoặc CI App.
