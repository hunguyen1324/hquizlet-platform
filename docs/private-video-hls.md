# Video HLS mã hóa AES-128

Video tải mới qua tab Video được FFmpeg chuyển thành H.264/AAC, các đoạn MPEG-TS khoảng 6 giây, mã hóa AES-128. Playlist và đoạn mã hóa nằm trong một ZIP không nén trên SFTP để tiếp tục dùng cơ chế xóa file/outbox hiện có. Backend đọc trực tiếp đoạn cần phát bằng seek, không tải toàn bộ video khi phát. Khóa giải mã không được đóng gói hoặc tải lên SFTP.

FFprobe kiểm tra codec trước khi xử lý. H.264 8-bit yuv420p, profile Baseline/Main/High, level tối đa 4.2, kích thước chẵn và chiều rộng tối đa 1280, không có metadata xoay, được giữ nguyên bằng stream copy. AAC-LC mono/stereo cũng được giữ nguyên. Nếu chỉ âm thanh không tương thích, chỉ chuyển âm thanh sang AAC; video không tương thích, quá lớn, 10-bit hoặc cần xoay mới chuyển mã hình ảnh. Nếu probe lỗi thì dùng encoder; nếu remux lỗi thì xóa đầu ra dở và thử encoder một lần trong cùng ngân sách thời gian. Cả hai đường đều dùng cùng key info AES-128 và cùng kiểm tra quyền khi phát.

Stream copy giữ chất lượng hình ảnh gốc. Đoạn HLS cắt theo keyframe của nguồn nên có thể dài hơn mục tiêu 6 giây; không dùng split_by_time cắt giữa các keyframe. Encoder vẫn đặt keyframe khoảng 6 giây. Log ghi mode và thời gian FFmpeg, không ghi key hoặc vé phát. Benchmark local cho clip 1280x720/30fps dài 4 giây: encode khoảng 0,66 giây, copy khoảng 0,13 giây; đây chỉ là bước FFmpeg, không bao gồm upload/download SFTP và không phải cam kết tốc độ server.

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

Migration `008_async_hls.up.sql` thêm trạng thái xử lý, output key và cho phép content type HLS trong ràng buộc nguồn video. Migration chạy khi Class khởi động. `content_type=application/vnd.apple.mpegurl` phân biệt bundle HLS với MP4/WebM cũ. Video cũ không tự chuyển mã; tải lại nếu muốn AES-128. Video YouTube không được chuyển mã bởi luồng này.

## Giới hạn và kiểm thử

Upload lưu bản gốc trên SFTP và trả 202 với processingState=processing. Worker nền lấy job từ PostgreSQL mỗi 5 giây, dùng row lock SKIP LOCKED để tránh xử lý trùng; một tác vụ mỗi process, hai thread FFmpeg, thời gian tối đa 10 phút. Job tồn tại qua restart. Thành công sẽ cập nhật ready và đưa bản gốc vào hàng đợi xóa; lỗi chuyển mã thành failed để giáo viên xóa/tải lại. Trong lúc xử lý, API từ chối phát bản gốc. Cần ổ tạm riêng tư trên App cho bản gốc, key, segments và bundle; dữ liệu tạm được dọn khi kết thúc. Đầu ra giới hạn 3 lần upload limit; kiểm tra dung lượng mỗi giây trong chuyển mã. Chỉ FFmpeg được phép đọc protocol file/crypto, không tải URL tùy ý.

Chuyển mã nền không phụ thuộc request upload nên timeout proxy hoặc đóng trang sau khi nhận 202 không hủy job. Bước truyền bản gốc tới SFTP vẫn phụ thuộc giới hạn upload của proxy. Nếu upload bị ngắt trước khi lưu metadata, object được dọn nếu đã ghi. Giao diện thăm dò trạng thái mỗi 5 giây; video failed không phát được, cần xóa/tải lại. Không xóa video đang xử lý để tránh chờ row lock đến khi job hoàn tất.

Trình duyệt cần MediaSource hỗ trợ hls.js. Trình duyệt chỉ có native HLS mà không hỗ trợ hls.js sẽ nhận thông báo; không hạ xuống link key không xác thực. MP4/WebM cũ vẫn dùng native video như trước.

Kiểm thử Go dùng FFmpeg thật tạo fixture, chuyển mã và giải mã AES-CBC để xác nhận MPEG-TS; kiểm tra khóa không nằm trong bundle, key khác nhau giữa video, playlist từ chối đường dẫn bên ngoài, URL bị sửa/hết hạn/sai user, quyền bị thu hồi và no-store. Kiểm thử tích hợp FFmpeg bỏ qua nếu máy không cài FFmpeg. Kiểm tra phát/tua thực tế qua Worker và SFTP production vẫn phải thực hiện sau deploy.

AES-128 HLS kiểm soát truy cập, không phải DRM: người đã có quyền xem vẫn nhận khóa và dữ liệu để phát, có thể lưu lại hoặc quay màn hình. `controlsList=nodownload` chỉ ẩn nút tải trong trình duyệt.
