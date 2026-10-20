# Đối chiếu Quizlet & Wayground và đề xuất tiến hoá web hquizlet

> Trạng thái: đề xuất, chưa sửa code. Ngày: 2026-10-20.
> **Giới hạn nguồn (đọc trước):** `wayground.com` chặn công cụ đọc tự động (robots), nên phần Wayground lấy từ thông cáo báo chí, bài hướng dẫn và trang đánh giá; trang chủ `quizlet.com` chỉ có nội dung giới thiệu. **Mình chưa thấy giao diện render của hai trang**, nên phần "UI" dưới đây là *mẫu tương tác/bố cục* rút từ mô tả, không phải sao chép giao diện cụ thể. Bạn nên mở hai trang, chụp màn hình các chỗ bạn thích, mình sẽ bám sát hơn.
> Không sao chép thiết kế, màu, logo, tên chế độ độc quyền của hai bên. Chỉ học **nguyên tắc trải nghiệm**.

## 1. hquizlet đã có gì (đỡ làm lại)

Trong `apps/web/src/features`: **Flashcards, Learn, Test, Match** (`learning/`), **Live** (`live/`), **lớp học** (`classes/`), **thư mục** (`folders/`), **tìm kiếm** (`search/`), **dashboard cá nhân/lớp/admin**, **ví/thanh toán**, **quiz bảo vệ + giao diện TOEIC** (`study-sets/exam`, `quiz`). Nên phần lớn việc là **gom lại, đánh bóng và thống nhất trải nghiệm**, không phải thêm chế độ mới.

## 2. Hai nền tảng làm tốt điều gì

### Quizlet — "học một mình, nhanh, ít ma sát"
Từ nguồn đã đọc: thẻ ghi nhớ là lõi; **Learn** (luyện thích ứng, đưa lại thuật ngữ hay sai), **Test** (tạo bài kiểm tra: trắc nghiệm / tự luận / đúng-sai / ghép), **Match/Gravity** (trò chơi có giờ), **Live** (chơi nhóm trong lớp); tìm bộ có sẵn của cộng đồng; AI dựng thẻ từ ghi chú/PDF và giải thích vì sao sai; ứng dụng di động có học ngoại tuyến; thư viện từ vựng có âm thanh. Điểm yếu người dùng hay nêu: **paywall**, quảng cáo, bộ do người dùng tạo có lỗi.
- **Bài học UI:** vào trang một bộ là thấy ngay *các cách học* như một dãy thẻ lớn; mọi chế độ chia sẻ chung dữ liệu và tiến độ; cài đặt ngắn gọn ngay trước khi bắt đầu.

### Wayground (trước là Quizizz) — "giáo viên điều khiển, học sinh chơi, có số liệu"
Từ nguồn đã đọc: vào bằng **mã/đường dẫn**; ba cách chạy: **live do giáo viên điều khiển**, **live học sinh tự nhịp**, **giao bài về nhà**; điểm, đồng hồ, âm thanh, bảng xếp hạng, ảnh đại diện (**có thể giảm hiệu ứng, ẩn bảng xếp hạng, giảm âm thanh**); **báo cáo tự động** (tham gia, độ chính xác, thời gian trả lời, từng câu, lỗi sai chung); hỗ trợ **dịch, đọc to, phông cho người khó đọc**, điều chỉnh độ khó bằng AI; gom hoạt động thành **gói bài**. Điểm yếu nêu trong đánh giá: **trễ/lag, lỗi đăng nhập**, bản miễn phí hạn chế, "đợi mọi người xong mới kết thúc".
- **Bài học UI:** màn học sinh cực đơn giản (nhập mã → tên → chơi); giáo viên thấy **bảng theo dõi trực tiếp**; sau buổi có **báo cáo rõ "câu nào cả lớp sai"**.

## 3. Khoảng cách so với hquizlet và đề xuất

