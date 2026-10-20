# Kế hoạch: Thiết kế lại giao diện Quiz + Tối ưu tốc độ tải

> Phạm vi: màn làm bài quiz của `hquizlet-platform` (React/Vite + service `study` viết bằng Go).
> Trạng thái: **kế hoạch, chưa sửa code**. Mọi nhận định dưới đây đến từ việc đọc code ở commit `18686a7`;
> **chưa đo trên hệ thống chạy thật và chưa xem giao diện render** – nên Phase 0 (đo) là bước bắt buộc trước khi tối ưu.

---

## 0. Tóm tắt nhanh

**Tốc độ.** Chậm không chủ yếu do "file nặng" ở frontend. Hai nguyên nhân lớn nhất nằm ở backend:

1. **Mỗi lần bấm chọn đáp án / sang câu / prefetch, server đọc – giải mã – ghi lại toàn bộ trạng thái phiên** (một cục JSONB chứa *tất cả* câu hỏi + đáp án + giải thích + đáp án của người dùng). Bài càng dài (TOEIC 200 câu) càng chậm.
2. **File nghe được lưu trong Postgres (cột `body`, tối đa 30 MB)** và mỗi lần cache miss phải đọc *nguyên file* vào RAM rồi mới phát.

Đề xuất của bạn (**nạp vào RAM + chia nhỏ file**) là đúng hướng, nhưng cần áp dụng **ở server, theo cách khác nhau cho "câu hỏi" và "âm thanh"** (xem mục 3). Nạp *toàn bộ* câu hỏi vào RAM của trình duyệt thì xung đột với yêu cầu chống copy / không lộ đáp án.

**Giao diện.** Hiện có 2 giao diện khác hẳn nhau (mặc định `qz-*` và TOEIC dùng Tailwind), nhiều thứ quan trọng bị giấu hoặc không được lưu (ví dụ cờ đánh dấu câu mất khi tải lại). Đề xuất thống nhất một khung, ưu tiên mobile, có phản hồi tức thì khi bấm (mục 5).

---

## 1. Chẩn đoán hiệu năng (theo mức ảnh hưởng)

