# Vai trò lớp học và bài tập về nhà

## Cài đặt và quyền

Phân quyền nằm trong **Cài đặt lớp > Vai trò & phân quyền**. Quản lý tiêu đề,
chủ đề và người xem video nằm trong **Cài đặt lớp > Quản lý video**.

- Role có sẵn: Chủ lớp, Giáo viên, Trợ giảng, Quản lý nội dung, Học viên, Người xem.
- Role hệ thống chỉ đọc; có thể nhân bản để tạo role riêng cho từng lớp.
- Role tùy chỉnh chọn Cho phép, Từ chối hoặc Không cấp cho từng action.
- Có thể gán nhiều role cho một thành viên. Từ chối thắng Cho phép.
- Role được gán tường minh thay thế teacher/student cũ, không cộng quyền cũ.
  Thành viên chưa được gán tường minh tiếp tục dùng role cũ.
- Gán danh sách rỗng thu hồi toàn bộ quyền thao tác trong lớp. Rời lớp/xóa thành viên
  xóa luôn bản gán; tham gia lại không lấy lại role tùy chỉnh cũ.
- Chủ lớp luôn toàn quyền và không thể bị gán role khác. Không tự gán role cho mình.
- Không xóa role đang được gán. Thu hồi khỏi thành viên trước.
- Cần class-access:Manage để quản trị role và gán role; quyền này có thể cấp quyền
  cho người khác nên chỉ gán cho người quản trị được tin cậy.
- API teacher/student cũ vẫn hoạt động với class-member:UpdateRole, nhưng không cho
  người gọi cấp nhiều quyền hơn quyền đang có. Thay đổi qua API cũ xóa bản gán riêng.
- API kiểm tra lại quyền mỗi request, gồm từng request HLS. Thu hồi quyền áp dụng
  cho request tiếp theo; segment đã tải vào trình duyệt không thể bị thu hồi.
- Quyền xem một video cụ thể vẫn được giới hạn bởi audience/viewerIds. Chỉ chủ lớp
  và người có class-video:ManageAudience được bỏ qua audience này.

GET `/v1/classes/{id}/access/me`: quyền có hiệu lực của người gọi.
GET `/v1/classes/{id}/access`: catalog action, role và bản gán thành viên.
POST `/v1/classes/{id}/access/roles`: tạo role.
PUT/DELETE `/v1/classes/{id}/access/roles/{key}`: sửa/xóa role tùy chỉnh.
PUT `/v1/classes/{id}/access/members/{userId}`: `{ "roleKeys": ["assistant"] }`.
PUT `/v1/classes/{id}/videos/{videoId}/audience`: sửa audience riêng biệt.
PUT `/v1/classes/{id}/documents/{documentId}`: đổi tên file, giữ phần mở rộng.
GET `/v1/classes/{id}/permissions` tiếp tục cung cấp quyền để Quiz kiểm tra live:Host.

Đây là RBAC trong phạm vi từng lớp, không thay thế IAM cấp nền tảng/tổ chức.

## Bài tập

Tab **Bài tập** cho phép nhập tiêu đề, hướng dẫn, hạn làm bài tùy chọn, lưu nháp
và giao bài cho cả lớp. Có tìm kiếm, lọc trạng thái và đánh dấu bài đã qua hạn.
Thời gian nhập theo múi giờ trình duyệt, gửi/lưu bằng timestamp có múi giờ.

File hỗ trợ PDF, Word (.doc/.docx), Excel (.xls/.xlsx), PowerPoint (.ppt/.pptx).
Tối đa 5 file, mỗi file 20 MiB, tổng 40 MiB/bài. Kiểm tra phần mở rộng và chữ ký
đầu file; không phải quét malware hoặc kiểm chứng toàn bộ định dạng Office.
File được lưu riêng tư trong PostgreSQL như tài liệu lớp hiện tại, chỉ tải qua
API có xác thực; không tạo URL công khai. Lưu bài, thêm/xóa file là một giao dịch.

- Quyền riêng: Read, Create, Update, Delete, Publish.
- Học viên chỉ thấy bài đã giao và file của các bài này.
- Người có Create xem được bài mình tạo; Update/Publish/Delete xem được bài nháp
  để quản lý. Sửa bài đã giao hoặc thay đổi trạng thái cần Publish cùng với Update.
- Bỏ chọn file cũ khi sửa sẽ xóa file đó lúc bấm lưu. Xóa bài xóa luôn file đính kèm.
- Chưa có nộp bài, chấm điểm, nhận xét hay thông báo tự động.

GET/POST `/v1/classes/{id}/assignments`: danh sách/tạo bài.
PUT/DELETE `/v1/classes/{id}/assignments/{assignmentId}`: sửa/xóa bài.
GET `/v1/classes/{id}/assignments/{assignmentId}/files/{fileId}`: tải file riêng tư.
POST/PUT nhận multipart: trường `metadata` là JSON `{title,instructions,dueAt,status,deleteFileIds}`;
trường `files` có thể lặp lại. Không cần đính kèm file để tạo bài.

## Triển khai

Migration `022_class_roles_assignments.up.sql` chạy qua migration runner hiện có.
Triển khai Class trước frontend để endpoint mới sẵn sàng. Không có thay đổi môi
trường hoặc cấu hình bốn node video. Cấu hình chroot/khóa dịch vụ vẫn giữ nguyên.
Không chạy thay đổi trên production chỉ để kiểm thử giao diện; preview dùng dữ liệu giả.

## Kiểm thử

- `go test ./services/class/...`: kiểm thử service và HTTP, gồm deny ưu tiên,
  role tường minh thay thế teacher cũ, bảo vệ chủ lớp, file riêng tư và rollback.
- Đặt `CLASS_TEST_DATABASE_URL` trỏ PostgreSQL kiểm thử rồi chạy
  `go test ./services/class/internal/http -run TestClassWorkspacePostgres -v -count=1`.
  Test tạo/xóa schema riêng, chạy migration hai lần, tạo/sửa/gán/xóa role, kiểm tra
  đọc video không lộ viewerIds, sửa tiêu đề không đổi audience, bài nháp/đã giao,
  file khác lớp và cascade khi xóa bài/thành viên.
- Frontend: `npm run build`, `npm test`; đã kiểm tra thao tác trên preview dữ liệu
  giả với desktop 1440px và mobile 390px, cả giao diện sáng/tối.
