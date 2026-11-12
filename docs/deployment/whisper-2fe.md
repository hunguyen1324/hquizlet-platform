# Whisper thử nghiệm trên VPS ::2fe

Máy: `2001:470:24:849::2fe` / `10.0.0.52`, hostname `v6-micro-37-fnqn.cldcode.host`, RAM 1 GiB. Kết nối qua Data `117.1.91.238:30149`.

Cài từ bản runtime whisper.cpp v1.8.2 đã kiểm chứng trên ::d0 và model đa ngôn ngữ `ggml-base-q5_1.bin`. Build Release, một luồng, tắt AVX/AVX2/FMA/F16C/AVX512/BMI2 để phù hợp CPU Xeon X5650. Model ở `/opt/whisper/models`, source/binary ở `/opt/whisper/whisper.cpp`.

## Đăng nhập từ PowerShell

```powershell
cd D:\quizlet\hquizlet-platform
$proxy = 'ssh -o HostKeyAlias=[vn-hn.cloudcode.io.vn]:30149 -i tmp/ci-ssh/data_deploy -p 30149 -W [%h]:%p root@117.1.91.238'
ssh -o "ProxyCommand=$proxy" root@2001:470:24:849::2fe
```

Nhập mật khẩu VPS khi được hỏi. Không lưu mật khẩu trong lệnh.

## Upload file từ PowerShell

```powershell
scp -o "ProxyCommand=$proxy" .\audio.mp3 'root@[2001:470:24:849::2fe]:/opt/whisper/input/audio.mp3'
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


Gói runtime có SHA-256 `756a509fb56b813b93df6559bb1cc0f44493c9a178e1174db070af5da44a994e`. Source gốc commit `4979e04f5dcaccb36057e059bbaed8a2f5288315`. Model SHA-1 `a3733eda680ef76256db5fc5dd9de8629e62c5e7`. Máy đích cài FFmpeg và OpenBLAS runtime; không cần cài compiler. Wrapper giới hạn OpenBLAS và OpenMP một luồng.

## Kiểm thử hoàn tất

Ngày 12/11/2026, mẫu JFK tiếng Anh dài 11 giây chạy qua wrapper thành công:

- Thời gian tổng: 120,87 giây (khoảng 2 phút), gồm đổi audio và đọc model.
- RAM đỉnh: 207520 KiB, khoảng 203 MiB.
- Tạo và đọc lại `wrapper-test.txt`, xác nhận có `wrapper-test.srt` trong `/opt/whisper/output`.
- Checksum gói runtime và model đều khớp; liên kết thư viện đầy đủ; OOM=0 trong lần thử.
- Đĩa còn khoảng 18 GiB. Chưa đánh giá độ chính xác tiếng Việt/Nhật hoặc file dài.

Không có API public hoặc dịch vụ tự chạy. Đây là bản cài thử bằng dòng lệnh, cùng model và tham số với ::d0. Tốc độ thực tế có thể thay đổi theo tải CPU/đĩa dùng chung.

## API đã bổ sung

Máy hiện chạy thêm HTTPS STT worker do systemd quản lý. App gửi file và nhận SRT qua coordinator. Xem [hướng dẫn API STT](stt-api.md). Lệnh transcribe thủ công vẫn dùng cùng khóa với worker API.