| # | Vấn đề | Vị trí | Ảnh hưởng | Mức |
|---|--------|--------|-----------|-----|
| B1 | `Update()` khoá dòng (`FOR UPDATE`), đọc cả `state`, `json.Marshal` **2 lần** (snapshot "before" + sau khi sửa), so sánh byte, rồi `UPDATE` cả JSONB. Làm cho cả `page`, `peek`, `answer`. | `services/study/internal/repository/quiz_session.go` (`Update`) | Độ trễ mỗi thao tác tỉ lệ với kích thước bài; Postgres TOAST đọc/ghi lại MB dữ liệu | **Cao** |
| B2 | `sessionView()` dựng lại **manifest toàn bộ câu** và duyệt tất cả câu để chấm điểm mỗi lần; response nào cũng kèm manifest + answers + results | `service/protected_quiz.go` (`sessionView`) | CPU + băng thông lặp lại vô ích (manifest không đổi) | Trung bình–Cao |
| B3 | `peek` (prefetch) cũng đi qua `Update` có khoá dòng; prefetch lại *ghi* (đánh dấu `Seen`) | `Act()` | Prefetch tranh khoá với thao tác thật của người học | Trung bình |
| B4 | Rate-limit bằng `INSERT … ON CONFLICT` vào Postgres **trên mỗi request** (`Allow`) | `repository/quiz_session.go` (`AllowN`) | 1 lượt ghi DB thừa mỗi lần bấm; audio thêm 1 lượt nữa | Trung bình |
| B5 | `safeQuizHTML()` (sanitize) chạy lại mỗi lần trả trang | `sessionView` | CPU lặp lại; có thể làm 1 lần khi tạo bài | Thấp–TB |
| B6 | `Start`/`Summary` đọc **toàn bộ câu hỏi** từ DB; mỗi phiên lại **sao chép toàn bộ câu hỏi** vào state (tối đa 5 phiên/bài/người) | `Start`, `Summary` | Mở màn bắt đầu chậm; DB phình; mọi phiên đều nặng | Trung bình |
| A1 | Âm thanh lưu dạng BYTEA trong Postgres; `Open()` đọc cả file (≤30 MB) vào RAM, kể cả khi trình duyệt chỉ xin `Range: bytes=0-0` (bước prefetch) | `storage/quiz_audio.go` (`Open`) | Lần nghe đầu của mỗi file rất chậm; áp lực RAM/DB | **Cao** |
| A2 | Cache âm thanh chỉ 64 MB/replica, trong tiến trình; restart là mất | `DefaultAudioCacheBytes` | Bài nghe dài nhanh bị đẩy khỏi cache | Trung bình |
| A3 | Mỗi request Range gọi lại `Audio()` → kiểm quyền + `View()` đọc cả state phiên (nếu không trúng `okAudio`) | `Audio()` | Phát audio/tua gây nhiều lần đọc JSONB lớn | Trung bình |
| F1 | Chế độ "đáp án ngay": UI **chờ server trả** mới hiện lựa chọn (`shownAnswers = view.answers` khi `instant`) → bấm xong cảm giác "đơ" khi B1 chậm | `ProtectedQuizPlayer.tsx`, `useProtectedQuiz.ts` | Cảm giác chậm khi chuyển câu | Trung bình |
| F2 | Prefetch chỉ **1 trang phía trước**, chờ 250 ms, và **bị huỷ khi `busy`**; lần đầu vào câu chưa có trong cache phải chờ mạng | `useProtectedQuiz.ts` | Nhảy xa (qua bảng câu hỏi) luôn chờ | Trung bình |
| F3 | Chưa thấy code-split riêng cho player quiz (chỉ admin/dashboard dùng `React.lazy`) | `apps/web/src/main.tsx` | Tải lần đầu nặng hơn cần thiết | Thấp–TB (cần đo) |

Đã làm tốt (không cần đụng): cache trang đã xem trong RAM trình duyệt, ghi đáp án theo hàng đợi nền, prefetch audio ticket, memo kiểm quyền (`WithAccessCache`), `View()` không khoá cho audio, gzip ở nginx, asset có hash + `immutable`.

---

## 2. Phase 0 – Đo trước khi sửa (≈ 0,5 ngày)

Không tối ưu theo cảm tính. Việc cần làm:

1. **Kích thước phiên** (chạy trên DB thật):
   ```sql
   SELECT id, study_set_id, pg_column_size(state) AS bytes
   FROM protected_quiz_sessions ORDER BY bytes DESC LIMIT 20;
   ```
2. **Kích thước audio**:
   ```sql
   SELECT study_set_id, count(*) AS n, pg_size_pretty(sum(length(body))) AS total,
          pg_size_pretty(max(length(body))) AS biggest
   FROM protected_quiz_audio GROUP BY 1 ORDER BY sum(length(body)) DESC LIMIT 20;
   ```
3. **Thêm `Server-Timing` + log thời gian** cho 4 endpoint: `GET /play/{id}` (page), `/peek`, `POST /answer`, `/audio-ticket` và stream audio. Tách: thời gian `access`, `Allow`, `Update` (đọc/ghi DB), `sessionView`, JSON encode.
4. **Chrome DevTools → Network** khi làm 1 bài TOEIC thật: ghi p50/p95 của page/answer, TTFB audio, kích thước response.
5. **Lighthouse + `vite build` bundle report** (rollup-plugin-visualizer) cho trang quiz.
6. Lưu kết quả vào `docs/perf/quiz-baseline.md` để so sánh trước/sau.

