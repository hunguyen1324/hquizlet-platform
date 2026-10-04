Bạn có thể gửi **toàn bộ nội dung dưới đây cho Gemini**. Kế hoạch yêu cầu Gemini khảo sát repository trước rồi triển khai từng phần, vì tôi chưa đọc được code hiện tại để xác định framework, database và các API cụ thể.

Mục tiêu thực tế là **ngăn truy cập trái phép, làm khó việc tải hàng loạt và hỗ trợ truy vết nội dung bị chia sẻ**. Người có quyền xem vẫn có thể chụp ảnh, OCR hoặc thu thập nội dung dần trong quá trình học.

---

## Nội dung giao cho Gemini

Bạn hãy khảo sát và triển khai cơ chế bảo vệ nội dung câu hỏi, đáp án và thẻ ghi nhớ trong repository hiện tại.

Thứ tự ưu tiên:

1. Kiểm tra quyền truy cập ở backend.
2. Giảm dữ liệu nội dung được gửi xuống client.
3. Chống thu thập nội dung hàng loạt.
4. Watermark theo người xem.
5. Kiểm soát xuất, nhân bản và chia sẻ nội dung.
6. Chặn sao chép ở giao diện chỉ là tùy chọn phụ.

Hãy thực hiện theo từng giai đoạn, có code, kiểm thử và tài liệu vận hành. Không dừng ở việc đưa ra đề xuất.

### 1. Nguyên tắc và phạm vi

- Đọc `AGENTS.md` và tuân thủ hướng dẫn repository.
- Nếu có thư mục `.codegraph/`, sử dụng CodeGraph trước khi tìm kiếm hoặc đọc code để xác định các luồng liên quan.
- Xác định framework, database, ORM, cơ chế đăng nhập, cache, storage và mô hình triển khai thực tế trước khi thiết kế.
- Tận dụng kiến trúc và thư viện hiện có. Chỉ thêm dependency khi có lý do rõ ràng.
- Không giả định ứng dụng đã có Redis, hàng đợi hoặc hệ thống thanh toán.
- Không tự thay đổi chính sách bộ học công khai, quyền sở hữu hoặc quyền mua nội dung.
- Giữ hoạt động học bình thường: lật thẻ, ôn lại, trả lời câu hỏi, xem kết quả và tiếp tục phiên học.
- Không cam kết chống sao chép tuyệt đối.
- Không xem UUID, URL khó đoán, CORS, mã hóa trong JavaScript hoặc chặn DevTools là cơ chế phân quyền.
- Không sử dụng watermark hoặc chặn Ctrl+C thay thế cho kiểm tra quyền backend.
- Mọi ngưỡng chống tải hàng loạt phải cấu hình được và có kiểm thử.
- Không ghi nội dung câu hỏi, đáp án, token hay thông tin cá nhân đầy đủ vào log chống lạm dụng.
- Không deploy production hoặc thực hiện migration phá hủy dữ liệu khi chưa được yêu cầu.

Nếu có điểm chưa rõ về chính sách sản phẩm, hãy ghi thành giả định riêng và tiếp tục các phần không phụ thuộc vào quyết định đó.

### 2. Khảo sát toàn bộ đường truy cập nội dung

Lập bản đồ các đường mà người dùng có thể lấy câu hỏi, thẻ hoặc đáp án.

Kiểm tra tối thiểu:

| Nhóm | Những gì cần tìm |
|---|---|
| Bộ học | Danh sách bộ, chi tiết bộ, preview, tìm kiếm |
| Câu hỏi | Danh sách, chi tiết, câu hỏi tiếp theo, lời giải |
| Flashcard | Danh sách thẻ, mặt trước, mặt sau, lật thẻ |
| Phiên học | Tạo phiên, tiếp tục, tiến độ, lịch sử |
| Bài kiểm tra | Tạo đề, lấy câu hỏi, nộp bài, xem kết quả |
| Chia sẻ | Link công khai, link riêng, embed |
| Xuất nội dung | CSV, JSON, PDF, in, tải xuống |
| Nhân bản | Sao chép bộ học, import từ bộ khác |
| Tệp | Ảnh, audio, tài liệu đính kèm |
| Dữ liệu render | SSR, hydration, dữ liệu preload, static HTML |
| Cache | CDN, server cache, service worker, browser storage |
| Truy cập khác | GraphQL, WebSocket, server action, API cũ |
| Database trực tiếp | SDK ở frontend, RLS hoặc security rules nếu có |

