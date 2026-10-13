# Video HLS mã hóa AES-128

Video tải mới qua tab Video được FFmpeg chuyển thành H.264/AAC, các đoạn MPEG-TS khoảng 6 giây, mã hóa AES-128. Playlist và đoạn mã hóa nằm trong một ZIP không nén trên SFTP để tiếp tục dùng cơ chế xóa file/outbox hiện có. Backend đọc trực tiếp đoạn cần phát bằng seek, không tải toàn bộ video khi phát. Khóa giải mã không được đóng gói hoặc tải lên SFTP.

## Quyền xem

Trong mô hình hiện tại, khóa học là nhóm/lớp: người xem phải đang là thành viên và thuộc audience của video. Chủ nhóm/giáo viên có quyền quản lý và xem. Chưa có quy trình mua/đăng ký khóa học độc lập; không suy ra quyền video từ một thanh toán bất kỳ.

POST `/v1/classes/{classId}/videos/{id}/playback` yêu cầu bearer token, trả URL vé HMAC 5 phút. GET `/v1/class-video-hls?ticket=...&asset=...` yêu cầu cả bearer token hợp lệ qua gateway và vé thuộc đúng user. Playlist, key và từng đoạn video đều kiểm tra lại membership và audience trong database. Hết hạn trả 401; thu hồi quyền chặn yêu cầu tiếp theo. URL bị chia sẻ riêng không đủ để phát.

Player hls.js gửi bearer token trên mọi yêu cầu, xin vé mới sau 2 phút khi có yêu cầu tiếp theo; dừng nếu mất quyền. Không gửi token sang origin/path ngoài endpoint video. Playlist dùng đường dẫn tương đối để hoạt động sau proxy `/api`. Responses không cache; key dài đúng 16 byte. Không đưa bearer token, vé hoặc key vào log.

## Cấu hình và triển khai

Giữ cấu hình SFTP tại [private-video-sftp.md](private-video-sftp.md). Trên App, sinh khóa mới **một lần**:

```bash
openssl rand -hex 32
```

Điền kết quả vào `/opt/hquizlet/.env`, không commit hoặc gửi trong chat:

```dotenv
VIDEO_HLS_MASTER_KEY=<64 ký tự hex vừa sinh>
```

Khóa AES riêng từng video được dẫn xuất bằng HMAC-SHA256 với master key này và ID ngẫu nhiên của object. Sao lưu master key trong kho bí mật: mất hoặc đổi nó khiến video HLS cũ không giải mã được. Nó độc lập với `VIDEO_TICKET_SECRET`, vì vậy có thể thay secret ký vé mà không phá dữ liệu video.

Nếu master key chưa cấu hình, video cũ vẫn phát, upload mới trả 503. Giá trị không hợp lệ khi đã cấu hình làm startup thất bại để tránh mã hóa bằng khóa sai.

Build image `class` mới (Dockerfile cài FFmpeg), `gateway`, frontend và phát hành Worker assets nếu web đang phục vụ từ Worker. Compose mới chuyển tiếp `VIDEO_HLS_MASTER_KEY`. Route HLS phải tới gateway có xác thực, không trỏ thẳng vào SFTP hoặc đường stream cũ. Khi ghép Nginx production, thêm `class-video-hls` vào location streaming và giữ nguyên origin secret gate hiện có; không ghi đè Nginx riêng của server bằng bản mặc định.

Không cần migration mới: `content_type=application/vnd.apple.mpegurl` phân biệt bundle HLS với MP4/WebM cũ. Video cũ không tự chuyển mã; tải lại nếu muốn AES-128. Video YouTube không được chuyển mã bởi luồng này.

## Giới hạn và kiểm thử

Chuyển mã hiện đồng bộ trong yêu cầu upload, một tác vụ mỗi process, hai thread FFmpeg, thời gian tối đa 10 phút. Khi bận, upload khác thất bại để thử lại. Cần ổ tạm riêng tư trên App cho bản gốc, key, segments và bundle; dữ liệu tạm được dọn khi kết thúc. Đầu ra giới hạn 3 lần upload limit; kiểm tra dung lượng mỗi giây trong chuyển mã. Chỉ FFmpeg được phép đọc protocol file/crypto, không tải URL tùy ý.

Proxy phía trước có thể hết thời gian chờ trước khi chuyển mã xong. Bản này phù hợp video ngắn; video dài cần một hàng đợi chuyển mã nền trước khi dùng production ở quy mô lớn. Không tự tăng timeout Cloudflare bằng cấu hình Nginx. Khi upload bị ngắt, không tạo metadata và dọn object nếu đã ghi.

Trình duyệt cần MediaSource hỗ trợ hls.js. Trình duyệt chỉ có native HLS mà không hỗ trợ hls.js sẽ nhận thông báo; không hạ xuống link key không xác thực. MP4/WebM cũ vẫn dùng native video như trước.

Kiểm thử Go dùng FFmpeg thật tạo fixture, chuyển mã và giải mã AES-CBC để xác nhận MPEG-TS; kiểm tra khóa không nằm trong bundle, key khác nhau giữa video, playlist từ chối đường dẫn bên ngoài, URL bị sửa/hết hạn/sai user, quyền bị thu hồi và no-store. Kiểm thử tích hợp FFmpeg bỏ qua nếu máy không cài FFmpeg. Kiểm tra phát/tua thực tế qua Worker và SFTP production vẫn phải thực hiện sau deploy.

AES-128 HLS kiểm soát truy cập, không phải DRM: người đã có quyền xem vẫn nhận khóa và dữ liệu để phát, có thể lưu lại hoặc quay màn hình. `controlsList=nodownload` chỉ ẩn nút tải trong trình duyệt.
