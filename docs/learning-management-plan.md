# Kế hoạch quản lý phiên quiz, nhóm học và video lớp học

Ngày: 09/10/2026. Trạng thái: đề xuất triển khai, chưa thay đổi mã ứng dụng.

## 1. Mục tiêu và quyết định phạm vi

Tạo ba khu vực liên kết với nhau: **Phiên quiz của tôi**, **Không gian lớp/nhóm học**, và **Video bài học**. Học sinh biết việc cần làm tiếp theo; giáo viên quản lý bài, người học và nội dung tại một nơi.

Quy ước đề xuất cho “1 quiz tối đa 5 phiên”: mỗi người dùng có tối đa **5 phiên đang làm trên cùng một quiz**, cộng chung luyện tập và thi. Phiên đã nộp, hết hạn hoặc đã xóa không chiếm chỗ. Quiz khác và người dùng khác có hạn mức riêng. Đây không phải giới hạn 5 lượt làm suốt đời hay 5 học sinh trong một lớp.

Do file đang mở là `HostLobby.tsx`, cần phân biệt **phiên làm bài cá nhân** với **phòng Live do giáo viên tổ chức**. Kế hoạch ưu tiên phiên cá nhân theo luồng protected quiz hiện có. Nếu yêu cầu 5 phiên nhằm vào phòng Live, áp dụng quy tắc riêng theo `(host_user_id, study_set_id)` cho phòng chờ/đang chạy; không dùng chung bộ đếm hoặc API với phiên cá nhân.

## 2. Cơ sở từ repository

- `apps/web/src/features/live/HostLobby.tsx`: có mã tham gia, danh sách người chơi, làm mới mỗi 3 giây và bắt đầu game; đây là giao diện một phòng, chưa phải bảng quản lý nhiều phiên.
- `services/study/internal/repository/quiz_session.go`: tạo phiên trong transaction, khóa theo người dùng; hiện đếm phiên có `active_until > now()` trên toàn tài khoản và chặn từ 2 phiên. `Recent` chỉ trả ID; interface chưa có thao tác xóa hoặc danh sách metadata đầy đủ.
- `services/study/internal/service/protected_quiz.go`: kiểm tra quyền quiz, giới hạn bắt đầu 20 lần/giờ, tạo snapshot câu hỏi; thời hạn lưu phiên hiện là 24 giờ. Cần tách hạn mức phiên khỏi rate limit.
- `apps/web/src/lib/api/protectedQuiz.ts`, `ProtectedQuizPlayer.tsx`: điểm tích hợp quản lý và tiếp tục phiên.
- `services/class/internal/repository/interfaces.go`: đã có lớp, mã mời, thành viên/vai trò, bộ học liệu và activity. Mở rộng nền tảng này thay vì tạo hệ thống lớp song song.

Các nhận xét trên được kiểm tra bằng CodeGraph với nguồn hiện tại. Trước khi triển khai phải kiểm tra lại route, migration, phân quyền và thay đổi mới trong working tree.

## 3. Trang quản lý phiên quiz

### Giao diện và luồng sử dụng

Route đề xuất `/my/quiz-sessions`; trong trang chi tiết quiz có nút **Quản lý phiên** mở danh sách đã lọc theo quiz.

```text
Phiên quiz của tôi                      [Tìm quiz...]
[Đang làm] [Đã hoàn thành] [Hết hạn]
Quiz: TOEIC luyện tập             Đang dùng 3/5 phiên
Tên phiên | Chế độ | Tiến độ | Bắt đầu | Hạn | Thao tác
Luyện Part 5 | Luyện tập | 12/30 | ... | ... | Tiếp tục · Xóa
                                          [Tạo phiên mới]
```

- Lọc theo quiz, chế độ và trạng thái; sắp xếp theo lần hoạt động gần nhất; phân trang phía server.
- Hiển thị tên quiz, nhãn phiên, câu đã trả lời/tổng câu, thời gian còn lại và ngày cập nhật. Điểm chỉ xuất hiện khi chính sách bài cho phép.
- Tiếp tục đúng câu gần nhất từ state server; refresh hoặc đổi thiết bị không tạo phiên mới.
- Khi đủ 5 phiên, hiển thị “Quiz này có 5/5 phiên đang làm. Tiếp tục hoặc xóa một phiên để tạo phiên mới”, kèm liên kết đến danh sách.
- Xóa một phiên có hộp xác nhận ghi rõ sẽ mất tiến độ chưa nộp. Không tự động xóa phiên cũ để lấy chỗ.
- Có chọn nhiều phiên và xóa hàng loạt sau MVP; server trả kết quả từng phiên để xử lý thất bại một phần.
- Có trạng thái rỗng, đang tải, lỗi kết nối, mất quyền và phiên không còn tồn tại; hỗ trợ bàn phím và màn hình nhỏ.

