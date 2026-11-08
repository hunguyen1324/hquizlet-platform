# HLS phân đoạn trên bốn VPS CloudCode

Ngày triển khai: 08/11/2026. Không chứa thông tin đăng nhập.

## Kiến trúc đã thực hiện

- App giữ xác thực, upload chia phần 8 MiB và chuyển mã FFmpeg. Không chạy
  FFmpeg trên các VPS 256 MiB.
- srv-pah58oxa (remote.io.vn:11065) nhận nguồn upload tạm, sau đó chỉ giữ
  manifest JSON nhỏ của video đã phân tán. Cleanup hiện hữu xóa nguồn sau
  khi chuyển mã thành công.
- Từng segment HLS mã hóa lưu đúng một lần trên một VPS. Luân phiên bốn
  node với điểm bắt đầu thay đổi theo object key; khi publish gặp node lỗi
  hoặc đầy thì chọn node khác và ghi rõ vị trí thực tế vào manifest.
- Playlist và mapping node ID được lưu trong manifest, không nhúng khóa AES.
  Khóa vẫn được suy ra từ VIDEO_HLS_MASTER_KEY và object key cũ.
- Khi phát, App kiểm quyền lớp và ticket như trước, đọc segment từ node được
  ghi trong manifest. Trình phát HLS tải các đoạn liên tiếp qua API hiện tại.
  Không ghép lại toàn bộ video thành file.
- Cache tối đa 128 manifest trong Class giảm số lần đọc từ máy cũ. Khi cache
  miss vẫn cần srv-pah58oxa; đây chưa phải kiến trúc chịu lỗi kho metadata.
- Trình duyệt vẫn đi qua Worker/App; không kết nối trực tiếp bốn IPv6. Cách
  này giảm đĩa và băng thông srv-pah58oxa, không loại bỏ giới hạn đường truyền
  của App/Worker. Không tuyên bố tăng tốc trước khi đo tải thực tế.

## Node đã xác minh

| ID cố định | IPv6 | IP nội bộ | RAM | Đĩa trống ban đầu |
| --- | --- | --- | --- | --- |
| video-1 | 2001:470:24:849::85 | 10.0.0.71 | 256 MiB | khoảng 3,9 GiB |
| video-2 | 2402:800:61ca:921b:be24:11ff:feca:b58c | 10.0.0.115 | 256 MiB | khoảng 3,9 GiB |
| video-3 | 2402:800:61ca:921b:be24:11ff:fe8b:edea | 10.10.10.70 | 256 MiB | khoảng 3,9 GiB |
| video-4 | 2402:800:61ca:921b:be24:11ff:fec7:eca7 | 10.10.10.83 | 256 MiB | khoảng 3,9 GiB |

Mỗi node có một CPU logic. srv-pah58oxa có một CPU logic, khoảng 454 MiB
RAM và 7,4 GiB đĩa trống. Các giá trị là thời điểm khảo sát, không phải quota
cam kết. Mỗi node mới giữ ít nhất 1 GiB dự phòng lúc ghi; ngân sách ban đầu
của bốn node khoảng 11,6 GiB trước overhead. Đây là một bản segment duy nhất,
không chia đôi dung lượng để giữ replica.

## Cấu hình và bảo mật

Bốn VPS dùng hqsegments, SSH port 22, chỉ internal-sftp, không shell/forward,
chroot /srv/hquizlet-hls, đường dẫn SFTP /segments. Dữ liệu nằm tại:
/srv/hquizlet-hls/segments/OBJECT_KEY.hls/segmentNNNNNN.ts

Private key riêng trên App:
/opt/hquizlet/infra/docker/video-secrets/shards-key (mode 0600).

Mapping cấu hình:
/opt/hquizlet/infra/docker/video-secrets/hls-nodes.json
VIDEO_HLS_NODES_FILE=/run/video-secrets/hls-nodes.json

Mẫu cấu hình không chứa secret: infra/docker/hls-nodes.example.json.
NodeConfig là mảng JSON gồm ID và Config (Host, Port, User, Root,
HostFingerprint, PrivateKeyFile, MaxConnections). Shard mặc định chọn
ssh-ed25519 để khớp đúng loại host key đã ghim; có thể đặt Config.HostKeyAlgorithm
rõ ràng nếu node dùng loại khác. Không tự nhận key mới khi fingerprint đổi.

Node ID xuất hiện trong manifest. Không đổi ID để trỏ máy khác hoặc bỏ node
khỏi cấu hình trước khi di chuyển hết segment của node đó.

Mỗi segment tối đa 32 MiB; checksum SHA-256 được kiểm sau truyền lên node
và trước khi trả cho người xem. Tối đa bốn response segment giữ buffer đồng
thời trên App; reader đóng hoặc request bị hủy sẽ giải phóng tài nguyên.
Manifest không publish cho đến khi toàn bộ segment đã ghi và xác minh.
Publish lỗi có thể để lại segment tạm; deletion outbox xóa chúng khi video
bị xóa, bao gồm hls_output_key của các job thất bại.