**Hoàn thành khi:** có bảng số liệu; xác nhận B1/A1 có đúng là nút cổ chai (nếu `state` chỉ vài chục KB thì B1 kém quan trọng hơn dự đoán).

---

## 3. Đánh giá đề xuất "nạp vào RAM + chia nhỏ file"

| Ý tưởng | Nhận xét | Cách áp dụng khuyến nghị |
|---|---|---|
| **Nạp vào RAM – phía server** | **Nên làm.** Câu hỏi của một bài là dữ liệu *bất biến* (khi đã chốt), rất hợp để cache. | Tách "bản chụp câu hỏi" (bất biến, cache RAM/LRU, dùng chung cho mọi phiên) khỏi "trạng thái phiên" (nhỏ, thay đổi). Xem Task P2. |
| **Nạp vào RAM – phía trình duyệt, toàn bộ bài** | **Không nên.** Phải gửi đáp án đúng/giải thích xuống client → dễ copy, lộ đáp án, trái yêu cầu chống sao chép/F12. | Chỉ giữ **cửa sổ trượt** (vài trang tới/lui, *không kèm đáp án đúng*). Đáp án chỉ trả sau khi trả lời (đã là cách hiện tại). |
| **Chia nhỏ file – câu hỏi** | Đúng hướng: tách theo "trang" (1 câu hoặc 1 nhóm TOEIC) để phục vụ trang `i` tốn O(trang) thay vì O(cả bài). | Tiền xử lý một lần khi tạo bản chụp: mỗi trang là một đoạn đã sanitize sẵn. |
| **Chia nhỏ file – âm thanh** | Có lợi nếu file nghe là file **dài** (cả Part/cả đề). Nếu đã là từng file ngắn theo câu thì lợi ích chính là *đổi nơi lưu + giảm dung lượng*, không phải chia. | (1) chuyển khỏi Postgres sang MinIO/ổ đĩa, phát bằng Range; (2) nén lại; (3) nếu file dài thì cắt theo nhóm câu khi import. Xem Task P3. |

> Cần xác nhận ở Phase 0: file nghe hiện tại **dài hay ngắn** (truy vấn số 2). Kết quả sẽ quyết định có cần cắt file hay chỉ cần đổi nơi lưu + nén.

---

## 4. Kế hoạch tối ưu theo giai đoạn

Mỗi task = 1 PR riêng, có test hoặc số đo trước/sau. **Không cache** nội dung quiz bảo vệ ở CDN/trình duyệt (`Cache-Control: private, no-store` giữ nguyên). Cache RAM trong tiến trình server là được vì không ra ngoài.

### Phase 1 – Thắng nhanh (≈ 1–2 ngày, ít rủi ro)

**P1.1 – Rate-limit không ghi DB mỗi request (B4)**
- Chuyển bucket `play`/`peek`/`audio` sang token-bucket trong Redis (đã có sẵn trong stack) hoặc bộ nhớ trong tiến trình; giữ DB cho `start` (20/giờ).
- *Xong khi:* một lượt `answer` không còn `INSERT/UPDATE` vào `protected_quiz_limits`.

**P1.2 – `peek`/`page` chỉ-đọc, không khoá, không ghi lại cả state (B3)**
- `peek` dùng đường `View()` (không `FOR UPDATE`, không UPDATE). Việc đánh dấu `Seen` chuyển sang cột/mảng riêng nhỏ (xem P2) hoặc làm khi trang thật sự được mở.
- *Xong khi:* peek không giữ khoá dòng; p95 `answer` không tăng khi prefetch chạy song song.

**P1.3 – Sanitize một lần (B5)**
- Chạy `safeQuizHTML` khi tạo phiên/bản chụp, lưu kết quả; bỏ gọi lại trong `sessionView`.