Với mỗi đường truy cập, ghi:

- File và symbol xử lý.
- Phương thức và route nếu có.
- Điều kiện đăng nhập.
- Quyền truy cập hiện tại.
- Các trường trả về.
- Có trả toàn bộ bộ học không.
- Có trả đáp án trước khi cần không.
- Có giới hạn số lượng không.
- Có thể dùng ID của tài khoản khác để truy cập không.
- Có kiểm tra bộ cha của câu hỏi/thẻ không.
- Có thể lấy nội dung qua đường khác dù giao diện đã ẩn không.

**Đầu ra:** tạo `docs/content-protection-audit.md` hoặc vị trí tài liệu phù hợp với repo.

Phân loại phát hiện:

- **P0:** lấy được nội dung riêng tư hoặc được bảo vệ khi không có quyền.
- **P1:** có quyền học nhưng dễ lấy toàn bộ nội dung hoặc đáp án qua một vài request.
- **P2:** thiếu giám sát, watermark hoặc kiểm soát các chức năng phụ.

Không ghi nhận lỗ hổng chỉ bằng suy đoán. Mỗi phát hiện cần có đường code hoặc kiểm thử chứng minh.

### 3. Xây dựng ma trận quyền truy cập

Từ mô hình dữ liệu hiện có, lập ma trận cho các nhóm:

- Khách chưa đăng nhập.
- Người đăng nhập nhưng không được cấp quyền.
- Người được phép học.
- Chủ sở hữu nội dung.
- Giáo viên hoặc người biên tập nếu hệ thống có.
- Quản trị viên theo phạm vi quyền thực tế.

Phân biệt các hành động:

```text
view_metadata
view_preview
study
reveal_answer
view_results
edit_content
export_content
duplicate_content
manage_sharing
```

Đây là tên minh họa, hãy dùng cách đặt tên phù hợp repository.

Các quy tắc bắt buộc:

1. Backend xác định người dùng từ session hoặc token đã được xác thực.
2. Không tin `userId`, `ownerId`, `role`, `isAdmin` hoặc `hasPurchased` do client gửi.
3. Kiểm tra quyền trên đúng bộ học được yêu cầu.
4. Khi truy cập bằng `questionId` hoặc `cardId`, xác minh đối tượng thuộc bộ học nào.
5. Nếu có lớp học, tổ chức hoặc tenant, kiểm tra phạm vi tương ứng.
6. Quyền học không tự động đồng nghĩa với quyền xuất hoặc nhân bản.
7. Trạng thái bộ học công khai không tự động đồng nghĩa với quyền tải mọi trường dữ liệu.
8. Không tìm thấy quyền phù hợp thì từ chối.
9. Quyền bị thu hồi phải có hiệu lực với các request tiếp theo theo thời gian hiệu lực đã thiết kế.

Không tự thêm các quyền mà sản phẩm chưa có; ghi rõ những quyền cần bổ sung và cách tương thích dữ liệu cũ.

**Đầu ra:** tài liệu ma trận quyền và một cơ chế kiểm tra quyền dùng chung, tránh viết các điều kiện khác nhau ở từng route.

### 4. Triển khai kiểm tra quyền ở backend

Đặt kiểm tra quyền tại tầng dùng chung thích hợp: service, policy hoặc tầng tương đương trong kiến trúc hiện có.

Luồng minh họa:

```text
Xác thực người dùng
    → Xác định tài nguyên và bộ học cha
    → Kiểm tra quyền thực hiện hành động
    → Kiểm tra trạng thái phiên nếu hành động cần phiên
    → Kiểm tra hạn mức
    → Truy vấn các trường được phép trả
    → Trả response tối thiểu
```

Yêu cầu:

