# API audio → SRT trên App và hai VPS Whisper

App chạy coordinator `http://127.0.0.1:8091`. Hai worker HTTPS dùng `base Q5_1` và OpenBLAS:

| Worker | Endpoint |
| --- | --- |
| d0 | `https://[2001:470:24:849::d0]:8090` |
| 2fe | `https://[2001:470:24:849::2fe]:8090` |

Mỗi file được giao cho một worker; hai worker xử lý tối đa hai file song song. File dài chưa được chia đoạn giữa các máy.

## Dùng ngay trên server App

```bash
stt-transcribe /duong-dan/audio.mp3 vi /duong-dan/ket-qua.srt
```

Lệnh upload file tới API trên App, in job ID, chờ xử lý và lưu file SRT. Ngôn ngữ nhận: `vi`, `ja`, `en`, `auto`. Khi chạy dưới root trên App, client tự đọc token từ `/etc/hquizlet-stt/service.env`. Với tài khoản khác, truyền token API qua biến môi trường `STT_API_TOKEN`.

Nếu ngắt lệnh giữa chừng, tác vụ vẫn tiếp tục trong hàng đợi. Tiếp tục tải kết quả bằng:

```bash
stt-transcribe --job JOB_ID
```

## Gọi API bằng curl

Nạp token trên App:

```bash
source /etc/hquizlet-stt/service.env
```

Gửi **nội dung nhị phân trực tiếp**, không phải multipart/form-data:

```bash
curl --fail-with-body -sS \
  -H "Authorization: Bearer $STT_TOKEN" \
  -H 'Content-Type: application/octet-stream' \
  --data-binary @/duong-dan/audio.mp3 \
  'http://127.0.0.1:8091/v1/transcriptions?language=vi'
```

Phản hồi `202 Accepted` có `id`, `status`, `language`, `createdAt`. Dùng ID nhận được:

```bash
curl --fail-with-body -sS -H "Authorization: Bearer $STT_TOKEN" \
  "http://127.0.0.1:8091/v1/transcriptions/$JOB_ID"

# Tải khi status = completed
curl --fail-with-body -sS -H "Authorization: Bearer $STT_TOKEN" \
  "http://127.0.0.1:8091/v1/transcriptions/$JOB_ID/srt" -o ket-qua.srt
```

Trạng thái: `queued` → `running` → `completed` hoặc `failed`. `worker` chỉ rõ máy thực hiện. API trả `409` khi lấy SRT chưa xong, `429` khi hàng đợi đầy, `401` khi token sai. Lỗi audio được ghi vào job với status `failed`.

## Giới hạn và lưu trữ

- File tối đa 64 MiB; audio tối đa 20 phút. Chỉ nhận file audio/media cục bộ được FFmpeg hỗ trợ trong nhóm WAV, MP3, OGG, FLAC, M4A/MP4, WebM/MKV, AAC. Không nhận URL/playlist.
- Tối đa 16 tác vụ chưa hoàn tất, gồm hai tác vụ đang chạy. Mỗi worker một tác vụ, một luồng CPU. Thời gian xử lý tối đa mỗi lần chạy 45 phút; file dài có thể vượt thời gian này trên CPU cũ.
- Tác vụ và file input lưu tại `/var/lib/hquizlet-stt` trên App. Input được xóa khi tác vụ hoàn tất/thất bại. Kết quả và metadata giữ 24 giờ sau cập nhật cuối, dọn mỗi giờ.
- Coordinator khởi động lại sẽ nạp lại hàng đợi; tác vụ đang chạy được xếp lại. Lỗi kết nối/bận worker được thử lại tối đa 6 lần. Chưa có idempotency key cho POST; gửi lại cùng file sẽ tạo job mới.
- Worker xóa file tạm sau mỗi yêu cầu. CLI thủ công và API chia sẻ khóa để tránh chạy nhiều model cùng lúc.

## Vận hành

Trên cả ba máy:

```bash
systemctl status hquizlet-stt
journalctl -u hquizlet-stt -n 50 --no-pager
systemctl restart hquizlet-stt
```