**P1.4 – UI phản hồi tức thì (F1)**
- Chế độ "đáp án ngay": hiện ngay trạng thái "đang chấm…" trên lựa chọn đã bấm (không chờ response), khoá các lựa chọn khác; khi server trả mới hiện đúng/sai.
- Prefetch **3 trang tới + 1 trang lui**, bỏ điều kiện huỷ khi `busy` (xếp hàng ưu tiên thấp), khi mở bảng câu hỏi thì prefetch đích được hover/giữ.

**P1.5 – Âm thanh: nén lại khi upload (A1, giảm 3–5× dung lượng)**
- Khi `Put`/import: chuyển sang mono, 64–96 kbps (MP3 hoặc Opus) bằng `ffmpeg` cho nội dung lời nói. Giữ file gốc trong MinIO nếu cần.
- *Xong khi:* dung lượng audio trung bình giảm rõ rệt, nghe thử không suy giảm chất lượng đáng kể.

### Phase 2 – Tách "bản chụp câu hỏi" và "trạng thái phiên" (≈ 3–5 ngày, thay đổi lớn nhất)

**Thiết kế**

```
quiz_snapshots            (bất biến, dùng chung nhiều phiên)
  id (hash nội dung + part), study_set_id, pages[]  -- mỗi trang đã sanitize, đã gom nhóm TOEIC
  meta: tổng câu, đếm theo Part, manifest (number, part, group)

protected_quiz_sessions   (nhỏ, thay đổi thường xuyên)
  id, user_id, study_set_id, snapshot_id, mode, layout, instant,
  started_at, deadline, submitted_at, current_index,
  seen[]  (hoặc bitset), answers{idx: text}  -- vài KB, không chứa câu hỏi
  sort_seed  -- để xáo thứ tự "sorting" lặp lại được, không lưu cả bản sao câu hỏi
```

- Máy chủ giữ **LRU trong RAM** cho snapshot (giới hạn theo byte, key = `snapshot_id`); trúng cache thì phục vụ trang `i` bằng cắt đúng 1 đoạn. Snapshot bất biến nên an toàn khi nhiều replica mỗi nơi cache riêng.
- `answer`: `UPDATE … SET answers = jsonb_set(answers, '{idx}', …)` trên dòng nhỏ (hoặc bảng `quiz_session_answers` upsert từng câu) – không còn đọc/ghi cả bài.
- Chấm điểm khi nộp bài, và khi chế độ "đáp án ngay" chỉ so đúng **câu vừa trả lời** (đáp án đúng nằm trong snapshot trên RAM server).
- Nếu nội dung quiz bị sửa sau khi phiên bắt đầu: phiên cũ vẫn trỏ snapshot cũ (giữ nhất quán); snapshot mới tạo cho phiên mới. Dọn snapshot không còn phiên nào tham chiếu.
- **Summary** lấy đếm từ metadata snapshot, không đọc toàn bộ câu hỏi (B6).

**Thu gọn response (B2)**
- `GET/start` trả manifest **một lần**. `page/peek/answer` chỉ trả: trang yêu cầu, kết quả của câu liên quan, `serverTime`, và (khi cần) điểm. Thêm phiên bản API (`/play` v2 hoặc `?compact=1`) để không vỡ client cũ.

**Di trú:** viết migration chuyển phiên đang chạy (≤ 24 giờ) sang dạng mới, hoặc cho hai dạng cùng tồn tại tới khi phiên cũ hết hạn (24 giờ) rồi xoá code cũ.

**Xong khi:** p95 `answer` và `page` < **150 ms** (server) với bài 200 câu; kích thước `state` mỗi phiên ≤ vài KB; test `protected_quiz_integration_test.go` vẫn pass, bổ sung test: không lộ đáp án trước khi trả lời, không đọc được trang chưa "seen" ngoài quy tắc hiện hành.

### Phase 3 – Âm thanh (≈ 2–4 ngày)

