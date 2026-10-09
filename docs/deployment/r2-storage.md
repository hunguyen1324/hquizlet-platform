# MinIO chính + R2 backup trên server Data

## Kiến trúc đã triển khai trong repository

- PostgreSQL lưu dữ liệu nghiệp vụ và metadata file.
- MinIO trên server Data nhận upload và phục vụ file chính.
- Worker độc lập chạy trên server Data, sao chép file sang R2 theo chu kỳ. R2 không nằm trong đường xử lý request của người dùng.
- Worker mã hóa nội dung, tên file và thư mục bằng rclone crypt trước khi gửi lên R2. R2_PUBLIC_URL không được sử dụng cho backup.
- PostgreSQL được pg_dump dạng custom, nén, kèm checksum SHA256. Không copy trực tiếp thư mục dữ liệu PostgreSQL đang chạy.

Worker quét MinIO qua S3 API, copy file mới/thay đổi và bỏ qua file không đổi. Không tạo thêm full copy file trên ổ server nhỏ; chỉ database dump cần dung lượng tạm. Mỗi file bị ghi đè giữ bản cũ tại versions/<run-id>/<bucket>. File bị xóa ở nguồn không bị xóa ở R2. Chưa tự động prune: backup hiện giữ cho đến khi quản trị viên chọn chính sách retention và kiểm tra phục hồi.

## Hiệu năng người dùng

- Đọc metadata từ PostgreSQL, đọc file từ MinIO/cache Nginx; không gọi R2 trong API upload, confirm, list hoặc get.
- Index danh sách file theo user_id + created_at + id, chỉ gồm file chưa xóa; index bao phủ size_bytes cho tổng dung lượng file active. Thứ tự phân trang có tie-breaker id.
- Pool File Service giới hạn 10 kết nối, giữ 5 idle; worker pg_dump có kết nối riêng.
- Cache lock Nginx giảm nhiều request cùng kéo một ảnh chưa có trong cache; background update phục vụ ảnh public trong lúc cập nhật.
- Worker mặc định 1 transfer, 2 checkers, tối đa 5 MiB/s và 10 request/s; container tối đa 0.5 CPU, 256 MiB RAM. Có thể giảm thêm theo tải thực tế.
- Đây là thay đổi giảm công việc trên request và cạnh tranh tài nguyên; chưa có benchmark tải production để cam kết một mức latency cụ thể. Danh sách hiện vẫn dùng OFFSET, phù hợp quota file hiện tại; khi mở quota rất lớn nên đổi sang cursor.

## Biến môi trường

Trên App: giữ STORAGE_PROVIDER=minio và cấu hình kết nối MinIO của server Data. Trên Data: ngoài POSTGRES_* và MINIO_ROOT_*, nạp:

```dotenv
BACKUP_PROVIDER=r2
BACKUP_ENABLED=true
BACKUP_PREFIX=hquizlet-backup
R2_ACCOUNT_ID=
R2_ENDPOINT=
R2_ACCESS_KEY_ID=
R2_SECRET_ACCESS_KEY=
R2_BUCKET_NAME=
BACKUP_ENCRYPTION_PASSWORD=
BACKUP_ENCRYPTION_SALT=
BACKUP_INTERVAL_SECONDS=900
BACKUP_DATABASE_INTERVAL_SECONDS=86400
BACKUP_BWLIMIT=5M
BACKUP_TPSLIMIT=10
BACKUP_SOURCE_BUCKETS=
```

R2_ENDPOINT tùy chọn, mặc định HTTPS endpoint theo R2_ACCOUNT_ID. BACKUP_SOURCE_BUCKETS trống mặc định chọn STORAGE_BUCKET (hquizlet) và MINIO_IMPORT_BUCKET (hquizlet-imports). imports/ là file xử lý tạm nên bị loại trừ; quiz-audio/ vẫn được backup. Nếu có bucket video riêng, thêm tên bucket vào danh sách phân cách bằng dấu phẩy, không có khoảng trắng.

Khóa mã hóa đã được tạo trong .env local nếu chưa có; không ghi đè khóa cũ. Cần lưu bản sao khóa ở nơi an toàn ngoài server Data để phục hồi khi server mất. Đổi/mất khóa khiến backup cũ không giải mã được. Bucket R2 nên private, dùng credentials chỉ cho bucket backup. Redis cache và NATS live state không được snapshot bởi worker này; dữ liệu nghiệp vụ PostgreSQL và file MinIO là phạm vi backup.

## Chuyển biến bằng file tạm

Tại máy dev, dùng SSH key đã được server chấp nhận:

```powershell
./infra/scripts/deploy-backup-env.ps1 -Server <SSH-host> -Port 30149 -User root
```

Script tạo file tạm chỉ chứa biến backup/R2, hạn chế quyền đọc trên Windows, chuyển qua SCP vào thư mục remote mode 0700, rồi dùng Python 3 gộp vào /opt/hquizlet/.env bằng atomic replace (mode 0600). Các biến PostgreSQL/MinIO đang có trên server được giữ nguyên. Script từ chối thay đổi khóa mã hóa đã được cấu hình khác.