- Bao phủ mọi đường truy cập được tìm thấy trong giai đoạn khảo sát.
- Không chỉ gắn middleware ở một route trong khi service hoặc API khác vẫn bỏ qua được.
- Với thao tác ghi, kiểm tra quyền và cập nhật phải tránh khoảng hở do trạng thái thay đổi đồng thời.
- Với thao tác đọc, kiểm tra quyền và truy vấn phải nhất quán; ưu tiên truy vấn có điều kiện phạm vi truy cập khi phù hợp.
- Nếu frontend truy cập database trực tiếp, kiểm tra và cập nhật RLS/security rules. Bảo vệ API riêng sẽ không đủ nếu client vẫn đọc trực tiếp được.
- Khóa đặc quyền hoặc service-role key không được xuất hiện ở frontend.
- Nếu xác thực bằng cookie, áp dụng cơ chế CSRF của stack cho thao tác thay đổi trạng thái.
- Không trả nội dung tài nguyên trong thông báo lỗi.

Quy ước lỗi:

| Trường hợp | Hành vi |
|---|---|
| Chưa xác thực | `401` theo quy ước hệ thống |
| Không có quyền | `403`, hoặc `404` nhất quán nếu cần che sự tồn tại |
| Input không hợp lệ | `400` hoặc mã hiện có |
| Vượt giới hạn | `429` kèm `Retry-After` |
| Phụ thuộc bảo vệ bị lỗi | Response có kiểm soát theo chính sách đã thiết kế |

Không thay đổi mã lỗi hiện có một cách tùy tiện nếu làm hỏng client.

### 5. Giảm lượng nội dung trả xuống client

Tách rõ dữ liệu metadata và nội dung học.

**API danh sách/chi tiết bộ học** chỉ trả những gì giao diện cần, ví dụ:

- Tên bộ học.
- Mô tả.
- Chủ sở hữu hiển thị.
- Số lượng câu/thẻ.
- Trạng thái truy cập.
- Preview được phép theo chính sách sản phẩm.

Không trả toàn bộ câu hỏi hoặc thẻ chỉ để frontend tính số lượng.

**API câu hỏi cho người học:**

- Trả câu hỏi và lựa chọn cần thiết.
- Không trả `correctAnswer`, cờ `isCorrect`, đáp án mẫu, lời giải hoặc dữ liệu chấm điểm nội bộ trước thời điểm được phép.
- Backend chấm bài.
- Chỉ trả phản hồi/lời giải theo chế độ học hoặc kiểm tra.
- Kiểm tra cả các trường gián tiếp làm lộ đáp án.

**API flashcard:**

- Có thể trả cả hai mặt nếu đó là lựa chọn cần thiết cho trải nghiệm học và phạm vi đã được giới hạn.
- Với bộ nội dung cần bảo vệ cao, cân nhắc lấy mặt sau khi lật thẻ.
- Nếu tách mặt sau thành request riêng, kiểm tra quyền và hạn mức trên request đó.
- Không tuyên bố việc tách mặt sau ngăn được người học lấy đáp án.

**Truy vấn và response:**

- Dùng danh sách trường được phép trả.
- Không serialize nguyên bản ghi ORM rồi xóa một vài trường.
- Giới hạn page size ở backend.
- Không cho phép tham số như `limit=999999`, `includeAll=true` hoặc tùy chọn mở rộng quan hệ bỏ qua giới hạn.
- Nếu có GraphQL, kiểm tra số lượng phần tử, alias, batching và độ phức tạp phù hợp.
- Giới hạn lượng nội dung qua nhiều endpoint cộng lại, không chỉ từng route.

**Frontend:**

- Loại bỏ preload toàn bộ bộ học đối với nội dung được bảo vệ.
- Điều chỉnh các hook/store đang phụ thuộc vào toàn bộ nội dung.
- Không nhúng dữ liệu được bảo vệ vào HTML hoặc hydration khi người xem không có quyền.
- Không lưu nội dung được bảo vệ lâu dài vào localStorage, IndexedDB hoặc service worker cache, trừ khi có tính năng offline được thiết kế rõ.
- Xóa dữ liệu nội dung trong bộ nhớ/cache ứng dụng khi logout hoặc chuyển tài khoản.
- Bảo đảm thay đổi không làm hỏng tiến độ học.

