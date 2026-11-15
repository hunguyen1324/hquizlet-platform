# Truyện HTML và sửa chữ OCR

Mục **Truyện** (`/manga`) nằm trong menu Công cụ. Luồng chính là HTML Mokuro + thư mục ảnh, không yêu cầu file .mokuro. JSON Mokuro vẫn được hỗ trợ để đọc/sửa OCR, nhưng chỉ tập nhập từ HTML mới xuất lại đúng HTML gốc.

1. Bấm **Chọn HTML**, chọn ví dụ `D:\manga\manga\manga_images2_2.html` (có thể chọn nhiều tập).
2. Bấm **Chọn thư mục ảnh**, chọn `D:\manga\manga` hoặc thư mục `manga_images2`. Giữ đúng tên và cấu trúc ảnh được HTML tham chiếu.
3. Xem lại tên/số trang, bấm **Nhập**. Mặc định riêng tư; chỉ chủ truyện đọc/sửa. Nếu nhập bị ngắt, chọn lại HTML/ảnh: hệ thống bỏ qua các trang đã lên của tập đang nhập. Không nhập đè tập hoàn tất có cùng mã nguồn.
4. Mở truyện, chọn một/hai trang, hướng phải/trái, số trang hoặc thu phóng. Trang đang đọc được nhớ trên trình duyệt.
5. Bấm khung thoại, sửa chữ Nhật hoặc bản dịch Việt, bấm **Lưu chữ OCR**. Ảnh và vị trí khung không đổi. Dữ liệu lưu có revision; phiên cũ không được ghi đè bản sửa mới. Có nút khôi phục OCR gốc của cả trang.
6. Bấm **HTML đã sửa** để tải bản HTML dùng các thay đổi đã lưu. Đặt HTML tải về cạnh thư mục ảnh như ban đầu để mở offline. Không phải file HTML độc lập chứa tất cả ảnh; đường dẫn ảnh tương đối được giữ nguyên. File gốc trên máy không bị sửa tự động.

Cài đặt cho phép đổi tên tập, chia sẻ với người dùng đã đăng nhập hoặc xóa. Người đọc được chia sẻ chỉ đọc/sao chép OCR; chỉ chủ truyện sửa và xóa.

## Dữ liệu và triển khai

Class service cung cấp `/v1/manga` và `/v1/manga/*`; gateway xác thực trước khi chuyển tiếp. Áp dụng migration 024/025 khi service khởi động. Ảnh JPEG/PNG (tối đa 12 MiB mỗi trang) lưu riêng trên SFTP dùng cấu hình VIDEO_SFTP hiện có, với key ảnh tách biệt key video. Metadata, HTML gốc, OCR gốc và bản sửa lưu PostgreSQL. Giữ tối đa 20 bản OCR cũ mỗi trang để phục hồi quản trị; UI khôi phục OCR gốc. Xóa tập tạo hàng đợi dọn ảnh bền vững.

Giới hạn HTML 8 MiB, 1.000 trang/tập, 1.000 khối OCR/trang, 512 KiB text/trang. HTML phải có cấu trúc Mokuro `.page > .pageContainer > .textBox`; HTML tùy ý không được chạy trong origin web. Server chỉ phân tích nội dung và đường dẫn ảnh cục bộ; chặn URL ngoài/traversal. Bản HTML xuất giữ CSS/script gốc, được trả dưới dạng download attachment với sandbox. Text sửa được escape theo HTML, không thực thi thành script trong hệ thống.

Cần triển khai cả class, gateway và Workers Static Assets. Kiểm tra kho ảnh bằng `/app/manga-tool --check-storage` trong môi trường private SFTP, chỉ tạo/đọc/xóa file probe ngẫu nhiên.

CLI hỗ trợ kiểm tra/xuất HTML offline:

```powershell
go run ./services/class/cmd/manga-tool --html D:/manga/manga/manga_images2_2.html --inspect-json ./tmp/parsed.json
# patch.json là mảng [{"pageIndex":0,"blockIndex":0,"lines":["Chữ đã sửa"]}]
go run ./services/class/cmd/manga-tool --html D:/manga/manga/manga_images2_2.html --patch ./tmp/patch.json --output ./tmp/chapter-edited.html
```

## Kiểm tra

Đã kiểm tra file thật manga_images2_2.html (42 trang) qua parse → sửa → export → parse và mở lại bằng Chrome với ảnh gốc: điều khiển chuyển trang, ảnh, script đọc truyện hoạt động. Giao diện mới đã thử desktop 1440px/mobile 390px cho chọn khung, sửa/lưu và tải HTML (API mock). PostgreSQL integration kiểm tra private/shared, owner-only edits, stale revision 409, restore, xuất HTML đã sửa và cleanup outbox. Unit tests kiểm tra ghép ảnh theo đường dẫn tương đối, thiếu/trùng ảnh, giới hạn và escape text.

## Triển khai production 2026-11-15

Đã triển khai `hquizlet-class:manga-html-20261115` và `hquizlet-gateway:manga-html-20261115`; cả hai healthy. Image overrides lưu tại `.image_tag.env` trên App. Backups image/config nằm tại `/opt/hquizlet/releases/manga-html-20261115`, tags `before-manga-20261115`. Worker frontend version `c0cc2abb-7f2c-43d1-a3d9-1170a56e7f7e`. Kiểm tra public `/manga` và `/api/healthz` trả 200, `/api/v1/manga` không token trả 401. Probe SFTP image write/read/delete thành công. Chưa nhập bộ truyện của người dùng vào production; người dùng chọn HTML và thư mục ảnh trong tài khoản của mình.