File payload remote bị xóa sau khi nhập thành công; script dọn payload và installer remote cùng file tạm local khi kết thúc. Nếu mất kết nối làm cleanup remote không thành công, script báo đường dẫn cần dọn; không thể bảo đảm xóa từ xa khi SSH không truy cập được. Không xóa .env local chứa bản gốc. Không in secret và không commit file tạm vào Git.

## Khởi động trên server Data

Cần các file backup.Dockerfile, docker-compose.backup.yml và infra/scripts/backup-*.sh có mặt tại /opt/hquizlet. Không tự restart App hoặc database khi bật worker.

```bash
cd /opt/hquizlet
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.backup.yml build backup
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.backup.yml up -d --no-deps backup
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.backup.yml logs --tail 50 backup
```

Chạy chu kỳ đầu ngay lúc worker khởi động; sau mỗi chu kỳ chờ 900 giây. Thời gian mất dữ liệu file tối đa phụ thuộc chu kỳ + thời gian sao chép + lỗi/retry, không đảm bảo đúng 15 phút. Database tối thiểu một backup mỗi 24 giờ khi worker hoạt động; cần WAL/PITR nếu yêu cầu phục hồi sát thời điểm sự cố.

Docker healthcheck dùng last-success trong volume backup-state; quá 2 giờ chưa có chu kỳ thành công sẽ unhealthy sau grace ban đầu. Docker không tự gửi cảnh báo: cần đưa trạng thái này vào monitoring. Worker không tự restart các API khi R2 lỗi. Tránh chạy hai stack backup độc lập cho cùng prefix.

Local dùng docker-compose.yml thay cho docker-compose.data.yml với cùng overlay. Kiểm tra cấu hình mà không in secrets bằng config --quiet, không dùng config để xuất toàn bộ environment ra log.

## Kiểm tra và phục hồi

Kiểm tra kết nối không sửa dữ liệu khi worker đã dừng hoặc dùng volume state riêng:

```bash
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.backup.yml run --rm --no-deps -e BACKUP_STATE_DIR=/tmp/backup-check backup check
```

Các ví dụ sau chạy trong shell của image backup với environment đã nạp. Trước tiên cấu hình remote:

```bash
source /opt/backup/backup-common.sh
backup_configure
backup_rclone lsf vault:database
```

Chọn một <run-id> đã có cả .dump và .sha256; tải vào thư mục tạm riêng, không phục hồi đè production:

```bash
mkdir -p /tmp/restore-check
backup_rclone copyto vault:database/<run-id>.dump /tmp/restore-check/database.dump
backup_rclone copyto vault:database/<run-id>.sha256 /tmp/restore-check/database.sha256
cd /tmp/restore-check
sha256sum -c database.sha256
pg_restore --list database.dump
# Restore vào database thử nghiệm đã tạo riêng:
pg_restore --no-owner --no-acl --exit-on-error --dbname=hquizlet_restore_check database.dump
# Restore file vào bucket thử nghiệm; không ghi đè bucket chính:
backup_rclone copy vault:files/hquizlet primary:hquizlet-restore-check
# Kiểm tra integrity định kỳ (có thể tốn đọc file):
backup_rclone cryptcheck primary:hquizlet vault:files/hquizlet --one-way --exclude '/imports/**'
```

Định kỳ kiểm tra bảng, số dòng và phát một file khôi phục trong môi trường thử nghiệm. Backup file/database không phải snapshot giao dịch nguyên tử giữa hai hệ thống. Worker dump DB trước khi quét file; file chính nên dùng key bất biến và không xóa vật lý trước khi backup hoàn thành. Phiên bản cũ nằm tại vault:versions/<run-id>/<bucket>; chọn phiên bản phù hợp khi phục hồi dữ liệu cũ.

## Xác minh đã thực hiện

- Go tests cho toàn bộ File Service.
- Kiểm thử worker với lỗi dump, lỗi kiểm tra dump, lỗi sao chép/upload, retry và dọn file tạm.
- Kiểm thử importer giữ biến MinIO/PostgreSQL, xóa payload thành công và từ chối đổi khóa.
- Compose overlay cho cả local và Data được kiểm tra config --quiet.
- Rclone thật đã kiểm tra mã hóa, giữ phiên bản cũ và khôi phục sau khi xóa nguồn trên local.
- R2 thật đã kiểm tra upload/download file thử mã hóa; dữ liệu tải về khớp và prefix thử đã được xóa.
- Việc build container/chạy backup server thật còn cần Docker hoạt động và SSH server Data.

Nguồn: [rclone copy](https://rclone.org/commands/rclone_copy/), [rclone crypt](https://rclone.org/crypt/), [R2 S3 API](https://developers.cloudflare.com/r2/api/s3/api/).