### 6. Phiên học nếu cần

Chỉ thêm lớp phiên học mới nếu luồng hiện tại cần nó để kiểm soát việc phân phối nội dung. Nếu đã có phiên học, hãy mở rộng cơ chế hiện có.

Thông tin có thể cần:

```text
sessionId
userId
setId
mode
createdAt
expiresAt
status
contentVersion
```

Quy tắc:

- Phiên gắn với người dùng và bộ học.
- Không dùng `sessionId` của người khác.
- Không dùng phiên cho bộ học khác.
- Phiên hết hạn hoặc bị thu hồi không tiếp tục cấp nội dung.
- Phiên không thay thế kiểm tra quyền bộ học.
- Tạo nhiều phiên mới không được đặt lại hạn mức của tài khoản.
- Nếu dùng cursor có chữ ký, chữ ký được tạo tại server và gắn với phạm vi truy vấn.
- Cursor không thay thế phân quyền.
- Người học phải ôn lại được câu/thẻ đã xem theo thiết kế.
- Không ép học tuần tự nếu sản phẩm hiện hỗ trợ nhảy câu hoặc chọn thẻ.

Nếu làm chế độ “câu tiếp theo”, backend quyết định nội dung được trả theo phiên. Client không được gửi danh sách ID tùy ý để kéo cả bộ.

### 7. Chống tải hàng loạt bằng nhiều lớp

Không chỉ đếm request. Một request có thể chứa rất nhiều câu/thẻ.

Thiết kế tối thiểu hai nhóm hạn mức:

**A. Hạn mức request**

- Theo tài khoản.
- Theo phiên.
- Theo nhóm endpoint.
- Theo IP như tín hiệu phụ.
- Cho cả các request bị từ chối nếu cần bảo vệ tài nguyên backend.

**B. Hạn mức nội dung**

- Số lượng câu/thẻ thực tế được cấp.
- Số nội dung mới khác nhau trong một khoảng thời gian.
- Số bộ học khác nhau được truy cập.
- Hành động reveal đáp án.
- Hoạt động export/duplicate nếu được phép.

Ví dụ cấu hình ban đầu để đánh giá, **không coi là ngưỡng phù hợp sẵn cho mọi ứng dụng**:

| Hạng mục | Giá trị thử nghiệm |
|---|---:|
| Page size mặc định | 10 câu/thẻ |
| Page size tối đa cho luồng học | 20 câu/thẻ |
| Prefetch | 1 batch tiếp theo |
| Request cấp nội dung | 60/phút/tài khoản |
| Nội dung mới được cấp | 300/10 phút/tài khoản |
| Tạo phiên học | 10/10 phút/tài khoản |

Trước khi chọn ngưỡng cuối cùng:

- Kiểm tra luồng ôn thẻ nhanh.
- Kiểm tra giao diện hiện tải những batch nào.
- Kiểm tra việc retry hoặc prefetch có làm tăng đếm bất hợp lý không.
- Dùng telemetry thực tế nếu có.
- Phân biệt hạn mức chống lạm dụng với quota sản phẩm.

**Yêu cầu triển khai:**

- Nếu chạy nhiều process/server, dùng bộ đếm dùng chung và thao tác tăng/kiểm tra nguyên tử.
- Nếu đã có Redis, tận dụng Redis hoặc thư viện hiện có phù hợp.
- Nếu chưa có, thiết kế phương án dựa trên hạ tầng thực tế; ghi rõ giới hạn của bản một instance.
- Không triển khai bằng `Map` trong RAM rồi tuyên bố đã bảo vệ được hệ thống nhiều server.
- Kiểm thử request đồng thời để tránh vượt hạn mức do race condition.
- Key của limiter phải có phạm vi rõ ràng và TTL.
- Giới hạn số key để tránh tạo dữ liệu không kiểm soát.
- Tách hạn mức request tổng với hạn mức nội dung mới, để retry không cấp thêm quota nhưng vẫn không tạo được vòng lặp request miễn phí.
- Ngân sách nội dung phải được đặt chỗ/kiểm tra an toàn trước khi trả response.
- Không cho phép đổi endpoint, đổi phiên hoặc đổi bộ học để bỏ qua hạn mức tài khoản.
- IP không phải định danh chính vì nhiều học sinh có thể chung mạng.
- Chỉ tin forwarded headers từ proxy đã cấu hình.
- Không tự khóa tài khoản chỉ vì một tín hiệu tốc độ.
- Client xử lý `429` bằng thông báo rõ và thời gian thử lại.
- Không retry liên tục sau `429`.

