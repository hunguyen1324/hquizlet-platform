export function videoPlaybackError(status: number | undefined, type: string): string {
  if (status === 401) return "Phiên đăng nhập hoặc vé xem video đã hết hạn. Hãy đóng và mở lại video; nếu vẫn lỗi, hãy đăng nhập lại.";
  if (status === 403) return "Bạn không còn quyền xem video này.";
  if (status === 404) return "Không tìm thấy video hoặc đoạn video trên máy lưu trữ.";
  if (status && status >= 500) return `Máy chủ không tải được dữ liệu video (HTTP ${status}). Hãy thử lại sau hoặc báo quản trị viên kiểm tra kho video.`;
  if (type === "mediaError") return "Không giải mã hoặc phát được dữ liệu video. Hãy thử mở lại video hoặc dùng trình duyệt khác.";
  return "Không tải được video. Hãy kiểm tra kết nối mạng rồi đóng và mở lại video.";
}