## Tương thích, chuyển video cũ và rollback

Legacy ZIP vẫn phát được khi đã bật shard config. Công cụ trong image:

```bash
/app/shard-migrate --check /tmp
/app/shard-migrate OBJECT_KEY BACKUP_DIRECTORY
```

Công cụ kiểm tra ZIP, sao lưu bản gốc vào thư mục riêng tư trên App, giữ
nguyên object key để không đổi khóa giải mã, ghi segment và xác minh rồi
thay ZIP bằng manifest bằng POSIX rename nguyên tử. Nếu publish chưa thành
công, ZIP cũ vẫn còn nguyên. Nếu backup cũ đã tồn tại do lần thử trước thất
bại, dùng thư mục backup mới; không ghi đè bản backup một cách im lặng.

Image triển khai: hquizlet-class:shards-20261108-v1.
CLASS_IMAGE đang ghim image này. Khi CI đã build đủ mã mới, chỉ bỏ override
sau khi xác nhận registry image hỗ trợ manifest phân tán và migration tool.
Release/backup cấu hình riêng tư:
/opt/hquizlet/releases/hls-shards-20261108-v1/

Sau khi video đã phân tán, KHÔNG chỉ đổi về image cũ: image cũ không hiểu
manifest JSON. Cần phục hồi từng ZIP từ backup về cùng object key và đổi
config/image có kiểm soát. Video mới tạo sau cutover cũng cần gom segment
thành ZIP trước khi rollback sang image không hỗ trợ shard. Giữ config node
và khóa AES để phục hồi. Backup ZIP chuyển đổi là tạm cho rollback, không
phải chính sách nhân đôi mọi video lâu dài.

## Xóa, lỗi node và vận hành

Deletion outbox hiện hữu gọi xóa thư mục segment trên mọi node, sau đó mới
xóa manifest; node offline khiến công việc được giữ lại để thử lần sau.
Mapping đọc là cố định nên không đọc nhầm segment khi thay thứ tự cấu hình.

Không có replica: một node không truy cập được thì segment nằm trên đó
không phát được, có thể làm dừng video. Checksum chỉ phát hiện hỏng dữ liệu,
không khôi phục dữ liệu. Cần backup riêng cho video quan trọng. Các VPS dùng
chung nhà cung cấp cũng không phải các failure domain độc lập.

Theo dõi dung lượng trống, hàng đợi deletion, lỗi shard read và job processing
failed. Không xóa file thủ công hoặc thay node ID để cân bằng dung lượng.
Thiết kế hiện phân đều số đoạn, không bảo đảm byte trên bốn đĩa bằng nhau.

Cloudflare public proxy/Worker vẫn là đường phát hiện hữu. Với lưu lượng
video lớn cần đường media IPv4/IPv6 và dịch vụ phân phối phù hợp; tham khảo:
https://developers.cloudflare.com/fundamentals/reference/policies-compliances/delivering-videos-with-cloudflare/

## Kiểm chứng

Go tests: toàn bộ services/class/... đã đạt, gồm SSH multi-host-key pinning,
phân phối segment một bản, mapping qua restart, checksum, từ chối traversal,
node offline khi publish, retry deletion, lỗi commit manifest giữ ZIP cũ,
giới hạn reader và giải phóng slot.

Preflight từ network container Class đã xác minh service key, fingerprint và
StatVFS trên đủ video-1, video-2, video-3, video-4. Class mới báo healthy.
Kiểm thử production PASS: upload 20 MiB có retry tiếp tục sau một lần mất
kết nối SFTP, chuyển mã ready, đọc playlist/khóa/segment qua API có xác thực,
từ chối các yêu cầu media chưa đăng nhập. Fixture 25 giây được remux thành
một segment do keyframe nguồn; kiểm thử này không tự chứng minh phân bố bốn node.
Đã xóa video/lớp/tài khoản QA; hàng đợi deletion về 0.

Video thật ID 7 đã được chuyển mà không đổi object key hoặc khóa AES:

| Node | Số segment | Byte segment đã xác minh |
| --- | --- | --- |
| video-1 | 16 | 2561456 |
| video-2 | 16 | 2089760 |
| video-3 | 16 | 1976976 |
| video-4 | 17 | 2673152 |

Toàn bộ 65 segment đọc lại qua tuyến App tới bốn VPS đều khớp SHA-256.
Manifest trên srv-pah58oxa còn 10937 byte, thay cho ZIP 9312000 byte.
Segment đầu lấy từ node mới đã giải mã bằng khóa production và FFmpeg decode
thành công. ZIP gốc 8,9 MiB giữ trong thư mục migration trên App để rollback.
Class healthy, DB chỉ còn video thật ID 7 trạng thái ready. Chưa kiểm thử UI
bằng phiên người dùng thật hoặc đo số người xem đồng thời.