**Khi bộ đếm bị lỗi:**

Thiết kế và ghi rõ chính sách theo nhóm thao tác:

- Truy cập không có quyền vẫn phải bị từ chối.
- Xuất hàng loạt hoặc chức năng nhạy cảm có thể tạm dừng.
- Luồng học có thể dùng phương án giảm tải dự phòng nếu phù hợp.
- Không âm thầm bỏ toàn bộ giới hạn.
- Có metric và cảnh báo lỗi phụ thuộc.

### 8. Phát hiện hành vi thu thập bất thường

Nếu hạ tầng hiện tại hỗ trợ, bổ sung sự kiện tối thiểu:

```text
content_access_denied
content_rate_limited
content_session_created
content_session_expired
content_export_attempted
content_bulk_pattern_detected
```

Dữ liệu log phù hợp:

- Mã tài khoản nội bộ hoặc định danh giả danh.
- Mã bộ học.
- Mã phiên hoặc định danh phiên an toàn.
- Nhóm hành động.
- Số lượng nội dung được cấp.
- Thời gian.
- Request correlation ID.
- Lý do từ chối.

Không log:

- Nội dung câu hỏi và đáp án.
- Token đăng nhập hoặc session secret.
- Email đầy đủ nếu không cần.
- URL có secret.
- Toàn bộ request body.

Các tín hiệu có thể theo dõi:

- Lấy liên tục nhiều nội dung chưa từng xem.
- Mở nhiều bộ học trong thời gian ngắn.
- Tạo phiên liên tục.
- Nhiều request đồng thời từ cùng tài khoản.
- Liên tục bị giới hạn rồi đổi đường truy cập.

Mỗi tín hiệu chỉ là bằng chứng hỗ trợ. Không kết luận gian lận từ một ngưỡng đơn lẻ.

Triển khai theo mức:

1. Ghi nhận.
2. Cảnh báo.
3. Giới hạn tạm thời.
4. Kiểm tra bổ sung nếu hệ thống đã có khả năng phù hợp.

Không tự thêm CAPTCHA, khóa vĩnh viễn hoặc hệ thống fingerprint thiết bị phức tạp trong giai đoạn đầu.

### 9. Watermark theo người xem

Watermark nhằm răn đe và hỗ trợ truy vết ảnh chụp. Nó có thể bị xóa khỏi DOM hoặc cắt khỏi ảnh, nên không được mô tả là cơ chế ngăn lấy dữ liệu API.

**Nội dung watermark đề xuất:**

```text
Tên nền tảng · Mã người xem · Mã phiên ngắn
```

Yêu cầu:

- Mã người xem là định danh phục vụ truy vết, không phải token đăng nhập.
- Ưu tiên mã giả danh; không mặc định hiển thị email hoặc số điện thoại.
- Mã do backend cấp và gắn với người xem đã xác thực.
- Nếu cần định danh ảnh theo thời điểm, backend cấp thời gian hoặc mã tương ứng.
- Không dùng dữ liệu client tự khai làm danh tính watermark.
- Có khả năng tra mã về tài khoản theo quyền quản trị phù hợp.
- Watermark không phải bằng chứng tuyệt đối về người làm lộ nội dung.

**Cách hiển thị:**

