# Hướng dẫn triển khai HQuizlet Platform lên 2 server NAT

Kiến trúc: **1 server App** (Nginx + 6 service Go + Web) và **1 server Data**
(Postgres + Redis + NATS + MinIO), nối với nhau qua mạng riêng (khuyến nghị
Tailscale) để DB/cache không lộ ra internet qua NAT port-forward.

Gợi ý phân vai trò dựa trên cấu hình 2 server bạn đang có:
- **Data server** → server có ổ đĩa lớn hơn (24GB, cổng SSH 30149) — vì
  Postgres + MinIO sẽ tăng dung lượng theo thời gian.
- **App server** → server còn lại (14GB, cổng SSH 30256) — service Go chạy
  stateless, không cần nhiều đĩa.

---

## 0. Việc cần làm ngay: thu hồi token GitHub đã dán trong chat

Vào GitHub → Settings → Developer settings → Personal access tokens →
Revoke token cũ, tạo token mới. Không dùng lại token cũ cho bất cứ việc gì.

---

## 1. Kết nối giữa 2 server: dùng thẳng IPv6 (đã test ping OK)

2 server đã xác nhận ping IPv6 thông cả 2 chiều, ~0% packet loss:

- Server App (`nat-mini-zzp40b`): `2001:470:24:849::29f`
- Server Data (`nat-go-zzp40b`):  `2001:470:24:849::2d2`

Vì vậy **không cần cài Tailscale** — dùng thẳng 2 địa chỉ IPv6 này để kết nối.

> ⚠️ Dải `2001:470::/32` là dải IPv6 công khai (Hurricane Electric), tức là
> về nguyên tắc có thể route được từ internet, không phải mạng riêng như
> `10.x.x.x`. Bắt buộc phải bật firewall (bước 2) để chỉ đúng server kia mới
> gọi được vào các port DB/cache/broker — nếu không, DB có thể lộ ra ngoài.
> Nếu muốn chắc chắn 100% an toàn thay vì tin vào firewall, vẫn có thể cài
> thêm Tailscale sau này để mã hoá + giới hạn truy cập ở tầng mạng, nhưng với
> ufw cấu hình đúng như bước 2 là đủ dùng cho giai đoạn này.

Do dùng địa chỉ IPv6, khi điền vào file cấu hình (`docker-compose.yml`,
`.env`) luôn phải **đặt trong dấu ngoặc vuông** `[...]`, ví dụ
`[2001:470:24:849::2d2]:5432` — đây là cú pháp chuẩn cho IPv6 trong URL/host:port.

---

## 2. Chuẩn bị thư mục & firewall trên từng server

Trên **cả 2 server**:

```bash
mkdir -p /opt/hquizlet/infra/docker /opt/hquizlet/infra/nginx
cd /opt/hquizlet && git clone --depth 1 https://github.com/hunguyen1324/hquizlet-platform.git tmp \
  && cp -r tmp/infra ./ && rm -rf tmp   # lấy sẵn thư mục infra, sau này CI sẽ tự cập nhật

# Cài Docker + Compose plugin nếu chưa có
curl -fsSL https://get.docker.com | sh
```

Firewall (dùng `ufw`) — chỉ mở port DB/cache/broker cho đúng IPv6 của server
kia, không mở cho cả dải mạng hay 0.0.0.0.

**Trên server Data** (chỉ cho phép server App gọi vào):

```bash
sudo ufw default deny incoming
sudo ufw allow 22/tcp                                            # giữ SSH
sudo ufw allow from 2001:470:24:849::29f to any port 5432 proto tcp   # Postgres
sudo ufw allow from 2001:470:24:849::29f to any port 6379 proto tcp   # Redis
sudo ufw allow from 2001:470:24:849::29f to any port 4222 proto tcp   # NATS
sudo ufw allow from 2001:470:24:849::29f to any port 8222 proto tcp   # NATS monitor
sudo ufw allow from 2001:470:24:849::29f to any port 9000 proto tcp   # MinIO API
sudo ufw allow from 2001:470:24:849::29f to any port 9001 proto tcp   # MinIO console (có thể bỏ nếu không cần truy cập ngoài)
sudo ufw enable
```

**Trên server App:**