**P3.1 – Bỏ BYTEA khỏi đường nóng (A1, A2)**
- Nguồn sự thật: MinIO (đã có bản sao `quiz-audio/{id}`); Postgres chỉ giữ metadata (id, mime, kích thước, `object_key`).
- `Open()` trả `io.ReadSeeker` đọc theo **Range** từ MinIO (hoặc từ **bản cache trên ổ đĩa** của app server: tải một lần → phục vụ bằng `http.ServeContent` trên `os.File`, hệ điều hành tự cache RAM). Không còn `io.ReadAll` 30 MB.
- `Range: bytes=0-0` (bước "làm nóng") chỉ kiểm tra tồn tại + đặt cache, không đọc cả file.

**P3.2 – Cắt file nghe dài (chỉ nếu Phase 0 cho thấy file dài)**
- Khi import: cắt theo nhóm câu/Part (ffmpeg với mốc thời gian, hoặc phát hiện khoảng lặng), lưu mỗi đoạn như một audio riêng gắn vào đúng trang. Mỗi trang chỉ tải đoạn của mình.
- Phương án nâng cao: HLS (đã có `hls.js` trong `package.json`) với segment URL ký ngắn hạn. Lưu ý trung thực: không có cách nào chặn tuyệt đối việc ghi lại âm thanh phát ra; mục tiêu là **không lộ một file hoàn chỉnh dễ tải** và giữ nguyên cơ chế ticket + kiểm quyền mỗi Range.

**P3.3 – Giảm chi phí kiểm quyền mỗi Range (A3)**
- Với snapshot (Phase 2), việc "câu này đã `seen` chưa và audio là nguồn nào" tra từ RAM + bitset nhỏ, không đọc state lớn. Giữ memo `okAudio` TTL ngắn.

**Xong khi:** TTFB audio lần đầu < **1 s** (mạng nội bộ), không còn truy vấn `SELECT body` trong đường stream, RAM tiến trình không tăng theo kích thước file.

### Phase 4 – Frontend (≈ 1–2 ngày)

- **Code-split** `ProtectedQuizPlayer` + `exam/*` bằng `React.lazy`; kiểm tra bundle (F3), tách `recharts` khỏi chunk quiz.
- **Skeleton** cho trang đang tải (thay vì màn trắng/`Đang chuẩn bị…`), và nhận biết nhảy xa: hiển thị skeleton ngay rồi thay nội dung.
- **Ưu tiên tải:** audio của trang kế được xin ticket + làm nóng ngay khi trang hiện tại mở (hiện chỉ làm sau 250 ms và khi không busy).
- **Offline/mạng chập chờn:** đáp án chưa gửi được hiển thị trạng thái "chưa lưu – thử lại" thay vì chỉ banner lỗi chung.
- Lưu **cờ đánh dấu câu** (`flags`) vào server/phiên (hiện chỉ là `useState`, mất khi tải lại).

### Mục tiêu đo (đề xuất)

| Chỉ số | Hiện tại | Mục tiêu |
|---|---|---|
| p95 `answer` / `page` (server) | *đo ở Phase 0* | < 150 ms |
| Chuyển câu đã prefetch | tức thì | tức thì (giữ) |
| Chuyển câu chưa prefetch | *đo* | < 300 ms cảm nhận (có skeleton) |
| TTFB audio lần đầu | *đo* | < 1 s |
| Kích thước `state` mỗi phiên | *đo* | ≤ vài KB |
| Kích thước response `answer` | *đo* | giảm ≥ 80% (bỏ manifest) |

---

## 5. Thiết kế lại giao diện quiz

### 5.1 Những điểm chưa thân thiện (rút ra từ code)