- Xuất hiện trong vùng câu hỏi, đáp án và cả hai mặt flashcard.
- Có thể lặp nhẹ tại nhiều vị trí để khó cắt bỏ hoàn toàn.
- Dùng mật độ và độ tương phản vừa đủ, không làm khó đọc.
- Không che lựa chọn, nút hoặc nội dung.
- Lớp overlay không bắt sự kiện chuột/chạm.
- Không làm screen reader đọc lặp lại watermark.
- Hoạt động trên mobile, desktop, dark mode và fullscreen nếu có.
- Kiểm tra mặt thẻ trước/sau khi dùng transform animation.
- Nếu cho phép in hoặc export, quy định riêng watermark trong đầu ra.
- Với người chưa đăng nhập xem preview công khai, không tạo danh tính cá nhân giả.

Không làm ảnh hoặc canvas thay thế toàn bộ văn bản chỉ để chống copy, vì ảnh hưởng tìm kiếm, khả năng tiếp cận và vẫn bị OCR.

### 10. Kiểm soát export, duplicate và sharing

Kiểm tra quyền riêng cho từng chức năng ở backend.

**Export:**

- Có quyền export mới tạo được file.
- Nếu export chạy nền, kiểm tra quyền khi tạo job và trước lúc tạo/cấp file theo thiết kế.
- Không cho phép job cũ trở thành đường lấy nội dung sau khi bị thu hồi quyền.
- File xuất được bảo vệ bằng cơ chế download có kiểm tra quyền hoặc URL có thời hạn phù hợp.
- URL có chữ ký là bearer credential; người có URL có thể dùng trong thời gian hiệu lực.
- Giới hạn lượng export và số job đồng thời.
- Nếu hệ thống chỉ có URL có chữ ký, ghi rõ hạn chế thu hồi trước khi URL hết hạn.

**Duplicate:**

- Quyền học không tự cấp quyền nhân bản.
- Backend kiểm tra bộ nguồn.
- Không nhận danh sách câu/thẻ rồi nhân bản mà bỏ qua quyền nguồn.
- Nếu cho phép nhân bản, xác định metadata nguồn cần giữ.

**Sharing:**

- Link chia sẻ tuân theo visibility và phạm vi hiện có.
- Token chia sẻ đủ ngẫu nhiên.
- Có thể thu hồi.
- Giới hạn phạm vi được xem.
- Có thời hạn nếu chính sách sản phẩm yêu cầu.
- Không dùng link chia sẻ để mở rộng quyền edit/export ngoài ý muốn.

**Tệp đính kèm:**

- Kiểm tra ảnh/audio riêng tư có đang nằm ở bucket hoặc URL công khai không.
- Nếu cần, chuyển sang cơ chế cấp quyền phù hợp với storage hiện tại.
- Ghi rõ nội dung đã tải xuống trước đó không thể bị thu hồi khỏi thiết bị người xem.

### 11. Cache và dữ liệu lưu ở client

Kiểm tra riêng đường cache vì có thể làm lộ nội dung dù API đã phân quyền.

Yêu cầu:

- Response chứa nội dung cá nhân hoặc được bảo vệ không đi vào shared cache dùng chung giữa người dùng.
- Chọn `Cache-Control` phù hợp; ưu tiên `private, no-store` cho response nhạy cảm nếu không có lý do khác.
- Kiểm tra CDN có ghi đè header không.
- Cache server, nếu có, phải kiểm tra quyền trước khi trả dữ liệu.
- Cache theo quyền phải có cách vô hiệu hóa khi quyền đổi.
- Không chỉ dựa vào cache key chứa `userId` để thay thế kiểm tra quyền.
- Kiểm tra framework SSR/static generation có tái sử dụng response giữa người dùng không.
- Service worker không cache nhầm API được bảo vệ.
- Logout hoặc đổi tài khoản xóa query cache/store liên quan.
- Tính năng offline, nếu có, phải được tài liệu hóa riêng vì dữ liệu đã tải xuống khó thu hồi.

### 12. Chặn copy ở giao diện — tùy chọn phụ

Chỉ triển khai sau các lớp backend.

Nếu có cấu hình bật:

- Áp dụng riêng vùng nội dung được bảo vệ.
- Không chặn copy trong ô nhập câu trả lời.
- Không chặn paste, điều hướng bàn phím hoặc shortcut toàn ứng dụng.
- Không chặn thao tác trợ năng.
- Không dùng script phát hiện DevTools.
- Không tự khóa người dùng khi họ nhấn Ctrl+C.
- Ghi rõ trong tài liệu rằng người dùng vẫn có thể lấy nội dung đã nhận.