| # | Chủ đề | Học từ | Đề xuất cho hquizlet | Ưu tiên |
|---|---|---|---|---|
| 1 | **Trang bộ đề (study set)** | Quizlet | Hàng thẻ chế độ lớn ở đầu trang: Thẻ ghi nhớ · Học (Learn) · Kiểm tra · Ghép · Làm bài quiz · Chơi live. Mỗi thẻ hiện *tiến độ của bạn* (vd "62% đã thuộc"). Nút chính duy nhất "Tiếp tục học". Chế độ nào không hợp loại bộ (quiz TOEIC vs từ vựng) thì ẩn, không để xám khó hiểu. | **Cao** |
| 2 | **Luồng bắt đầu một chế độ** | Quizlet | Hộp cài đặt ngắn *một bước* (số câu, thứ tự, hiện đáp án), nhớ lựa chọn lần trước. Khớp với màn bắt đầu quiz đã đề xuất ở `quiz-ui-redesign-and-performance.md` mục 5.3. | **Cao** |
| 3 | **Khung làm bài thống nhất** | cả hai | Một khung (thanh trên: thoát · tiến độ · giờ · đánh dấu; thanh đáy: Trước/Tiếp + bảng câu) dùng cho Learn/Test/Quiz/TOEIC. Chi tiết ở mục 5 kế hoạch quiz. | **Cao** |
| 4 | **Vào phòng live bằng mã** | Wayground | Màn "Nhập mã → nhập tên → vào" một cột, to, chạy tốt trên điện thoại; hiện tên phòng/giáo viên để chắc đúng phòng; gợi ý khi mã sai. Kết hợp rate limit (xem `rate-limit-plan.md`). | **Cao** |
| 5 | **Ba cách chạy bài trong lớp** | Wayground | Khi giáo viên giao một bộ cho lớp: chọn **Live (giáo viên điều khiển)** / **Live (học sinh tự nhịp)** / **Giao bài về nhà (có hạn nộp)**. Hiện đã có Live và lớp; cần xem *giao bài về nhà có hạn nộp* đã có chưa (mình chưa kiểm tra sâu). | Trung bình |
| 6 | **Báo cáo sau buổi học** | Wayground | Trang báo cáo cho giáo viên: tham gia/hoàn thành, độ chính xác, thời gian trung bình, **top câu sai nhiều nhất + đáp án sai phổ biến**, bảng từng học sinh, xuất CSV. Tái dùng dashboard lớp (`classes/:id/dashboard`). | Trung bình |
| 7 | **Giảm kích thích / trợ năng** | Wayground | Công tắc "Chế độ tập trung": tắt âm thanh, ẩn bảng xếp hạng, giảm chuyển động; cỡ chữ A−/A+; phông dễ đọc; tương phản cao. Đây là lợi thế dễ làm mà đa số trang kém. | Trung bình |
| 8 | **Học thích ứng (Learn)** | Quizlet | Đảm bảo Learn dựa trên lịch lặp lại ngắt quãng (thuật ngữ sai quay lại sớm hơn), và hiện "còn N thẻ chưa thuộc". Kiểm tra `LearnMode.tsx` đang làm đến đâu rồi mới quyết. | Trung bình |
| 9 | **Tạo bộ nhanh bằng AI / nhập hàng loạt** | cả hai | Dán văn bản/CSV/PDF → xem trước thẻ → sửa → lưu. Đã có import (`crates/import-core`); thêm bước *xem trước & sửa lỗi* ngay trong luồng tạo. AI sinh thẻ chỉ làm sau, có kiểm duyệt chất lượng (điểm yếu chung: câu AI đôi khi sai). | Thấp–TB |
| 10 | **Tìm kiếm & thư viện cộng đồng** | Quizlet | Ô tìm kiếm nổi bật trên trang chủ, lọc theo môn/loại (TOEIC, từ vựng, ngữ pháp), nhãn "đã xác minh" để giảm bộ sai. | Thấp–TB |
| 11 | **Ngoại tuyến / PWA** | Quizlet | Học thẻ ngoại tuyến cho bộ *không bảo vệ*. Quiz bảo vệ (chống copy) **không** nên cache ngoại tuyến. | Thấp |
| 12 | **Trang chủ** | cả hai | Chia hai lối "Tôi là học sinh / Tôi là giáo viên" (Quizlet làm vậy); học sinh: ô tìm kiếm + "Tiếp tục học"; giáo viên: "Tạo bộ", "Bắt đầu live", lớp của tôi. | Thấp–TB |

