# Không gian lớp học, video cộng đồng và Kanji graph

## Hành vi

- Menu **Video & bài đăng** mở `/videos`. Nội dung mới ở không gian cộng đồng mặc định công khai cho tài khoản HQuizlet. Người tạo có thể đổi thành riêng tư hoặc chọn mã người dùng được xem; quyền được kiểm tra ở API và khi lấy byte video.
- Bộ chọn không gian trong mục Video cho phép vào thư viện video lớp. Video đã có trong lớp vẫn giữ nguyên quyền; không tự công khai dữ liệu cũ khi đổi điều hướng.
- **Truyện** nằm trong lớp, mặc định chia sẻ cho thành viên lớp. Quyền `class-manga:Read/Create/Update/Delete` có trong phần quản lý vai trò. Hai vai trò dựng sẵn `manga-editor` và `manga-deleter` độc lập; giáo viên/chủ lớp/quản lý nội dung có quyền theo cấu hình quản lý sẵn.
- Chủ truyện cũ có thể chọn **Đưa truyện từ thư viện cũ vào lớp** nếu có quyền nhập truyện ở lớp đích. Không tự đoán lớp đích của dữ liệu cũ.
- Trình đọc mở toàn màn hình với **Vừa trang** mặc định, tự tính lại theo vùng hiển thị khi mở bảng OCR hoặc đổi kích thước. Có thể chọn 75–200% để phóng to.
- Danh sách phát có thể tạo, sửa tên/mô tả/quyền xem, thêm video bằng **Lưu vào**, gỡ video và xóa. Mỗi danh sách tối đa 500 video. Quyền danh sách không mở rộng quyền của từng video. Bộ chọn Lưu vào hiển thị tối đa 100 danh sách của người dùng.
- Bài đăng hỗ trợ tiêu đề và nội dung văn bản, chỉnh sửa, xóa, tìm kiếm và quyền xem. Đây chưa phải toàn bộ hệ thống kênh/bình luận/đăng ký của YouTube.

## Nguồn video

Hỗ trợ nhúng YouTube, URL đầy đủ `tiktok.com/@.../video/...`, Facebook `.../videos/...`, `/reel/...`, `watch/?v=...`, và phát URL HTTPS trực tiếp có đuôi MP4/WebM. Có thể tải file MP4/WebM lên kho video đang cấu hình. File tải lên cộng đồng phát dạng gốc có HTTP Range; video lớp vẫn dùng pipeline hiện có.

Link rút gọn TikTok/Facebook cần mở thành URL đầy đủ trước khi thêm. Nội dung công khai vẫn có thể bị nhà cung cấp tắt nhúng, xóa, chặn vùng hoặc yêu cầu đăng nhập. Nút mở nguồn gốc luôn có cho video liên kết; không tải lại hoặc vượt quyền của nguồn bên ngoài.

Tài liệu nhà cung cấp:
- https://developers.tiktok.com/doc/embed-player/
- https://developers.facebook.com/docs/plugins/embedded-video-player/

## Kanji graph không dùng Neo4j

Bản cũ lưu cạnh READING trong Neo4j và truy vấn một bước từ các chữ Hán của từ hiện tại. Chức năng này không cần một máy chủ graph riêng.

Triển khai hiện tại:

1. Bảng `kanji_word_index(char, flashcard_id, study_set_id)` có khóa chính `(char, flashcard_id)` và chỉ mục `(study_set_id, char, flashcard_id)`.
2. Trigger cùng transaction với flashcard cập nhật chỉ mục khi thêm, đổi từ hoặc đổi học phần. Khóa ngoại dọn cạnh khi xóa thẻ/học phần; không có hàng đợi có thể bỏ sót thao tác import.
3. Migration backfill một lần với marker, advisory lock và khóa ghi ngắn hạn trên flashcards để tránh dữ liệu cũ đua với cập nhật. Dữ liệu lớn nên chạy migration trước khi chuyển lưu lượng.
4. `GET /v1/study-sets/{id}/kanji-graph?term=...` kiểm tra quyền học học phần rồi lấy từ ở học phần hiện tại và các học phần do người dùng sở hữu. Không truy vấn học phần riêng của người khác hoặc học phần bị ẩn. Giới hạn 200 liên kết, có cờ `truncated`.
5. Giao diện cache kết quả 30 giây theo người dùng/học phần/từ. Khi API lỗi, chỉ mục Map trong bộ nhớ cung cấp các thẻ đã tải của học phần và hiện thông báo rõ. Chỉ mục cục bộ dựng một lần theo phiên bản danh sách thẻ, tra theo chữ, không quét mọi thẻ mỗi lần mở.
6. Bộ thủ/cây thành phần và cách viết tiếp tục dùng dữ liệu KanjiVG tĩnh theo shard, cache ở trình duyệt; không cần Neo4j cho các chức năng này.

Phạm vi graph hiện tại là liên kết chữ–từ, tương tự phần READING của bản cũ; chưa triển khai truy vấn đường đi nhiều bước xuyên toàn bộ thư viện có chia sẻ/phân quyền. Nếu mở rộng, có thể dùng bảng cạnh và recursive CTE/BFS giới hạn độ sâu. Chỉ nên cân nhắc Neo4j khi cần nhiều loại cạnh hoặc truy vấn đường đi phức tạp và benchmark thực tế cho thấy PostgreSQL không đạt mục tiêu.

Chưa có số đo latency SQL trên dữ liệu triển khai: Docker/PostgreSQL chưa chạy trong phiên kiểm tra. Sau khi bật cơ sở dữ liệu, cần chạy integration tests và `EXPLAIN (ANALYZE, BUFFERS)` trên truy vấn `/kanji-graph` với dữ liệu thực; không cam kết thời gian trả lời từ kiểm thử giả lập.

## Triển khai và kiểm tra

- Class service tự áp dụng migration `026_scoped_manga` và `027_public_media`; study service thêm migration `025_kanji_index`.
- Cần triển khai web, gateway, class service và study service cùng lượt. Dùng cấu hình kho video và khóa ký hiện có cho tải/phát video.
- Web: `npm run build --prefix apps/web`; `npm test --prefix apps/web`.
- Go: `go test ./services/class/... ./services/study/internal/http ./services/study/internal/migration ./services/gateway/...`.
- Các integration test PostgreSQL của repository cần `DATABASE_URL` và/hoặc biến test DB theo helper tương ứng. Bài test Kanji lifecycle xác nhận thêm/sửa/xóa thẻ đồng bộ chỉ mục, bao gồm chữ ngoài BMP.