Nếu ảnh hưởng trải nghiệm nhiều hơn giá trị bảo vệ, để mặc định tắt.

### 13. Bộ kiểm thử bắt buộc

Dùng công cụ kiểm thử hiện có trong repo. Ưu tiên integration test cho phân quyền, response và limiter; không chỉ mock policy rồi kiểm tra route gọi hàm đó.

**A. Phân quyền**

- Khách không đọc được nội dung riêng tư.
- Người A không đọc được bộ riêng của người B.
- Đổi `setId`, `questionId`, `cardId` không vượt quyền.
- Thẻ/câu hỏi thuộc bộ khác bị từ chối.
- Người học hợp lệ đọc được nội dung theo chính sách.
- Chủ sở hữu vẫn quản lý được nội dung.
- Quyền quản trị tuân thủ phạm vi hệ thống.
- Thu hồi quyền chặn request tiếp theo.
- Gọi API trực tiếp vẫn bị kiểm soát.
- API cũ hoặc server action không tạo đường bỏ qua.
- Nếu có truy cập database trực tiếp, kiểm thử RLS/security rules.

**B. Dữ liệu trả về**

- API metadata không chứa toàn bộ nội dung.
- API câu hỏi không chứa đáp án trước thời điểm cho phép.
- SSR/hydration không nhúng dữ liệu vượt quyền.
- Page size lớn bị giới hạn hoặc từ chối.
- Batch gồm nhiều ID không vượt giới hạn.
- Mặt sau thẻ được trả đúng theo thiết kế.
- Kết quả bài kiểm tra không làm lộ nội dung chưa được phép xem.

**C. Phiên học**

- Không dùng được phiên của người khác.
- Không dùng phiên cho bộ khác.
- Phiên hết hạn bị xử lý đúng.
- Cursor sửa đổi bị từ chối nếu dùng cursor có chữ ký.
- Tạo phiên mới không đặt lại hạn mức tài khoản.
- Ôn lại và tiếp tục phiên hoạt động.

**D. Chống tải hàng loạt**

- Vượt hạn mức trả `429` và `Retry-After`.
- Request đồng thời không vượt ngân sách do race condition.
- Nhiều endpoint dùng chung hạn mức nội dung đúng thiết kế.
- Nhiều server dùng chung bộ đếm.
- Tăng page size không tránh được quota.
- Retry không tính như nội dung mới nhưng vẫn chịu request limit.
- Chung IP không khiến mọi tài khoản bị khóa không hợp lý.
- Giả forwarded header không bỏ qua giới hạn.
- Lỗi phụ thuộc được xử lý đúng chính sách.
- Luồng học bình thường không bị chặn ở ngưỡng thử nghiệm.

**E. Cache**

- Người B không nhận response đã cache của người A.
- Logout/chuyển tài khoản xóa dữ liệu liên quan.
- Service worker không giữ nội dung được bảo vệ trái thiết kế.
- Thu hồi quyền không bị cache phân quyền kéo dài ngoài thời gian đã cam kết.

**F. Watermark và trải nghiệm**

- Watermark đúng người xem.
- Hiển thị ở câu hỏi, lời giải và hai mặt thẻ.
- Không che nội dung trên màn hình nhỏ.
- Không bắt sự kiện click/touch.
- Không bị screen reader đọc lặp.
- Dark mode và fullscreen hoạt động.
- Người dùng vẫn lật thẻ, trả lời và xem kết quả bình thường.

**G. Export/share/tệp**

- Không có quyền export thì không tạo hoặc tải được file qua đường được kiểm soát.
- Duplicate kiểm tra quyền nguồn.
- Link đã thu hồi không cấp thêm nội dung.
- Tệp riêng không truy cập được qua URL công khai ngoài thiết kế.
- Tài liệu nêu rõ giới hạn của URL có chữ ký.

### 14. Triển khai theo thứ tự

