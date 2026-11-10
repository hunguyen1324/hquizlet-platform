import { useEffect, useRef, type ReactNode } from "react";
import type { ClassVideo } from "../../lib/api";

export function VideoWatchDialog({ video, error, onClose, children }: {
  video: ClassVideo; error: string; onClose: () => void; children: ReactNode;
}) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = dialogRef.current;
    const previousFocus = document.activeElement as HTMLElement | null;
    const previousOverflow = document.body.style.overflow;
    dialog?.showModal();
    document.body.style.overflow = "hidden";
    return () => {
      dialog?.close();
      document.body.style.overflow = previousOverflow;
      previousFocus?.focus();
    };
  }, []);

  return <dialog ref={dialogRef} className="video-watch-dialog" aria-labelledby="video-watch-title"
    onCancel={event => { event.preventDefault(); onClose(); }}
    onClick={event => {
      if (event.target !== event.currentTarget) return;
      const bounds = event.currentTarget.getBoundingClientRect();
      if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) onClose();
    }}>
    <header className="video-watch-header">
      <div><p className="video-watch-eyebrow">{video.category || "Bài giảng của nhóm"}</p><h2 id="video-watch-title">{video.title}</h2></div>
      <button type="button" className="video-watch-close" onClick={onClose} aria-label="Đóng video" title="Đóng video (Esc)" autoFocus>
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" /></svg>
      </button>
    </header>
    <div className="video-watch-stage">{children}</div>
    {error && <p role="alert" className="video-watch-error">{error}</p>}
    <footer className="video-watch-footer"><span className="video-source-badge">{video.source === "sftp" ? "Video của nhóm" : "YouTube"}</span><span>Nhấn <kbd>Esc</kbd> để đóng</span></footer>
  </dialog>;
}
