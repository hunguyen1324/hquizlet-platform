import type { ClassVideo } from "../../lib/api";

const stageLabels = {
  queued: "Đang chờ xử lý",
  preparing: "Đang chuẩn bị video",
  encoding: "Đang mã hóa video",
  storing: "Đang lưu video",
  ready: "Video đã sẵn sàng",
  failed: "Xử lý thất bại",
};

export function VideoProcessingProgress({ video }: { video: ClassVideo }) {
  const percent = typeof video.processingPercent === "number" && Number.isFinite(video.processingPercent)
    ? Math.max(0, Math.min(99, Math.floor(video.processingPercent))) : undefined;
  const label = stageLabels[video.processingStage ?? "queued"] ?? stageLabels.queued;
  return <div className="class-video-processing" role="status" aria-live="polite">
    <strong>Đang xử lý video{percent !== undefined ? ` · ${percent}%` : "…"}</strong>
    <progress value={percent} max={100} aria-label={`Tiến độ xử lý ${video.title}`} />
    <small>{label}</small>
  </div>;
}