| Giai đoạn | Công việc | Điều kiện hoàn thành |
|---|---|---|
| 1 | Khảo sát và ma trận quyền | Có bản đồ đường truy cập, phát hiện có bằng chứng |
| 2 | Sửa phân quyền backend | Các kiểm thử truy cập trái phép đạt |
| 3 | Giảm payload, bảo vệ đáp án | Client học được mà không tải toàn bộ nội dung |
| 4 | Limiter và telemetry | Có kiểm thử đồng thời và cấu hình ngưỡng |
| 5 | Watermark | Hiển thị đúng, không gây lỗi trải nghiệm |
| 6 | Export/share/cache/tệp | Đã kiểm tra các đường phụ |
| 7 | Staging và rollout | Có hướng dẫn vận hành, rollback và giới hạn còn lại |

Nguyên tắc rollout:

- Sửa lỗi phân quyền rõ ràng trước.
- Với ngưỡng phát hiện hành vi, chạy ghi nhận trước nếu có môi trường phù hợp.
- Giới hạn page size vẫn phải có trần an toàn.
- Triển khai thay đổi API và frontend theo cách tương thích.
- Theo dõi tỷ lệ `429`, lỗi phiên và gián đoạn học.
- Điều chỉnh ngưỡng dựa trên dữ liệu.
- Không dùng rollback làm mở lại lỗi phân quyền đã sửa.

Nếu dùng feature flag, phân biệt rõ:

```text
watermark_enabled
bulk_detection_enforcement_enabled
copy_restriction_enabled
```

Không tạo một flag chung có thể vô tình tắt mọi kiểm tra quyền.

### 15. Tài liệu và kết quả bàn giao

Sau khi hoàn thành, trả lại:

1. **Những vấn đề đã xác nhận:** route/file liên quan và mức độ.
2. **Ma trận quyền cuối cùng.**
3. **Các file đã sửa và lý do.**
4. **Migration và cách áp dụng**, nếu có.
5. **Biến môi trường/cấu hình mới**, giá trị mặc định và phạm vi.
6. **Hạn mức thực tế đã chọn** và lý do.
7. **Kiểm thử đã chạy**, kết quả và kiểm thử chưa chạy.
8. **Hướng dẫn kiểm tra thủ công** bằng hai tài khoản có quyền khác nhau.
9. **Hướng dẫn giám sát và điều chỉnh ngưỡng.**
10. **Hướng dẫn rollback an toàn.**
11. **Giới hạn còn lại**, gồm chụp màn hình, OCR, xóa watermark và thu thập chậm bởi tài khoản hợp lệ.

Không báo “đã bảo vệ hoàn toàn”. Chỉ báo những đường đã kiểm tra và hành vi đã chứng minh.

### 16. Tiêu chí nghiệm thu cuối cùng

Công việc đạt yêu cầu khi:

- Người không có quyền không lấy được nội dung qua các đường truy cập đã khảo sát.
- Đổi ID không vượt được quyền.
- API học thông thường không trả toàn bộ bộ học được bảo vệ.
- Đáp án kiểm tra được giữ ở backend đến thời điểm cho phép.
- Có giới hạn lượng nội dung, không chỉ số request.
- Đổi phiên hoặc route không đặt lại hạn mức tài khoản.
- Bộ đếm phù hợp số instance triển khai.
- Watermark gắn đúng người xem và không cản trở học.
- Export, duplicate, sharing và tệp tuân thủ quyền.
- Cache không làm lộ dữ liệu giữa tài khoản.
- Luồng học hợp lệ vẫn hoạt động.
- Có kiểm thử chứng minh các điểm trên và tài liệu về giới hạn.

---

Cơ sở của kế hoạch là kiểm tra quyền trên từng request và từng tài nguyên, cùng giới hạn lượng dữ liệu/tài nguyên API. Đây là các hướng được nêu trong [OWASP Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html), [API1: Broken Object Level Authorization](https://owasp.org/API-Security/editions/2023/en/0xa1-broken-object-level-authorization/) và [API4: Unrestricted Resource Consumption](https://owasp.org/API-Security/editions/2023/en/0xa4-unrestricted-resource-consumption/).
