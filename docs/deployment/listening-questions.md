# Câu hỏi nghe và phụ đề

Trong màn hình tạo/sửa bộ câu hỏi, mỗi câu có khối audio và phụ đề:

1. Tải audio lên (tối đa 30 MiB), bấm **Nghe thử** để kiểm tra. Khi tạo bộ mới, lần tải đầu tạo một bản nháp riêng tư; bấm Lưu để lưu câu hỏi.
2. Chọn file SRT UTF-8 (tối đa 1 MiB) hoặc nhập nội dung trực tiếp. Có thể sửa, tải xuống hoặc xóa phụ đề.
3. Hoặc chọn ngôn ngữ tự động/Việt/Nhật/Anh rồi bấm **Tạo phụ đề bằng STT**. Backend chuyển audio đã tải lên tới coordinator; hai worker xử lý các tác vụ song song. Một file chạy trên một worker.
4. Có thể lưu và quay lại trong lúc chờ. Màn hình tìm tác vụ gần nhất theo audio đã lưu. Kết quả không ghi đè phần SRT người dùng vừa sửa; dùng nút áp dụng kết quả khi cần. Sau khi áp dụng, bấm Lưu để lưu phụ đề vào câu hỏi.

Thay audio sẽ xóa phụ đề của audio cũ. Hủy một bộ mới tạo bằng luồng tải audio sẽ xóa bản nháp đó. Đóng tab trước khi lưu có thể để lại bản nháp riêng tư.

## Backend và cấu hình

Study tự chạy migration `024_listening`: thêm `quiz_question.srt_text` và bảng liên kết tác vụ với người dùng/bộ câu hỏi. SRT lưu cùng câu hỏi; audio dùng kho riêng tư hiện có. Nội dung SRT được kiểm tra ở frontend và backend. API làm bài chỉ trả transcript khi được phép xem đáp án, tránh lộ nội dung khi thi.

Các route nằm dưới `/v1/study-sets/{setId}/listening` và yêu cầu quyền sửa bộ câu hỏi:

| Method | Route | Chức năng |
| --- | --- | --- |
| GET | `/audio?source=private-audio:...` | Nghe thử audio riêng tư của bộ |
| POST | `/transcriptions` | JSON `{ "audioUrl": "private-audio:...", "language": "vi" }`, tạo tác vụ |
| GET | `/transcriptions/{jobId}` | Trạng thái, kèm `srtText` khi hoàn tất |
| GET | `/transcriptions?audioUrl=...` | Tìm lại tác vụ gần nhất của audio |

Study dùng `STT_API_URL` và `STT_API_TOKEN`. Docker Compose ánh xạ `stt-app-host:host-gateway`, dùng `STT_DOCKER_API_URL` (mặc định `http://stt-app-host:8091`). Không đưa token vào biến `VITE_*`. Nếu thiếu cấu hình STT, tải audio và nhập SRT thủ công vẫn sử dụng được; tạo phụ đề tự động báo dịch vụ chưa sẵn sàng.

Audio chuyển nội bộ từ Study tới coordinator, không nhận URL tải tùy ý. Mỗi người dùng được tạo tối đa 10 tác vụ/giờ. Giới hạn thời lượng STT là 20 phút; kết quả coordinator giữ 24 giờ, vì vậy cần áp dụng và lưu SRT vào câu hỏi trước khi hết hạn.

## Trạng thái triển khai

- Hai worker và coordinator đã chạy, đã kiểm chứng chuyển audio từ App và nhận SRT từ cả hai worker.
- Mã Study/frontend mới nằm trong workspace, chưa phát hành lên website.
- Đã mở UFW chỉ cho TCP từ bridge `br-2584f7eeccc8` / subnet Docker `172.18.0.0/16` tới listener `172.17.0.1:8091`; không mở cổng ra Internet.
- Đã đổi token API và đồng bộ coordinator, môi trường container Study và `.env` local. Coordinator/Study đã khởi động lại, healthcheck từ container tới STT thành công. Unit coordinator khởi động sau Docker để địa chỉ bridge sẵn sàng.
- Người dùng đã xác nhận cho phép thay đổi firewall/token. Cấu hình cũ sao lưu trong thư mục root-only `/root/stt-bridge-backup` trên App; bản sao chứa token cũ, cần giữ riêng tư. Nếu Docker tạo lại mạng với bridge/subnet khác, phải cập nhật quy tắc UFW tương ứng.

## Kiểm thử đã thực hiện

- Frontend: 58 tests, gồm parse SRT UTF-8/BOM/CRLF và dữ liệu không hợp lệ.
- Go Study: tests HTTP, phân quyền transcript, validation SRT; integration với PostgreSQL riêng kiểm tra migration chạy lại, lưu SRT, giới hạn chủ sở hữu/bộ câu hỏi, nghe thử và vòng đời tác vụ với STT giả lập.
- Module STT: tests và vet; build Linux.
- Kiểm thử thật từ container Study: hai audio mẫu được gửi qua endpoint/token đã cấu hình, xử lý trên d0 và 2fe, tải lại SRT qua cùng mạng Docker thành công. Token cũ và yêu cầu không có token đều bị từ chối (401); Study vẫn healthy. Dùng client Go gửi nhị phân; BusyBox wget trong container không gửi đúng file WAV ở phép thử đầu.
- Chưa kiểm thử giao diện bằng trình duyệt do browser host không khả dụng. Chưa kiểm thử luồng giao diện website thật vì Study/frontend mới chưa được phát hành.

Xem thêm [STT API](stt-api.md) để vận hành coordinator và worker.