```bash
sudo ufw default deny incoming
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

Kiểm tra lại bằng `sudo ufw status verbose` sau khi bật, đảm bảo đúng IP.

> Lưu ý: nhà cung cấp NAT của bạn cần forward port 80/443 công khai vào
> đúng server App thì người dùng ngoài internet mới truy cập được web/app.
> Việc này cấu hình ở phía panel NAT (cloudcode.io.vn), không phải trên OS.

---

## 3. File `.env` trên từng server (KHÔNG commit vào git)

### Server Data — `/opt/hquizlet/.env`

```env
POSTGRES_DB=hquizlet
POSTGRES_USER=hquizlet
POSTGRES_PASSWORD=<mật khẩu mạnh>
REDIS_PASSWORD=<mật khẩu mạnh>
MINIO_ROOT_USER=<user riêng, không dùng minioadmin>
MINIO_ROOT_PASSWORD=<mật khẩu mạnh>
STORAGE_BUCKET=hquizlet
DATA_BIND_IP=[2001:470:24:849::2d2]
```

### Server App — `/opt/hquizlet/.env`

```env
DATA_HOST=[2001:470:24:849::2d2]
POSTGRES_USER=hquizlet
POSTGRES_PASSWORD=<giống bên Data>
POSTGRES_DB=hquizlet
REDIS_PASSWORD=<giống bên Data>
MINIO_ROOT_USER=<giống bên Data>
MINIO_ROOT_PASSWORD=<giống bên Data>
STORAGE_BUCKET=hquizlet
STORAGE_PUBLIC_BASE_URL=http://<domain hoặc IP public>/files
JWT_SECRET=<chuỗi ngẫu nhiên dài>
CLASS_INTERNAL_TOKEN=<chuỗi ngẫu nhiên>
ADMIN_TOKEN=<chuỗi ngẫu nhiên>
SEPAY_API_TOKEN=
SEPAY_BIDV_BANK_ACCOUNT_ID=
SEPAY_WEBHOOK_API_KEY=
```

---

## 4. Cài GitHub Actions self-hosted runner trên máy dev

1. Vào repo trên GitHub → **Settings → Actions → Runners → New self-hosted runner**,
   chọn Windows, làm theo hướng dẫn tải & cấu hình (`config.cmd`).
2. Cài đặt runner như **Windows Service** để nó tự chạy nền kể cả khi bạn
   không đăng nhập (`./svc.cmd install` rồi `./svc.cmd start`).
3. Đảm bảo máy dev có sẵn: **Docker Desktop**, **Go**, **Node.js/pnpm**,
   **Rust/cargo**, **OpenSSH client** (đã có sẵn theo log của bạn) — để chạy
   được cả bước test lẫn build/push image.

---

## 5. Thêm Secrets vào GitHub repo

Vào **Settings → Secrets and variables → Actions**, thêm:

| Secret | Giá trị |
|---|---|
| `APP_SERVER_HOST` | domain hoặc IP Tailscale server App |
| `APP_SERVER_PORT` | 30256 (hoặc port SSH hiện tại của server App) |
| `APP_SERVER_USER` | `root` (khuyến nghị đổi sang user riêng, không dùng root) |
| `APP_SERVER_SSH_KEY` | private key SSH (dạng OpenSSH, khớp với public key đã add vào `authorized_keys` của server App) |
| `DATA_SERVER_HOST` | domain hoặc IP Tailscale server Data |
| `DATA_SERVER_PORT` | 30149 |
| `DATA_SERVER_USER` | `root` (nên đổi) |
| `DATA_SERVER_SSH_KEY` | private key SSH cho server Data |

Tạo cặp khóa riêng cho CI (không dùng key cá nhân của bạn):

```powershell
ssh-keygen -t ed25519 -f ci_deploy_key -N '""'
# copy nội dung ci_deploy_key.pub vào ~/.ssh/authorized_keys của từng server
# copy nội dung ci_deploy_key (private) vào secret tương ứng
```

---

## 6. Deploy lần đầu (thủ công, trước khi để CI tự động)

**Server Data:**

```bash
cd /opt/hquizlet
docker compose -f infra/docker/docker-compose.data.yml --env-file .env up -d
```

**Server App:**

```bash
cd /opt/hquizlet
echo "IMAGE_TAG=latest" > .image_tag.env
docker compose -f infra/docker/docker-compose.app.yml --env-file .env --env-file .image_tag.env pull
docker compose -f infra/docker/docker-compose.app.yml --env-file .env --env-file .image_tag.env up -d
```

Kiểm tra:

```bash
curl http://<IP-hoặc-domain-server-App>/api/healthz
```

---

## 7. Từ giờ về sau: chỉ cần `git push` lên `main`

Push code → runner trên máy dev tự chạy: test (`make ci-gate`) → build & push
image lên GHCR → SSH deploy lên server App. Data layer **không** tự deploy
lại mỗi lần push (ít đổi, rủi ro cao hơn) — muốn cập nhật thì vào tab
**Actions → Run workflow → tick "Deploy luôn cả data layer"**.

---

## 8. Backup MinIO + PostgreSQL sang R2

Dùng worker riêng trên server Data, không chạy backup trong API App. Xem [hướng dẫn R2 backup](r2-storage.md) để nạp credentials qua file tạm, khởi động overlay Docker, kiểm tra và phục hồi.

Worker mã hóa backup trước khi gửi lên R2; file chính vẫn phục vụ từ MinIO. Giữ khóa mã hóa ngoài server để phục hồi khi server Data mất. Theo dõi healthcheck backup và thời gian lần thành công gần nhất bằng công cụ monitoring hiện có. Không chạy thêm cron dump cũ nếu không có nhu cầu giữ một bản backup local riêng.