### Quy tắc backend

1. Đếm theo `(user_id, study_set_id)` với cùng điều kiện phiên hoạt động tại thao tác tạo và tại API danh sách.
2. Giữ khóa transaction phía database; khóa hiện tại theo user vẫn bảo đảm đúng, có thể tối ưu theo user + quiz sau khi đo. Chặn trường hợp hai tab đồng thời tạo phiên thứ 6.
3. Bổ sung API danh sách, lấy metadata/tiếp tục và xóa. Đường dẫn cụ thể chốt theo conventions hiện có; không coi các route trong tài liệu là API đã tồn tại.
4. Phân biệt lỗi nghiệp vụ `QUIZ_SESSION_LIMIT` (đề xuất HTTP 409) và `QUIZ_RATE_LIMIT` (HTTP 429); quota trả `limit`, `activeCount`, `studySetId`.
5. Xóa phải kiểm tra chủ phiên ở server, xử lý idempotent và tuần tự với answer/submit để không ghi lại phiên đã xóa. Có thể đánh dấu `deleted_at` trước rồi dọn state theo lịch.
6. Phiên đã nộp của bài giáo viên giao: không cho học sinh xóa bằng chứng điểm. Có thể ẩn khỏi danh sách cá nhân; lưu kết quả học tập riêng khỏi state phiên tạm.
7. Cân nhắc index `(user_id, study_set_id, active_until)` theo execution plan; cập nhật OpenAPI, migration và frontend API cùng nhau.
8. Khi triển khai, không xóa thêm phiên chỉ vì thay đổi hạn mức. Phiên hết hạn được dọn theo chính sách; lịch sử điểm có thời hạn lưu riêng.

### Nghiệm thu

- Một người tạo được 5 phiên quiz A; phiên thứ 6 bị chặn; vẫn tạo được phiên quiz B.
- Xóa/nộp/hết hạn một phiên giải phóng một chỗ. Người khác không đọc/xóa/tiếp tục phiên đó.
- Hai request đồng thời ở mức 4 chỉ đưa tổng số lên 5; retry tạo phiên không sinh bản sao nếu có idempotency key.
- Tiếp tục giữ đáp án và deadline gốc; mất quyền nội dung được xử lý rõ ràng.
- Xóa không làm mất kết quả bài đã giao hoặc ảnh hưởng dữ liệu người khác.

## 4. Nghiên cứu và thiết kế nhóm học

### Tham khảo đã xác minh

Trang tham gia Wayground phụ thuộc ứng dụng web; URL Study Groups được cung cấp không đọc được nội dung trực tiếp trong phiên nghiên cứu. Đối chiếu bằng trang tính năng và tài liệu hỗ trợ chính thức, không coi màn hình đăng nhập/tham gia là danh sách tính năng đầy đủ.

| Sản phẩm | Tính năng được tài liệu xác nhận | Hướng áp dụng |
|---|---|---|
| Wayground | Lớp tổ chức học sinh, giao học liệu và lọc báo cáo; có thể giao cho học sinh cụ thể | Roster, bài giao toàn lớp/nhóm nhỏ, hạn nộp, báo cáo theo lớp |
| Quizlet Study Groups | Mời bạn, học cùng nhau và theo dõi tiến độ | Nhóm tự học, mục tiêu chung, học liệu chia sẻ, tiến độ nhóm |
| Quizlet Class Progress | Theo dõi hoạt động học và điểm tốt nhất; xem học sinh đã bắt đầu/hoàn thành | Giáo viên nhận diện người chưa làm bài và nội dung cần hỗ trợ |