1. **Hai giao diện tách rời:** kiểu "Thường" (CSS `qz-*`) và kiểu TOEIC (thành phần `exam/*` dùng Tailwind, thanh "← Lưu và thoát" nền xanh đậm riêng) trông và hành xử khác nhau.
2. **Tuỳ chọn bị giấu** trong `<details>` "Tuỳ chọn" (giao diện, Part, xem đáp án, thời gian). Người dùng mới không biết có; còn "Giao diện: Thường / TOEIC (IIG)" là khái niệm kỹ thuật.
3. **Chế độ đáp án ngay chặn sang câu** cho tới khi trả lời (`canAdvance`), nhưng không giải thích vì sao nút bị vô hiệu.
4. **Phản hồi chậm** khi bấm đáp án (F1) – người dùng không biết đã bấm được chưa.
5. **Cờ đánh dấu mất khi tải lại** (chỉ ở bộ nhớ React).
6. **Không có dấu hiệu "đã lưu"**, trong khi thanh `qz-loadbar` chỉ hiện khi đang gửi.
7. **Màn kết quả** chưa có "làm lại các câu sai", phân tích theo Part, hay gợi ý bước tiếp.
8. Điều hướng câu hỏi nằm trong một hộp thoại riêng; không thấy trạng thái từng câu ngay trên thanh tiến độ.

> Chưa xem giao diện thực tế đã render. Nếu bạn gửi ảnh chụp những màn "không ổn" nhất, mình sẽ ưu tiên đúng chỗ đó.

### 5.2 Nguyên tắc thiết kế

- **Một khung duy nhất** cho mọi bài (thường + TOEIC): cùng thanh trên, cùng thanh dưới, cùng bảng điều hướng; TOEIC chỉ khác ở *bố cục nội dung* (2 cột).
- **Mobile trước:** vùng bấm ≥ 44 px, nút hành động chính cố định ở đáy trong tầm ngón cái.
- **Mỗi màn một việc chính**, thông tin phụ gom vào bảng trượt (bottom sheet).
- **Phản hồi tức thì** cho mọi lần bấm; trạng thái luôn nhìn thấy được (đã lưu / đang lưu / lỗi).
- **Giữ lại những gì đang tốt:** phím tắt (1–4, A–D, ←/→, F), token màu theo theme sáng/tối, hỗ trợ `prefers-reduced-motion`.

### 5.3 Bố cục đề xuất

**Màn bắt đầu** – hiện tuỳ chọn ngay, không giấu:

```
┌──────────────────────────────────────────────┐
│ ← Tên bộ đề                         ⚙ Phiên  │
│ 200 câu · ~120 phút                          │
│                                              │
│ ┌──────────────────────────────────────────┐ │
│ │ ▶ Làm tiếp · 87/200 · còn 52:10          │ │   ← thẻ nổi bật, chỉ khi có phiên dở
│ │ ▓▓▓▓▓▓▓▓░░░░░░░                Tiếp tục  │ │
│ └──────────────────────────────────────────┘ │
│                                              │
│ Chọn cách làm                                │
│ ┌──────────────┐  ┌──────────────┐           │
│ │ Luyện tập    │  │ Thi thử      │           │   ← thẻ chọn (radio), có mô tả 1 dòng
│ │ xem đáp án   │  │ tính giờ,    │           │
│ │ ngay         │  │ nộp mới biết │           │
│ └──────────────┘  └──────────────┘           │
│                                              │
│ Phạm vi   [ Tất cả ] [P1·6] [P2·25] [P3·39]… │   ← chip, hiện khi bộ đề có Part
│ Thời gian [ 30′ ][ 60′ ][ 120′ ][ Tuỳ chỉnh ]│   ← mặc định gợi ý theo số câu
│                                              │
│ [        Bắt đầu làm bài        ]            │   ← nút chính cố định đáy
└──────────────────────────────────────────────┘
```
- "Thường / TOEIC" bỏ khỏi người dùng: **tự chọn** bố cục theo bộ đề (có tag Part → TOEIC); để chế độ nâng cao nếu cần ghi đè.

**Màn làm bài (điện thoại)**

