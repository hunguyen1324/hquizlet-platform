# Giai đoạn 4: tự đồng bộ URL tunnel

Python 3 standard library, Docker và systemd trên nat-mini. Không cần Node/Wrangler trên server. Script không restart tunnel: phát hiện phiên hiện tại, kiểm tra đủ bảy service bằng secret, cập nhật upstream và xác minh public health. Lỗi sẽ được thử lại ở lần timer tiếp theo.

Script dùng API riêng cho một secret `UPSTREAM_ORIGIN`, không tải lại Worker code, assets hoặc thay toàn bộ bindings. Trước khi bật, đổi runtime `UPSTREAM_ORIGIN` từ Text sang **Secret** với cùng URL hiện tại trên dashboard (xóa biến Text và thêm Secret, deploy). Giữ `ORIGIN_PROXY_SECRET` là Secret. Không gửi token/secret vào chat hoặc commit.

API: https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/secrets/methods/update/

## 1. Tạo API token

Cloudflare → My Profile → API Tokens → Create Token → Custom token:

- Account permission: **Workers Scripts: Edit**.
- Account resources: chỉ tài khoản chứa `hquizlet`.
- Chọn hạn sử dụng phù hợp và ghi lịch gia hạn.

Quyền này có phạm vi account, không giới hạn riêng một Worker. Script chỉ gọi Worker đã cấu hình. Copy Account ID đầy đủ từ dashboard; không dùng token ID thay API token.

## 2. Lấy code về server

Nếu đã chỉnh Nginx trực tiếp trên server, sao lưu file đó trước. Không dùng git reset để lấy code vì có thể mất secret/cấu hình.

```bash
cd /opt/hquizlet
cp infra/nginx/nginx.conf /root/nginx-before-phase4.conf
chmod 600 /root/nginx-before-phase4.conf
git pull --ff-only
python3 --version
install -d -m 700 /etc/hquizlet /var/lib/hquizlet-tunnel-sync
install -m 600 infra/cloudflare/tunnel-sync/config.example.json /etc/hquizlet/tunnel-sync.json
nano /etc/hquizlet/tunnel-sync.json
```

Điền account ID, token và origin secret thật; initial_origin là URL tunnel đang dùng để có thể rollback lần cập nhật đầu. Các giá trị URL không kèm path. File secret đặt ngoài repo, quyền 600.

## 3. Xác minh và bật timer

```bash
cd /opt/hquizlet
python3 infra/cloudflare/tunnel-sync/sync.py --dry-run
python3 infra/cloudflare/tunnel-sync/sync.py
install -m 644 infra/cloudflare/tunnel-sync/hquizlet-tunnel-sync.service /etc/systemd/system/
install -m 644 infra/cloudflare/tunnel-sync/hquizlet-tunnel-sync.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now hquizlet-tunnel-sync.timer
systemctl list-timers hquizlet-tunnel-sync.timer
journalctl -u hquizlet-tunnel-sync.service -n 30 --no-pager
```

Chỉ bật timer sau khi chạy thủ công thành công. Dry run không ghi Cloudflare nhưng vẫn kiểm tra tunnel thật. Public health được kiểm tra mỗi phút; mỗi lần cập nhật chỉ thực hiện khi URL đổi hoặc public health lỗi.

## 4. Nghiệm thu và rollback

- Trên thời gian bảo trì, restart tunnel; chờ log có URL mới và Registered tunnel connection.
- Trong khoảng 1–2 phút sau khi URL mới hoạt động, kiểm tra health qua Worker và trạng thái timer. Có thể gián đoạn trong thời gian tạo/xác minh URL.
- Frontend Static Assets vẫn tải khi backend mất kết nối.
- Script không ghi trạng thái thành công nếu public health thất bại; thử khôi phục URL trước khi có thể.
- Không chạy đồng thời deploy Worker và sync khi thử nghiệm. API cập nhật secret tạo thay đổi deployment; cần nghiệm thu preservation của ASSETS và secret trong tài khoản thật.

```bash
systemctl disable --now hquizlet-tunnel-sync.timer
systemctl stop hquizlet-tunnel-sync.service
```

Sau khi dừng, chỉnh UPSTREAM_ORIGIN thủ công trên dashboard nếu cần. Không xóa file state trong lúc service chạy. Token hết hạn hoặc thiếu quyền: sửa config, chạy thủ công và xem log trước khi bật lại.
