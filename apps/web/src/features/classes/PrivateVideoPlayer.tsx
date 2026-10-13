import { useEffect, useRef } from "react";
import Hls from "hls.js";
import { classVideoApi } from "../../lib/api";

export function PrivateVideoPlayer({ url, token, classId, videoId, title, onError }: {
  url: string; token: string; classId: number; videoId: number; title: string; onError: (message: string) => void;
}) {
  const ref = useRef<HTMLVideoElement>(null);
  const errorRef = useRef(onError); errorRef.current = onError;
  useEffect(() => {
    const video = ref.current;
    if (!video) return;
    if (!url.includes("/class-video-hls?")) { video.src = url; return () => { video.removeAttribute("src"); video.load(); }; }
    if (!Hls.isSupported()) { errorRef.current("Trình duyệt này chưa hỗ trợ phát HLS có xác thực. Hãy dùng Chrome, Edge hoặc Firefox phiên bản mới."); return; }
    let currentURL = url, refreshedAt = Date.now(), active = true;
    let renewal: Promise<string> | undefined;
    const hls = new Hls({ maxBufferLength: 30, xhrSetup: async (xhr, resource) => {
      if (Date.now() - refreshedAt > 120_000) {
        renewal ??= classVideoApi.playback(token, classId, videoId).finally(() => { renewal = undefined; });
        currentURL = await renewal; refreshedAt = Date.now();
      }
      if (!active) throw new Error("Player closed");
      const target = new URL(resource, window.location.href);
      const current = new URL(currentURL, window.location.href);
      if (target.origin !== current.origin || target.pathname !== current.pathname) throw new Error("Unexpected video resource");
      target.searchParams.set("ticket", current.searchParams.get("ticket") ?? "");
      xhr.open("GET", target.toString(), true);
      xhr.setRequestHeader("Authorization", `Bearer ${token}`);
    } });
    hls.on(Hls.Events.ERROR, (_event, data) => { if (data.fatal && active) errorRef.current("Không phát được video hoặc quyền xem đã hết hạn. Hãy đóng và mở lại video."); });
    hls.attachMedia(video); hls.loadSource(url);
    return () => { active = false; hls.destroy(); video.removeAttribute("src"); video.load(); };
  }, [url, token, classId, videoId]);
  return <video ref={ref} aria-label={title} controls playsInline preload="metadata" controlsList="nodownload" style={{ width: "100%", aspectRatio: "16 / 9", background: "#000" }} />;
}