### Những thứ **không** nên bắt chước
- **Paywall khắt khe** làm người dùng phàn nàn ở Quizlet: nếu dùng mô hình trả phí, giữ *lõi học miễn phí*, chỉ tính các phần tốn chi phí (AI, quiz bảo vệ…).
- **Trễ khi live** (than phiền ở Wayground): ưu tiên độ ổn định — SSE/live đã có, thêm đo độ trễ và test tải trước khi quảng bá.
- **Bắt cả phòng đợi người chậm nhất** để kết thúc: cho giáo viên nút "Kết thúc ngay".
- Sao chép tên chế độ/hình ảnh/linh vật/âm thanh của hai bên.

## 4. Hướng thiết kế (rút ra, áp cho toàn site)

1. **Một hệ thiết kế**: dùng token màu hiện có (`--primary`, `--card`…) + bộ component chung (nút, thẻ chế độ, chip, bottom sheet, thanh tiến độ). Dọn việc song song hai hệ style (CSS thuần `qz-*` và Tailwind) — chọn một.
2. **Mobile trước**: vùng bấm ≥ 44 px, nút chính cố định đáy; học sinh chủ yếu dùng điện thoại.
3. **Mỗi màn một việc chính** và trạng thái luôn thấy được (đã lưu / đang lưu / lỗi).
4. **Vui nhưng tắt được**: hiệu ứng trò chơi (âm thanh, bảng xếp hạng) là tuỳ chọn, mặc định vừa phải.
5. **Số liệu dẫn đường**: mọi chế độ đều kết thúc bằng màn tổng kết có hành động tiếp theo ("Ôn lại câu sai", "Học tiếp").

## 5. Lộ trình gợi ý

| Giai đoạn | Nội dung | Lý do |
|---|---|---|
| **A (1–2 tuần)** | Bản mẫu 4 màn (trang bộ đề, bắt đầu chế độ, làm bài, vào live) → bạn duyệt; bộ component + token dùng chung; mục 1–4 | Giá trị người dùng thấy rõ nhất, ít rủi ro |
| **B (2–3 tuần)** | Khung làm bài thống nhất + kế hoạch hiệu năng quiz (Phase 1–2); chế độ tập trung & trợ năng (mục 7) | Hết "chậm", dễ dùng hơn |
| **C (2–3 tuần)** | Báo cáo giáo viên (mục 6), cách giao bài (mục 5), rate limit | Thu hút lớp học |
| **D (sau)** | Nhập/AI tạo bộ, thư viện cộng đồng, ngoại tuyến, trang chủ theo vai trò | Mở rộng |

## 6. Câu hỏi cần bạn xác nhận

1. Đối tượng chính: **học sinh tự học** (giống Quizlet) hay **giáo viên chạy lớp** (giống Wayground), hay luyện **TOEIC**? Quyết định thứ tự giai đoạn.
2. Bạn thích những điểm cụ thể nào ở hai trang (gửi ảnh chụp nếu được)?
3. Muốn mô hình miễn phí/trả phí thế nào cho phần mở rộng?
4. Làm bản mẫu bằng HTML tĩnh để duyệt trước khi viết vào React được không?

## Nguồn

- [Quizlet – trang chủ](https://quizlet.com)
- [Quizlet review (Headway)](https://makeheadway.com/blog/quizlet-review/)
- [Quizizz becomes Wayground (PR Newswire)](https://www.prnewswire.com/news-releases/quizizz-becomes-wayground-announces-new-ai-and-curriculum-supports-302489367.html)
- [Wayground (Quizizz) for Teachers (Educators Technology)](https://www.educatorstechnology.com/2022/10/what-is-quizizz-and-how-to-use-it-with.html)
- [Wayground/Quizizz trên Capterra](https://capterra.com/p/207437/Quizizz/)