Unit chạy bằng user `hqstt`; worker giới hạn RAM 700 MiB, coordinator 180 MiB. Systemd tự khởi động dịch vụ khi reboot. Token và cấu hình tại `/etc/hquizlet-stt/service.env`, quyền root 0600. Hai worker có chứng chỉ riêng; App tin chứng chỉ từ `workers-ca.pem`, không bỏ kiểm chứng TLS. Khi cấp lại chứng chỉ worker, cần cập nhật CA bundle trên App rồi restart coordinator.

Coordinator giữ endpoint loopback trên **host App** và hiện có thêm listener riêng `172.17.0.1:8091` cho Docker. Study dùng hostname `stt-app-host` ánh xạ `host-gateway`; token chỉ nằm ở backend. UFW trên App đã cho phép riêng TCP từ bridge `br-2584f7eeccc8` / subnet `172.18.0.0/16` tới listener này. Đã đồng bộ token mới và xác minh container Study gọi được healthcheck qua hostname nội bộ. Mã giao diện và Study mới chưa được phát hành lên production. Xem `listening-questions.md` để tích hợp và kiểm thử.

Token API đã lưu thêm vào `.env` local của platform ở biến `STT_API_TOKEN`; file được Git bỏ qua. Token này khác token nội bộ của worker.

## Mã nguồn và build

Mã nguồn ở `services/stt`, Go stdlib, module độc lập:

```powershell
cd D:\quizlet\hquizlet-platform\services\stt
$env:GOWORK='off'
go test ./...
go vet ./...
$env:GOOS='linux'
$env:GOARCH='amd64'
$env:CGO_ENABLED='0'
go build -trimpath -o stt .
```

Unit mẫu ở `services/stt/deploy`. Lệnh client ở `services/stt/stt-transcribe.py`. Token, private key TLS và file cấu hình thật không nằm trong mã nguồn được commit.

## Kiểm chứng triển khai

Đã gửi đồng thời hai audio mẫu 11 giây từ App qua API: job `3ffe41f675bd0945f9f317f8dd8d0de0` chạy trên d0, job `e8ca979858fc87847926d79cf7fbc2d5` chạy trên 2fe. Cả hai hoàn tất và App tải được SRT có timestamp/nội dung đúng. Tệp mẫu kết quả trên App có dạng `/root/stt-api-test-JOB_ID.srt` (còn bản trong API theo thời hạn lưu trữ).

Go tests xác minh xác thực, ngôn ngữ, hàng đợi đầy, upload rỗng, khôi phục job sau restart, chuyển audio tới worker và tải subtitle. `go vet` đã qua. Worker và coordinator đều chạy dưới systemd; TLS được xác minh bằng CA bundle gồm chứng chỉ riêng của hai worker.

Đã kiểm chứng thêm lệnh client thật trên App: `stt-transcribe /root/stt-api-test.wav en /root/stt-client-test.srt` upload qua API, chờ job `f70b24e5ebdaa1ce64aee13f9d594866` hoàn tất và lưu SRT thành công. Worker gọi Whisper trực tiếp trong thư mục tác vụ, chia sẻ khóa flock với CLI thủ công; toàn bộ file tạm được dọn ngay cả khi timeout hoặc kết nối bị hủy.

## Kiểm chứng mạng Docker Study

Sau khi người dùng cho phép, đã thêm quy tắc UFW chỉ cho TCP từ `br-2584f7eeccc8` / `172.18.0.0/16` tới `172.17.0.1:8091`, đổi token API và đồng bộ coordinator, container Study, `.env` local. Token cũ và truy cập không xác thực trả 401. Đã gửi hai audio mẫu bằng client Go trong container Study: job `76efc063f37d03c58ba54cdf5599d1bf` chạy trên d0, job `65b993652a17ba78b2b8b3b09950521d` chạy trên 2fe. Cả hai trả SRT và tải được về từ container; Study healthy. Giao diện và backend Study mới chưa được phát hành lên production.
