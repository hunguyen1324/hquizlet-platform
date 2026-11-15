# Công cụ lấy phụ đề YouTube

Tool chỉ tải phụ đề gốc/tự động, không tải media. Dùng yt-dlp 2026.10.26 đã ghim phiên bản. Không cần API key. Không sử dụng cookie tài khoản hay giải challenge. Nếu nhà cung cấp từ chối truy cập, trả thông báo lỗi và giữ luồng nhập SRT/VTT.

## Windows

Cần Python 3.10+ và Go theo go.work. Trong thư mục hquizlet-platform:

```powershell
./tools/captions/Get-VideoSubtitles.ps1 -Install -Video 'https://www.youtube.com/watch?v=J2cmxVjMZtc' -Language ja -Output ./tmp/lesson.json -Srt ./tmp/lesson.ja.srt
```

`-Install` tạo môi trường Python riêng dưới tmp; lần sau bỏ tham số này. `-Provider auto` thử nguồn trực tiếp rồi yt-dlp; `-Provider yt-dlp` chỉ thử extractor; `-Provider direct` dùng để chẩn đoán nguồn cũ. `-Translation vi` yêu cầu bản dịch nếu YouTube cung cấp, không đảm bảo có. JSON chứa `availableTracks`, `original`, `translation`, `message`. SRT là phụ đề nguyên ngữ. File output được ghi đè khi chỉ định cùng đường dẫn. Có thể nhập SRT vào video trong lớp ngay mà chưa cần triển khai backend mới.

## Go CLI / Linux

```sh
go build -o ./tmp/caption-tool ./services/class/cmd/caption-tool
./tmp/caption-tool --video J2cmxVjMZtc --provider yt-dlp --yt-dlp /path/to/yt-dlp --language ja --translation vi --output ./tmp/lesson.json --srt ./tmp/lesson.ja.srt
```

Mã thoát: 0 lấy được nguyên ngữ (bản dịch có thể thiếu), 1 không lấy được/phát sinh lỗi ghi file, 2 tham số sai. JSON vẫn có danh sách track nếu extractor đọc được metadata nhưng tải phụ đề thất bại. Không đưa URL ký hoặc log thô vào JSON. Dùng `--translation=` để bỏ bản dịch.

## Tích hợp web

Class service tự thử yt-dlp khi nguồn trực tiếp thất bại nếu đặt `YOUTUBE_YTDLP_BIN` tới executable. Dockerfile chung chỉ cài Python/yt-dlp cho class và đóng gói `/app/caption-tool`; biến trên đã được đặt trong image. Build/triển khai lại class image; không cần migration hoặc sửa API/web. Có thể tắt fallback bằng cách đặt biến rỗng. Quyền lớp và audience hiện tại vẫn được kiểm tra trước khi gọi tool.

Giới hạn: tối đa 2 yêu cầu đồng thời, tổng timeout 27 giây (nguồn trực tiếp 6 giây, tool tối đa 20 giây), cache kết quả 10 phút/lỗi 1 phút. Metadata tối đa 8 MiB, log 64 KiB; mỗi file phụ đề đọc tối đa 3 MiB, được kiểm tra lại 10.000 câu/1 MiB nội dung. File tạm nằm trong thư mục riêng và xóa sau mỗi lần chạy. Không chạy shell; chỉ nhận ID YouTube hợp lệ và mã ngôn ngữ. Timeout không đủ hoặc YouTube thay đổi có thể cần cập nhật bản yt-dlp đã ghim rồi kiểm tra lại.

## Xác minh

Video J2cmxVjMZtc: tool đã lấy được 191 câu tiếng Nhật và xuất SRT trên máy kiểm tra. Chưa lấy được bản dịch Việt trong lần kiểm tra này. Không coi track tồn tại là đã tải được nội dung. Unit tests kiểm tra fallback/cache, chọn track thủ công, partial translation, metadata sai, giới hạn output và dọn file tạm. Các kiểm tra này không đảm bảo mọi video/IP server đều lấy được phụ đề.

Tài liệu chính thức yt-dlp: https://github.com/yt-dlp/yt-dlp#subtitle-options