```
┌──────────────────────────────────────┐
│ ✕   12 / 50   ▓▓▓░░░░░░   ⏱ 41:08  ⚑ │  ← thoát, tiến độ, đồng hồ, đánh dấu
│ ● ● ◐ ○ ○ ○ ○ …                      │  ← (tuỳ chọn) chấm trạng thái từng câu, cuộn ngang
├──────────────────────────────────────┤
│ [ ▶ ──●──────── 0:12/0:48  1x ]      │  ← trình phát nghe cố định (nếu có)
│                                      │
│ Câu hỏi (cỡ chữ lớn, tuỳ chỉnh được) │
│                                      │
│ ┌────────────────────────────────┐   │
│ │ A   Đáp án A                   │   │  ← thẻ lớn ≥ 56px
│ └────────────────────────────────┘   │
│ ┌────────────────────────────────┐   │
│ │ B   Đáp án B             ✓ chọn│   │
│ └────────────────────────────────┘   │
│ …                                    │
├──────────────────────────────────────┤
│ ☰ Bảng câu   [ ‹ Trước ] [ Tiếp › ]  │  ← thanh đáy cố định
└──────────────────────────────────────┘
```
- **Luyện tập – đáp án ngay:** sau khi chọn, một **bottom sheet** trượt lên với "Chính xác / Chưa đúng", đáp án đúng, giải thích; nút chính đổi thành **"Câu tiếp theo"**. Nút bị khoá thì có chú thích "Chọn một đáp án để tiếp tục".
- **Thi thử:** không phản hồi đúng/sai; chọn xong có dấu "đã lưu" nhỏ; nút "Nộp bài" chỉ ở bảng câu + câu cuối.
- **Bảng câu (bottom sheet):** lưới số câu, màu + biểu tượng (không chỉ màu): đã làm, chưa làm, đã đánh dấu, hiện tại; sau khi nộp thêm đúng/sai. Có lọc nhanh "Chưa làm / Đã đánh dấu".

**TOEIC / có đoạn văn hoặc ảnh – màn rộng:** 2 cột – trái: ảnh/đoạn văn/trình phát (cuộn riêng); phải: danh sách câu của nhóm. Màn hẹp: xếp dọc, đoạn văn thu gọn được.

**Màn kết quả**
- Vòng điểm lớn (đúng/tổng, %), thời gian, so với lần trước.
- Phân tích theo Part (thanh ngang) khi là TOEIC.
- Hành động: **Xem lại câu sai**, **Làm lại các câu sai**, Làm lại toàn bộ, Về bộ đề.

### 5.4 Thành phần & token

- Giữ token hiện có (`--primary`, `--card`, `--border`, `--muted-foreground`; màu đúng/sai/cờ `--qz-ok/bad/flag`), tạo bộ component dùng chung: `QuizShell`, `QuizTopBar`, `QuizBottomBar`, `AnswerOption`, `FeedbackSheet`, `QuestionGrid`, `AudioBar`, `StatusPill` ("Đã lưu / Đang lưu / Lỗi – thử lại").
- Bỏ phụ thuộc Tailwind riêng của `exam/*` trong màn quiz, hoặc ngược lại thống nhất một hệ — chọn một, tránh hai hệ style song song.
- Cỡ chữ câu hỏi có nút A−/A+ (lưu cục bộ).

### 5.5 Trợ năng (WCAG 2.1 AA)

- Tương phản chữ ≥ 4.5:1 (kiểm tra cả theme tối); đúng/sai **không chỉ dựa vào màu** (kèm ✓/✗ + chữ).
- Điều hướng bàn phím đầy đủ, focus rõ, `aria-live` cho "Chính xác/Chưa đúng" và trạng thái lưu.
- Vùng bấm ≥ 44 px; tôn trọng `prefers-reduced-motion` (đã có một phần).
- Thông báo khi đồng hồ còn 5 phút / 1 phút (live region, không chỉ đổi màu).

### 5.6 Nội dung chữ (UX copy)

