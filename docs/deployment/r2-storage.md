# MinIO → R2: backup file

PostgreSQL chính vẫn chạy trên server Data. R2 chỉ backup object của MinIO; backup database dùng Neon, xem [neon-backup.md](neon-backup.md).

## Cách hoạt động

Worker backup độc lập sao chép file mới/thay đổi từ MinIO sang R2, mã hóa nội dung và tên file bằng rclone crypt. Không gọi R2 trong API upload/read/list. Không có pg_dump, pg_restore, credentials PostgreSQL hoặc image PostgreSQL trong worker R2.

Dùng copy, không sync: file bị xóa ở MinIO vẫn giữ trên R2. File bị ghi đè được lưu bản cũ tại versions/<run-id>/<bucket>. Chưa tự động xóa backup R2; không đặt lifecycle xóa toàn bộ prefix vì có thể mất bản duy nhất của file không đổi.

Mặc định chạy mỗi 900 giây sau chu kỳ, một transfer, hai checkers, tối đa 5 MiB/s; container giới hạn 0.5 CPU và 256 MiB RAM. File truyền trực tiếp giữa hai kho, không tạo full copy local. Quota/index/cache phục vụ người dùng không phụ thuộc worker.

## Giới hạn dung lượng R2

Worker mặc định chặn upload khi tổng bucket R2 đạt **9.500.000.000 byte (9.5 GB thập phân)**. `BACKUP_MAX_STORAGE_BYTES` có thể giảm nhưng không tăng quá ngưỡng này. Đo toàn bộ bucket qua remote R2 thô, gồm dữ liệu mã hóa, versions và prefix khác. Đầu mỗi chu kỳ đo bucket; trước mỗi file, dự trù và cộng dồn kích thước mã hóa; nếu file không vừa, dừng chu kỳ mà không xóa backup cũ. Nếu không đo được dung lượng, cũng dừng upload. Worker tự thử lại theo chu kỳ và không cập nhật last-success khi bị chặn; healthcheck sẽ báo unhealthy khi backup quá cũ.

Đây là giới hạn của worker, không phải quota Cloudflare hay giới hạn hóa đơn. Các bucket khác trong tài khoản, phí API, upload từ chương trình khác và multipart chưa hoàn tất không nằm trong bảo đảm này. Chỉ chạy một worker ghi vào bucket này. Phép đo mỗi chu kỳ và kiểm tra từng object vẫn phát sinh API; nên giảm tần suất backup nếu có nhiều object. Worker dự trù cả file không đổi nên có thể dừng sớm khi gần ngưỡng. Không sửa object nguồn trong lúc đang truyền; giới hạn kích thước và transfer chặn object lớn lên trước khi bắt đầu truyền, nhưng không tạo snapshot nguồn.

Cần chuyển thêm `infra/scripts/backup-quota.py`, build lại image (có Python 3) và tạo lại container để áp dụng. Chỉ thêm biến vào `.env` với image cũ chưa có hiệu lực.

## Biến trên server Data

```dotenv
BACKUP_ENABLED=true
BACKUP_PROVIDER=r2
BACKUP_PREFIX=hquizlet-backup
R2_ACCOUNT_ID="<account-id>"
R2_ACCESS_KEY_ID="<access-key-id>"
R2_SECRET_ACCESS_KEY="<secret-access-key>"
R2_BUCKET_NAME="<backup-bucket>"
BACKUP_ENCRYPTION_PASSWORD="<giu-nguyen-khoa-ma-hoa-cu>"
BACKUP_ENCRYPTION_SALT="<giu-nguyen-salt-cu>"

BACKUP_MAX_STORAGE_BYTES=9500000000
BACKUP_INTERVAL_SECONDS=900
BACKUP_BWLIMIT=5M
BACKUP_TPSLIMIT=10
BACKUP_SOURCE_BUCKETS=
```

Giữ khóa mã hóa cũ; lưu bản sao offline. R2_PUBLIC_URL không dùng cho backup. BACKUP_SOURCE_BUCKETS trống mặc định backup hquizlet và hquizlet-imports; bỏ qua imports/ tạm, vẫn giữ quiz-audio/. MINIO_ROOT_USER/PASSWORD đã có trên Data được dùng để đọc nguồn.

## Khi log báo `Backup disabled`

Worker chỉ chạy khi `BACKUP_ENABLED` có giá trị chính xác là `true`. Lệnh `check` cũng thoát trước khi kiểm tra kết nối nếu backup chưa bật; `Backup disabled` không phải kết quả kiểm tra thành công.