Nguồn: [Wayground Classes](https://waygroundorg.freshdesk.com/support/solutions/articles/158000404841-classes-on-wayground), [Quizlet Study Groups](https://quizlet.com/features/study-groups), [Hỗ trợ Study Groups](https://help.quizlet.com/hc/en-ca/articles/39890856206733-Studying-with-Study-Groups), [Quizlet Class Progress](https://quizlet.com/features/teacher-class-progress). Tính năng/gói dịch vụ của đối thủ có thể thay đổi; đây là căn cứ thiết kế, không cam kết sao chép toàn bộ sản phẩm.

### Đề xuất để trải nghiệm tốt hơn

Các điểm dưới đây là đề xuất của dự án, chưa phải kết luận rằng đối thủ thiếu chúng:

- Một danh sách **Việc cần làm** gộp quiz, video và ôn tập; ưu tiên bài sắp đến hạn và phần học còn yếu.
- Tách mục đích: **lớp học** có giáo viên, bài bắt buộc và điểm; **nhóm tự học** có mục tiêu chung, người điều phối và tiến độ chia sẻ tự nguyện.
- Mỗi bài video có thể nối với quiz kiểm tra; mỗi kết quả quiz có liên kết quay lại đoạn bài học liên quan.
- Tiến độ giải thích được: đã nộp bài, độ chính xác, phần cần ôn; không xem số lần mở trang là mức độ thành thạo.
- Học sinh mặc định xem tiến độ cá nhân; giáo viên xem lớp mình; nhóm tự học chỉ chia sẻ chỉ số đã đồng ý. Bảng xếp hạng là tùy chọn.

### Cấu trúc giao diện

Route đề xuất `/classes/:classId` với các tab:

| Tab | Học sinh | Chủ lớp / giáo viên |
|---|---|---|
| Tổng quan | Việc cần làm, thông báo, mục tiêu | Tổng quan hoàn thành, bài đến hạn |
| Bài tập | Quiz được giao, tiếp tục/nộp | Tạo bài, đối tượng nhận, lịch, số lượt |
| Học liệu | Quiz, flashcards, thư mục | Thêm/xóa liên kết học liệu, quyền truy cập |
| Video | Xem bài, tiếp tục, quiz đi kèm | Thêm video, sắp xếp chương, lịch phát hành |
| Thành viên | Danh sách phù hợp quyền riêng tư | Mã mời, duyệt tham gia, vai trò, loại thành viên |
| Tiến độ | Kết quả của bản thân | Báo cáo từng bài/người/phần kiến thức |

MVP nhóm học: tham gia bằng mã/link, thu hồi hoặc đổi mã mời, quyền chủ lớp/giáo viên/học sinh, bài giao có deadline và đối tượng, thông báo trong ứng dụng, tiến độ cá nhân và báo cáo lớp. Dùng role hiện có sau khi kiểm tra semantics, không tự cho phép mọi thành viên giao bài.

Giai đoạn tiếp: đồng giáo viên, nhóm nhỏ, mục tiêu tuần, lịch nhắc, ôn phần sai, câu hỏi thảo luận có kiểm duyệt. Hoãn chat thời gian thực và tích hợp LMS đến khi luồng học cốt lõi ổn định.

### Dữ liệu và tích hợp

- Mở rộng dữ liệu lớp với loại không gian, cài đặt tham gia và quyền chia sẻ tiến độ.
- Thêm assignment (lớp, người giao, loại tài nguyên, lịch, deadline, chính sách lượt làm) và assignment recipient cho toàn lớp/nhóm/cá nhân.
- Gắn submission với assignment + người học + phiên; lưu snapshot thông tin cần cho điểm khi học liệu thay đổi.
- Activity/outbox thông báo sự kiện giao bài, nộp bài và xuất bản video; consumer phải xử lý trùng sự kiện.
- Báo cáo dùng kết quả thật từ study/quiz, phân trang và tổng hợp phía server. Khi service phụ thuộc lỗi phải hiển thị dữ liệu chưa khả dụng, tránh báo sai thành “0 tiến độ”.
- Tư cách thành viên không tự động vượt entitlement nội dung trả phí; thống nhất chính sách học liệu được cấp cho lớp với luồng kiểm tra quyền hiện tại.

## 5. Trang video lớp học và bảo vệ nguồn

### Mục tiêu khả thi

Cho chủ lớp thêm video từ nguồn họ quản lý và cấp quyền xem trên website. Không có nút tải, không trả URL nguồn riêng tư hoặc credential của giáo viên trong API học sinh. **Không thể bảo đảm học sinh tuyệt đối không lưu video hoặc quay màn hình** khi thiết bị đã được phép phát.

`controlsList="nodownload"` chỉ điều chỉnh nút điều khiển trình duyệt, không bảo vệ dữ liệu. Xem [MDN controlsList](https://developer.mozilla.org/en-US/docs/Web/API/HTMLMediaElement/controlsList).

### Chọn cách cung cấp video

| Nguồn | Phương án | Mức bảo vệ / giới hạn |
|---|---|---|
| Link nhúng công khai hoặc không công khai | Embed được nhà cung cấp hỗ trợ | Triển khai nhanh; có thể lộ ID/link; không đáp ứng yêu cầu giấu nguồn |
| Video private của nhà cung cấp hỗ trợ API | Chủ lớp kết nối tài khoản hoặc cấp quyền theo cơ chế chính thức | Chỉ phát nếu nhà cung cấp hỗ trợ; không thể phát mọi link private |
| File/link có quyền truy cập hợp lệ | Backend nhập vào kho video riêng rồi chuyển mã | Hướng ưu tiên để giấu nguồn gốc và kiểm soát quyền phát |
| Kho video do website quản lý | Upload trực tiếp có ticket; HLS/DASH và CDN riêng | Kiểm soát tốt hơn, có chi phí lưu trữ/chuyển mã/phát |

Không yêu cầu giáo viên đưa mật khẩu hoặc cookie cá nhân. Link Google Drive/YouTube private không mặc nhiên dùng được trong player tùy biến; chỉ hỗ trợ qua API/OAuth chính thức nếu phù hợp, hoặc hướng dẫn tải lên file được phép sử dụng.

### Luồng đề xuất

1. Chủ lớp mở tab Video, chọn **Thêm video**: nhập tên, mô tả, chương, nguồn, thumbnail, phụ đề và lịch xuất bản.
2. Backend kiểm tra quyền quản lý lớp và nguồn được hỗ trợ. Nếu nhập URL, dùng allowlist/provider adapter, chặn địa chỉ nội bộ, kiểm tra DNS/redirect, kích thước và timeout để tránh SSRF.
3. Job nhập/chuyển mã chạy nền; trạng thái `draft → importing → processing → ready → published`, có `failed` và retry. Không xuất bản khi chưa phát thử thành công.
4. File gốc nằm trong storage private; chỉ lưu source URL/credential ở backend, mã hóa secret và che khỏi log, lỗi, analytics.
5. Học sinh chọn video; backend kiểm tra đăng nhập, thành viên lớp, entitlement nếu có, lịch phát hành và trạng thái video; trả playback ticket có hạn ngắn.
6. CDN/provider kiểm tra quyền cho manifest và mọi segment, phụ đề/thumbnail private nếu cần. Không chỉ ký URL playlist rồi để segment công khai; không redirect học sinh sang URL nguồn.
7. Player hỗ trợ tua, tốc độ, phụ đề, chất lượng, tiếp tục từ vị trí gần nhất và watermark tài khoản. Gia hạn ticket trước khi hết hạn sau khi kiểm tra lại quyền.
8. Khi loại khỏi lớp/thu hồi video: không cấp ticket mới. Với token đã cấp, xác định độ trễ thu hồi tối đa bằng TTL; nếu cần thu hồi tức thì phải kiểm tra phiên tại edge hoặc có cơ chế revoke của provider.

Có thể dùng dịch vụ streaming managed để giảm công vận hành; chọn nhà cung cấp sau thử nghiệm private playback, chi phí và browser compatibility. [Cloudflare Stream security](https://developers.cloudflare.com/stream/viewing-videos/securing-your-stream/) mô tả signed URLs và Allowed Origins; giới hạn domain hỗ trợ chống nhúng ngoài, không thay thế xác thực người xem.

Nếu giáo viên yêu cầu mức bảo vệ cao hơn, đánh giá DRM Widevine/FairPlay/PlayReady và watermark động/forensic ở giai đoạn sau; đây là hạng mục có chi phí, tích hợp thiết bị và vận hành riêng.

### Dữ liệu và trang quản trị

- `video_asset`: owner, provider, asset ID/storage key, nguồn backend-only, trạng thái xử lý, thời lượng, phiên bản, thông báo lỗi đã làm sạch.
- `class_video`: lớp, asset, tiêu đề, chương, thứ tự, lịch xuất bản, quyền và quiz liên quan. Một asset có thể được dùng trong nhiều lớp có quyền.
- `video_progress`: người học, class video, vị trí xem, các khoảng đã xem, cập nhật cuối. Ghi định kỳ có throttle; merge tiến độ từ nhiều thiết bị.
- `playback_session`: người học, video, expiry, revoked state nếu kiến trúc cần kiểm tra trực tuyến; tránh lưu token thô trong log.
- Trang giáo viên hiển thị số video, trạng thái xử lý, dung lượng/thời lượng và lỗi nhập; có sửa, phát thử, xuất bản, thu hồi, xóa.
- Khi xóa asset, kiểm tra lớp đang dùng và bài giao liên quan; không xóa file đang dùng bởi lớp khác. Dọn file bằng job có retry.
- Không khẳng định đã học chỉ từ heartbeat hoặc tua đến cuối; báo cáo xem video là tín hiệu tham gia, quiz sau video mới đo hiểu bài.

### Nghiệm thu video

- Thành viên được phép phát; người ngoài, học sinh bị loại, video chưa xuất bản bị từ chối.
- Ticket hết hạn/chữ ký sai bị từ chối ở cả manifest và segment; làm mới ticket không làm gián đoạn xem bình thường.
- API/HTML/network của học sinh không chứa URL nhập ban đầu, secret hay link file gốc. URL phát tạm thời vẫn quan sát được và phải được kiểm soát.
- URL độc hại, redirect vào mạng nội bộ, nguồn thiếu quyền hoặc quá lớn được từ chối an toàn.
- Safari/mobile và Chrome/desktop phát, tua, phụ đề, phục hồi mạng và resume đạt yêu cầu.
- Ngân sách được ước tính từ phút lưu, phút xem, bitrate, số người xem đồng thời và chi phí chuyển mã trước khi mở rộng.

## 6. Thứ tự triển khai

| Đợt | Công việc | Điều kiện hoàn thành |
|---|---|---|
| 1 | Quản lý phiên cá nhân, giới hạn 5, tiếp tục và xóa | Kiểm thử quyền, concurrent create/delete/submit và luồng UI đạt |
| 2 | Không gian lớp, bài giao, lịch và tiến độ | Giáo viên giao bài; đúng học sinh tiếp tục/nộp; báo cáo không lộ dữ liệu |
| 3 | Thử nghiệm video private với một nguồn/provider | Nhập → xử lý → phát bằng ticket; nguồn gốc không lộ; đo chi phí |
| 4 | Video production, chương/phụ đề/quiz liên quan | Browser QA, thu hồi quyền, retry job và monitoring đạt |
| 5 | Nhóm tự học nâng cao, nhóm nhỏ và ôn phần yếu | Đánh giá mức sử dụng, quyền riêng tư và hiệu quả học |

Chưa đưa thời gian cố định trước khi chốt provider video, phạm vi Live và các API hiện có. Mỗi đợt có migration riêng, rollout có feature flag và kế hoạch rollback; không sửa dữ liệu kết quả học tập khi tắt tính năng.

## 7. Checklist kỹ thuật khi bắt đầu thực hiện

- [ ] Chốt quy tắc 5 phiên cá nhân như mục 1; xác nhận có cần quản lý phòng Live trong cùng đợt hay không.
- [ ] Chốt wireframe desktop/mobile cho danh sách phiên, không gian lớp và hai vai trò xem/quản lý video.
- [ ] Kiểm tra route hiện tại, ownership, entitlement, schema và các thay đổi chưa commit trước khi sửa.
- [ ] Cập nhật contracts/OpenAPI, migration, backend, client API và UI theo từng đợt.
- [ ] Kiểm thử database thực cho hạn mức đồng thời; kiểm thử quyền xuyên người dùng/lớp và vòng đời video.
- [ ] Theo dõi lỗi tạo/xóa/tiếp tục phiên, độ trễ báo cáo, lỗi chuyển mã/phát và chi phí video; log không chứa nguồn riêng tư.
- [ ] Chốt thời hạn lưu state, kết quả bài giao, video và chính sách xóa; tách ẩn lịch sử cá nhân khỏi xóa bằng chứng điểm.

## 8. Các lựa chọn còn cần chốt khi triển khai

1. “5 phiên” có bao gồm phòng Live do chủ quiz tạo không? Mặc định tài liệu áp dụng cho phiên cá nhân.
2. Nguồn video đầu tiên là upload file, kho riêng của giáo viên hay provider private cụ thể? Đề xuất upload + một provider được hỗ trợ, không nhận mọi URL.
3. Quyền xem video chỉ theo thành viên lớp hay có gói trả phí và thời hạn khóa học?
4. Ngân sách video và số người xem đồng thời dự kiến để chọn managed streaming hoặc storage/CDN tự quản.
5. Có cần giữ kết quả bài đã nộp dài hạn và quy trình giáo viên sửa/xóa điểm không?

Các lựa chọn này không cản trở triển khai trang quản lý phiên theo giả định đã ghi rõ ở mục 1.
