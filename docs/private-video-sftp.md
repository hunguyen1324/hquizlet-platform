# Video riêng tư trên kho SFTP Cloudcode

Server app `nat-mini-zzp40b` kiểm tra quyền lớp và truyền video cho trình duyệt qua HTTPS hiện có của HQuizlet. Server data `nat-go-zzp40b` giữ metadata trong PostgreSQL. Cloudcode chỉ lưu file, không cần sửa `index.js`, cổng HTTP 24968 hoặc tạo subdomain.

## Cấu hình trên server app

Chức năng mặc định tắt khi `VIDEO_SFTP_HOST` rỗng. Sửa `/opt/hquizlet/.env`, giữ nguyên các biến đang có, thêm:

```dotenv
VIDEO_SFTP_HOST=us-sj.cloudcode.io.vn
VIDEO_SFTP_PORT=2022
VIDEO_SFTP_USER=nh2515901_p40b.fb256b5a
VIDEO_SFTP_ROOT=/private-videos
VIDEO_SFTP_HOST_FINGERPRINT=SHA256:e/KXFq4ifVSFtIAz+OpVKKzB6jpFVrgNn7VZNRe1tFY
VIDEO_SFTP_PASSWORD_FILE=/run/video-secrets/sftp-password
VIDEO_SFTP_MAX_CONNECTIONS=4
VIDEO_MAX_UPLOAD_MB=80
VIDEO_TICKET_SECRET=964450c559cfe2be9e91a6b61e48d4c180e680e170d5f09df31655226d23a857
```

Fingerprint trên lấy từ phiên SSH đã kết nối trong quá trình thiết lập; xác nhận với nhà cung cấp nếu có thể. Backend không chấp nhận host key khác và không dùng `InsecureIgnoreHostKey`.

Tạo file mật khẩu trên **server app**, không nhập vào Git hoặc chat:

```bash
cd /opt/hquizlet
install -d -m 700 infra/docker/video-secrets
read -r -s -p 'Cloudcode SFTP password: ' video_sftp_password
printf '\n'
(umask 077; printf '%s' "$video_sftp_password" > infra/docker/video-secrets/sftp-password)
unset video_sftp_password
```

Sinh `VIDEO_TICKET_SECRET` bằng `openssl rand -hex 32` và điền trực tiếp vào `.env`; giữ file `.env` riêng tư. Password file được mount read-only vào container `class`. Có thể thay password bằng `VIDEO_SFTP_PRIVATE_KEY_FILE` trỏ tới khóa trong thư mục secret nếu nhà cung cấp hỗ trợ SSH key; khi đó đặt `VIDEO_SFTP_PASSWORD_FILE=` để không đọc file mật khẩu.

Đường dẫn `/private-videos` là đường dẫn **SFTP chroot**, tương ứng `/home/container/private-videos` trong SSH Cloudcode. Thư mục phải tồn tại và cho phép tài khoản SFTP ghi file.

## Triển khai

Code này phải được dựng thành image mới cho `class`, `gateway` và frontend `nginx`. Workflow `.github/workflows/deploy.yml` hiện có hỗ trợ dựng và triển khai các image. Không bật cấu hình SFTP trên image cũ: image cũ chưa có API này.

Sau khi image mới đã được phát hành, cập nhật Compose và Nginx từ cùng phiên bản code. Chạy từ server app (thay `<tag-image-moi>` bằng tag thực tế, không dùng nguyên placeholder):

```bash
cd /opt/hquizlet
export IMAGE_TAG=<tag-image-moi>
docker compose --env-file .env -f infra/docker/docker-compose.app.yml pull class gateway nginx
docker compose --env-file .env -f infra/docker/docker-compose.app.yml up -d --no-deps class gateway nginx
docker compose --env-file .env -f infra/docker/docker-compose.app.yml logs --tail=50 class
```

Migration `007_private_videos.up.sql` được dịch vụ class áp dụng khi khởi động và có thể chạy lại. Nó bổ sung cột cho video SFTP, giữ nguyên video YouTube. Không cần đổi server data hoặc cấu hình dịch vụ `file` đang phục vụ ảnh/tài liệu.

Vì giao diện của bạn nằm ở `hquizlet.h2quiz.workers.dev`, cần phát hành lại assets Worker từ phiên bản mới trên máy phát triển đã đăng nhập Cloudflare:

```sh
npm --prefix infra/cloudflare run build
npm --prefix infra/cloudflare run deploy
```

Proxy Worker hiện có đã chuyển tiếp `/api/*`, body streaming và header Range; không cần thay `UPSTREAM_ORIGIN` đang hoạt động.

## API và cách sử dụng

