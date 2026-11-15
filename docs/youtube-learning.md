# Học video YouTube theo phụ đề

Trong thư viện video của lớp, bấm video YouTube để mở màn hình học. Trình phát chính thức nằm bên trái, danh sách phụ đề bên phải (xuống dưới trên điện thoại).

- Phụ đề đồng bộ theo thời gian; bấm câu để tua và nghe, lặp câu, câu trước/sau, đổi tốc độ, tìm kiếm, theo dõi tự động, đổi cỡ chữ và tải SRT.
- Shadowing: nghe và nói cùng câu gốc, hiển thị bản dịch nếu có.
- Phát âm: thu bằng microphone và nghe lại để tự so sánh. Bản thu không gửi lên server, tự dừng sau 60 giây; cần HTTPS/localhost và quyền microphone. Chưa có chấm điểm phát âm hoặc furigana.
- Luyện nghe: ẩn câu gốc/bản dịch, nhập nội dung nghe được rồi đối chiếu phụ đề, bỏ qua dấu câu và khoảng trắng.
- Bài tập: ẩn một từ trong câu thực tế và kiểm tra đáp án. Không tạo nội dung hay bản dịch giả.

## Nguồn phụ đề

Server thử đọc các track công khai mà YouTube trả về và bản dịch tiếng Việt nếu có. Đây là best-effort: một video có CC trong YouTube không đảm bảo server đọc được track (giới hạn vùng, nhà cung cấp, quyền hoặc thay đổi định dạng). Không vượt đăng nhập/challenge. YouTube Data API captions.download yêu cầu quyền chỉnh sửa video: https://developers.google.com/youtube/data/reference/rest/v3/captions/download

Khi không lấy được, video vẫn phát và giao diện hướng dẫn nhập SRT/VTT. Nhập file nguyên ngữ và bản dịch riêng với mốc thời gian tương ứng. Học viên dùng file trong phiên học; người có class-video:Update có thể lưu/chỉnh/xóa phụ đề dùng chung cho lớp. File tối đa 1 MiB, 10.000 câu, mỗi câu 3.000 ký tự, mốc thời gian tối đa 12 giờ.

## API và triển khai

Áp dụng migration `023_video_subtitles.up.sql`, triển khai lại class service và web. Các endpoint GET/PUT/DELETE `/v1/classes/{classId}/videos/{videoId}/subtitles` đều kiểm tra lớp, quyền ViewVideo và danh sách người xem hiện tại; PUT/DELETE cần thêm Update. Nội dung chia sẻ lưu PostgreSQL; xóa video sẽ cascade phụ đề. Không cần YouTube API key cho nguồn công khai best-effort. Không cache HTTP công khai; cache nguồn trên server có giới hạn và thời hạn ngắn.

Trình phát dùng YouTube IFrame API, giữ điều khiển chính thức; video bị cấm nhúng có thông báo và liên kết mở YouTube. Tham khảo https://developers.google.com/youtube/iframe_api_reference

## Kiểm tra

Frontend unit tests và production build; Go class-service tests, bộ kiểm tra public-caption parser/URL an toàn và phân quyền subtitle; PostgreSQL integration chạy migration hai lần rồi lưu/đọc/xóa phụ đề và thay đổi audience. Chrome headless kiểm tra desktop 1440px, mobile 390px, đồng bộ/tua/lặp/tốc độ/tìm kiếm, nghe chép lại, điền từ và nhập SRT dùng chung bằng player/API mô phỏng. Microphone và khả năng tải caption của từng video thực tế cần kiểm tra trên môi trường triển khai.