- Dùng ngôn ngữ người học: "Luyện tập – xem đáp án ngay", "Thi thử – tính giờ như thật".
- Nút bị khoá luôn có lý do ngay cạnh. Lỗi nói rõ cách xử lý ("Chưa lưu được câu 12 – Thử lại").
- Hộp thoại nộp bài giữ nội dung hiện có (đã tốt: nêu số câu chưa làm + nút "Đến câu chưa làm").

### 5.7 Task giao diện

| Task | Nội dung | Ước lượng |
|---|---|---|
| U1 | Bản mẫu (wireframe/Figma hoặc HTML tĩnh) 4 màn: bắt đầu, làm bài mobile, làm bài TOEIC desktop, kết quả → bạn duyệt | 1–2 ngày |
| U2 | Bộ component dùng chung + token (mục 5.4); thay thanh "Lưu và thoát" TOEIC | 2 ngày |
| U3 | Màn bắt đầu mới: chip Part/thời gian, tự chọn layout, thẻ làm tiếp | 1 ngày |
| U4 | Màn làm bài: `AnswerOption`, `FeedbackSheet`, thanh đáy, trạng thái lưu, skeleton | 2–3 ngày |
| U5 | Bảng câu (bottom sheet) + lọc; lưu cờ đánh dấu lên server | 1–2 ngày |
| U6 | Bố cục 2 cột TOEIC dùng chung khung; trình phát nghe cố định | 2 ngày |
| U7 | Màn kết quả mới: phân tích Part, làm lại câu sai | 2 ngày |
| U8 | Rà trợ năng + kiểm thử trên điện thoại thật + cập nhật test (`QuizPlayer.test.tsx`, `quiz.test.tsx`) | 1–2 ngày |

---

## 6. Ràng buộc bảo mật phải giữ nguyên

- Câu hỏi giao **theo trang**; đáp án đúng/giải thích **chỉ** trả sau khi đã trả lời (hoặc đã nộp) và chỉ cho câu đã trả lời.
- Âm thanh: ticket ký ngắn hạn, kiểm quyền + phiên ở **mỗi** Range request; không URL công khai tới MinIO.
- Nội dung bảo vệ: `Cache-Control: private, no-store`; không đưa lên CDN.
- Không đưa token/khoá vào repo; dùng `.env`.
- Rate-limit chuyển sang Redis/RAM vẫn phải giữ hành vi từ chối khi vượt ngưỡng (và fail-closed khi không kiểm tra được).

---

## 7. Thứ tự thực hiện đề xuất

1. **Phase 0** – đo (bắt buộc).
2. **Phase 1** – thắng nhanh (P1.1–P1.5), song song với **U1** (bản mẫu giao diện để bạn duyệt).
3. **Phase 2** – tách snapshot/trạng thái phiên (đóng góp lớn nhất cho tốc độ chuyển câu).
4. **Phase 3** – âm thanh.
5. **U2–U8** – dựng giao diện mới trên API đã nhanh (làm sau Phase 2 để không phải làm lại phần gọi API).
6. **Phase 4** – frontend (code-split, skeleton) cùng lúc với U4.
7. Đo lại, so với `docs/perf/quiz-baseline.md`.

---

## 8. Câu hỏi mở (cần bạn xác nhận)

1. Màn nào "không ổn" nhất với bạn (bắt đầu / làm bài thường / TOEIC / kết quả)? Có ảnh chụp thì mình bám theo.
2. File nghe hiện tại: mỗi câu một file ngắn, hay một file dài cho cả Part/đề? (Quyết định P3.2.)
3. Bài dài nhất bạn dùng bao nhiêu câu? Cần hỗ trợ bao nhiêu người làm bài cùng lúc?
4. Có thể chấp nhận **đổi API** (`/play` v2) không, hay phải giữ tương thích client cũ?
5. Giao diện mới: ưu tiên mobile hay desktop trước?