Các endpoint dưới đây đi qua gateway hiện tại, thêm prefix `/api` khi dùng Nginx. Không gửi thông tin đăng nhập SFTP cho trình duyệt.

* `POST /v1/classes/{classId}/videos/upload?title=...&audience=all&viewerIds=[]`: bearer token, multipart với đúng một trường file tên `file`; chỉ chủ lớp/giáo viên. `audience=selected` dùng JSON array user ID trong `viewerIds`. Backend kiểm tra chữ ký định dạng MP4/WebM, tối đa 80 MiB mặc định, truyền thẳng tới SFTP, lưu metadata sau khi file hoàn tất. Đây không phải kiểm tra codec: nên dùng MP4 H.264/AAC với faststart hoặc WebM tương thích trình duyệt.
* `GET /v1/classes/{classId}/videos`: danh sách video có quyền xem, `source` là `sftp` hoặc `youtube`. Không trả storage key hay credential.
* `PUT /v1/classes/{classId}/videos/{id}`: chỉnh `title`, `audience`, `viewerIds`; video SFTP giữ nguyên file. `url` không cần cho video SFTP.
* `POST /v1/classes/{classId}/videos/{id}/playback`: bearer token, trả `{path, expiresAt}` chứa vé HMAC riêng cho người xem/video/lớp, hiệu lực 2 giờ. Ghép `path` với gateway base (`/api` trong production) rồi dùng trong `<video src=... controls>`.
* `GET/HEAD /v1/class-video-stream?ticket=...`: xác thực vé và kiểm tra lại quyền lớp/video từ database cho **mỗi yêu cầu**, kể cả Range khi tua. Trả 206/416 đúng chuẩn; không cache video; không buffer toàn bộ video vào RAM hay ổ đĩa app. Vé là link có thể chia sẻ trong thời hạn hiệu lực; nó không phải DRM hay phiên đăng nhập độc lập, nên người được xem vẫn có thể chia sẻ vé hoặc lưu video. Thu hồi quyền chặn yêu cầu tiếp theo, không ngắt luồng đang phát hoặc xóa dữ liệu đã tải.
* `DELETE /v1/classes/{classId}/videos/{id}`: chủ lớp/giáo viên xóa file SFTP và metadata; nếu SFTP lỗi thì trả lỗi để người dùng thử lại.

Trong tab video của lớp: chọn **Tải video riêng tư lên**, chọn MP4/WebM, đặt quyền, rồi thêm video. Video YouTube cũ vẫn phát như trước. Vé hết hạn thì đóng/mở video để cấp vé mới.

## Giới hạn vận hành

* SFTP mã hóa đường truyền qua cổng 2022; không cần cổng HTTP trên kho. App cần truy cập outbound tới host này.
* Giới hạn 4 kết nối SFTP đồng thời (tính cả tải lên/phát/xóa); yêu cầu sau chờ và hủy khi client ngắt kết nối. Nhiều học sinh xem đồng thời cần đánh giá băng thông và giới hạn kết nối Cloudcode trước khi tăng.
* Nginx dành riêng đường video cho streaming, upload body tối đa 90 MiB để chừa multipart overhead. Nếu tăng `VIDEO_MAX_UPLOAD_MB`, phải điều chỉnh Nginx và kiểm tra giới hạn request của mọi proxy phía trước HQuizlet. Upload lớn qua Worker/Cloudflare vẫn có thể bị giới hạn trước khi tới app; nên chia video dài thành các đoạn nhỏ ở cấu hình hiện tại.
* Hạn mức Cloudcode là 10 GiB theo thông báo tài khoản; `df` không phản ánh quota riêng. Backend không tự chuyển mã hoặc quản lý quota toàn server. Lỗi hết đĩa trả lỗi upload và không tạo video trong lớp.
* Upload dùng file `.part` và đổi tên khi hoàn tất. Nếu kết nối bị cắt/máy chủ khởi động lại trước cleanup, có thể còn `.part`. Xóa file `.part` cũ khi chắc chắn không có upload đang chạy.
* Xóa lớp sẽ cascade metadata và đưa file vào bảng `class_video_deletions`. Backend dọn hàng đợi mỗi phút, chỉ bỏ mục sau khi xóa SFTP thành công; kho mất kết nối thì giữ lại để thử tiếp. Nếu cấu hình SFTP tắt, hàng đợi chờ tới khi bật lại. Upload không lưu được metadata sẽ cố xóa file ngay; nếu cleanup đó cũng thất bại, đối chiếu file với `class_videos.storage_key` và hàng đợi trước khi dọn kho thủ công.
* Không có dữ liệu xác thực Cloudcode trong repository. Kiểm thử dùng SSH/SFTP giả lập; kết nối thật, tốc độ và phát video trên production cần kiểm chứng sau khi cấu hình secrets.
