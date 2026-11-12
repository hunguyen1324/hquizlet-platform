# Whisper thử nghiệm trên VPS ::d0

Máy: `2001:470:24:849::d0` / `10.10.10.22`, hostname `v6-micro-37-oauj.cldcode.host`, RAM 1 GiB. Kết nối qua Data `117.1.91.238:30149`.

Cài whisper.cpp v1.8.2 và model đa ngôn ngữ `ggml-base-q5_1.bin`. Build Release, một luồng, tắt AVX/AVX2/FMA/F16C/AVX512/BMI2 để phù hợp CPU Xeon X5650. Model ở `/opt/whisper/models`, source/binary ở `/opt/whisper/whisper.cpp`.

## Đăng nhập từ PowerShell

```powershell
cd D:\quizlet\hquizlet-platform
$proxy = 'ssh -o HostKeyAlias=[vn-hn.cloudcode.io.vn]:30149 -i tmp/ci-ssh/data_deploy -p 30149 -W [%h]:%p root@117.1.91.238'
ssh -o "ProxyCommand=$proxy" root@2001:470:24:849::d0
```

Nhập mật khẩu VPS khi được hỏi. Không lưu mật khẩu trong lệnh.

## Upload file từ PowerShell

```powershell
scp -o "ProxyCommand=$proxy" .\audio.mp3 'root@[2001:470:24:849::d0]:/opt/whisper/input/audio.mp3'
```

## Chạy trên VPS

```bash
transcribe /opt/whisper/input/audio.mp3 vi /opt/whisper/output/audio
cat /opt/whisper/output/audio.txt
```

Ngôn ngữ: `vi` tiếng Việt, `ja` tiếng Nhật, `en` tiếng Anh, `auto` tự phát hiện. Lệnh xuất `.txt` và `.srt`, tự đổi audio về WAV mono 16 kHz, chạy một luồng và chỉ cho phép một tác vụ cùng lúc. Không chạy thêm whisper-cli ngoài wrapper khi đang có tác vụ.

Thử mẫu có sẵn:

```bash
transcribe /opt/whisper/whisper.cpp/samples/jfk.wav en /opt/whisper/output/demo
```

Chưa nối NATS, API hoặc giao diện web. Đây là cài đặt thử độc lập theo yêu cầu.

Nguồn: https://github.com/ggml-org/whisper.cpp/releases/tag/v1.8.2 và https://huggingface.co/ggerganov/whisper.cpp.

## Kết quả đã đo

Cài đặt và kiểm thử hoàn tất. Source commit `4979e04f5dcaccb36057e059bbaed8a2f5288315`, model SHA-1 `a3733eda680ef76256db5fc5dd9de8629e62c5e7` đã đối chiếu đúng. Có OpenBLAS 0.3.32, giới hạn OPENBLAS_NUM_THREADS=1 và OMP_NUM_THREADS=1 trong wrapper.

Mẫu `samples/jfk.wav` dài 11 giây, tiếng Anh:

- Bản CPU không OpenBLAS: 146,32 giây, RAM đỉnh 200944 KiB.
- Bản OpenBLAS qua wrapper gồm chuyển đổi audio: 99,84 giây, RAM đỉnh 208796 KiB (khoảng 204 MiB).
- Đã xuất và đọc lại `/opt/whisper/output/wrapper-test.txt`, đồng thời tạo `/opt/whisper/output/wrapper-test.srt`.
- Nội dung mẫu nhận dạng đúng câu tiếng Anh; chưa đo chất lượng tiếng Việt/tiếng Nhật hoặc file dài. Không suy ra tuyến tính thời gian cho file dài từ mẫu này.
- Sau cài đặt đĩa còn khoảng 17 GiB. Không mở cổng API hoặc cài dịch vụ tự chạy; dùng lệnh transcribe khi cần.

## API đã bổ sung

Máy hiện chạy thêm HTTPS STT worker do systemd quản lý. App gửi file và nhận SRT qua coordinator. Xem [hướng dẫn API STT](stt-api.md). Lệnh transcribe thủ công vẫn dùng cùng khóa với worker API.