Trên server Data, sửa `/opt/hquizlet/.env` (không phải `.env` trên máy Windows):

```bash
cd /opt/hquizlet
nano .env
```

Đặt `BACKUP_ENABLED=true`, `BACKUP_PROVIDER=r2` và kiểm tra các biến R2, khóa mã hóa ở trên đã có giá trị thật. Giữ nguyên khóa mã hóa và `BACKUP_PREFIX` của backup cũ. Mỗi biến chỉ có một dòng khai báo. Nếu shell đã export `BACKUP_ENABLED` hoặc `BACKUP_PROVIDER`, bỏ override trước khi chạy Compose:

```bash
unset BACKUP_ENABLED BACKUP_PROVIDER
```

Sau khi lưu `.env`, chạy lại lệnh `up` bên dưới để Compose tạo lại container với environment mới. `restart` không nạp lại `.env`. Chỉ sửa `.env` thì không cần build lại image.

Cảnh báo orphan `docker-neon-backup-1` xuất hiện vì các file Compose đang dùng không khai báo service Neon. Không thêm `--remove-orphans` khi worker Neon vẫn cần chạy.

## Chuyển code mới lên Data

Chạy trên PowerShell máy Windows:

```powershell
cd D:\quizlet\hquizlet-platform
scp -P 30149 infra/docker/backup.Dockerfile infra/docker/docker-compose.backup.yml root@vn-hn.cloudcode.io.vn:/opt/hquizlet/infra/docker/
scp -P 30149 infra/scripts/backup-common.sh infra/scripts/backup-data.sh infra/scripts/backup-quota.py root@vn-hn.cloudcode.io.vn:/opt/hquizlet/infra/scripts/
```

Worker cũ trên server vẫn dump PostgreSQL cho đến khi được thay bằng phiên bản này. Sau khi chuyển file, trên Data chạy:

```bash
cd /opt/hquizlet
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.backup.yml stop backup
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.backup.yml build backup
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.backup.yml up -d --no-deps backup
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.backup.yml logs --tail 50 backup
```

Log thành công mới: **Encrypted MinIO backup cycle completed**. Image mới dựa trên rclone, không cần tải image PostgreSQL. Các dump database cũ trên R2 không bị tự xóa.

## Kiểm tra và phục hồi

Kiểm tra kết nối không sửa dữ liệu:

```bash
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.backup.yml run --rm --no-deps -e BACKUP_STATE_DIR=/tmp/backup-check backup check
```

Mở Bash **trong container backup** bằng lệnh sau trên server Data. `/opt/backup` chỉ nằm trong image, không nằm trên host; `--entrypoint` bỏ qua entrypoint worker để mở shell và Compose vẫn nạp environment:

```bash
docker compose --env-file .env -f infra/docker/docker-compose.data.yml -f infra/docker/docker-compose.backup.yml run --rm --no-deps --entrypoint /bin/bash backup
```

Sau khi thấy prompt trong container, chạy:

```bash
source /opt/backup/backup-common.sh
backup_configure
# Khôi phục vào bucket thử nghiệm đã tạo riêng, không ghi đè kho chính:
backup_rclone copy vault:files/hquizlet primary:hquizlet-restore-check
# Đối chiếu định kỳ; có thể tốn đọc file:
backup_rclone cryptcheck primary:hquizlet vault:files/hquizlet --one-way --exclude '/imports/**'
```

Dùng `exit` để thoát shell container. `check` thành công phải in **MinIO and R2 connectivity OK**; worker chạy thành công phải in **Encrypted MinIO backup cycle completed**.

Healthcheck dùng last-success; mặc định cảnh báo unhealthy sau 2 giờ không có chu kỳ thành công (sau grace). Cần monitoring riêng để nhận cảnh báo. Thời gian dữ liệu chưa được backup phụ thuộc chu kỳ, thời gian truyền và lỗi/retry; đây không phải đồng bộ real-time.

Script deploy-backup-env.ps1 tạo payload tạm, chuyển bằng SCP, gộp vào .env bằng install-backup-env.py rồi dọn payload. Script yêu cầu SSH key; nếu SSH bằng mật khẩu, sửa .env trên server qua nano và chuyển code bằng SCP như trên. .env local không được push bởi CI. Khi mất kết nối SSH, kiểm tra cảnh báo cleanup remote và dọn thư mục tạm được chỉ rõ.

Tham khảo: [rclone copy](https://rclone.org/commands/rclone_copy/), [rclone crypt](https://rclone.org/crypt/).
