import { QRCodeSVG } from "qrcode.react";

export function ClassInviteQr({ inviteCode }: { inviteCode: string }) {
  if (!inviteCode) return null;

  const joinUrl = new URL("/classes/join", window.location.origin);
  joinUrl.searchParams.set("code", inviteCode);

  return (
    <details className="class-invite-qr">
      <summary>Hiện mã QR</summary>
      <QRCodeSVG
        value={joinUrl.href}
        size={176}
        marginSize={4}
        title="Mã QR để tham gia lớp học"
      />
      <a href={joinUrl.href}>Mở liên kết tham gia</a>
    </details>
  );
}
