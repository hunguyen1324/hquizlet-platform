import { useEffect, useRef, useState } from "react";
import { PrivateVideoPlayer } from "./PrivateVideoPlayer";
import { classVideoApi, type ClassVideo, type ClassVideoInput } from "../../lib/api";
import type { ClassMember } from "../../types";
import { useAuth } from "../auth/AuthContext";
import { VideoProcessingProgress } from "./VideoProcessingProgress";

const emptyInput: ClassVideoInput = { title: "", url: "", audience: "all", viewerIds: [] };

export function ClassVideos({ classId, members, canManage, mode = "videos" }: { classId: number; members: ClassMember[]; canManage: boolean; mode?: "videos" | "upload" | "permissions" }) {
  const { token } = useAuth();
  const [videos, setVideos] = useState<ClassVideo[]>([]);
  const [input, setInput] = useState<ClassVideoInput>(emptyInput);
  const [editing, setEditing] = useState<number>();
  const [playing, setPlaying] = useState<ClassVideo>();
  const [playbackURL, setPlaybackURL] = useState("");
  const [source, setSource] = useState<"sftp" | "youtube">("sftp");
  const [file, setFile] = useState<File>();
  const [fileInputKey, setFileInputKey] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const playerDialog = useRef<HTMLDialogElement>(null);
  const [success, setSuccess] = useState("");
  useEffect(() => {
    if (playing && mode !== "upload") playerDialog.current?.showModal();
  }, [playing, mode]);
  const [category, setCategory] = useState("");
  const [search, setSearch] = useState("");
  useEffect(() => { if (!busy) { setEditing(undefined); setInput(emptyInput); setCategory(""); setPlaying(undefined); setSuccess(""); } }, [mode]);
  const filtered = videos.filter(v => v.title.toLocaleLowerCase("vi").includes(search.toLocaleLowerCase("vi")));
  const categories = [...new Set(filtered.map(v => v.category || "Video khác"))];

  useEffect(() => {
    let active = true;
    setPlaying(undefined); setPlaybackURL(""); setVideos([]); setLoading(true); setError("");
    if (!token) return;
    classVideoApi.list(token, classId).then(data => { if (active) setVideos(data); })
      .catch(err => { if (active) setError(err instanceof Error ? err.message : "Không tải được video"); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [token, classId]);

  async function refresh() {
    if (!token) return;
    const data = await classVideoApi.list(token, classId);
    setVideos(data);
    setPlaying(undefined);
  }

  const hasProcessing = videos.some(video => video.processingState === "processing");
  useEffect(() => {
    if (!token || !hasProcessing) return;
    let active = true;
    const timer = window.setInterval(() => {
      classVideoApi.list(token, classId).then(data => { if (active) setVideos(data); })
        .catch(err => { if (active) setError(err instanceof Error ? err.message : "Không kiểm tra được trạng thái video"); });
    }, 5000);
    return () => { active = false; window.clearInterval(timer); };
  }, [token, classId, hasProcessing]);

  return <div className="tab-content class-videos">
    <div className="class-section-heading"><div><h2>{mode === "upload" ? "Tải video lên nhóm" : mode === "permissions" ? "Phân quyền video" : "Thư viện video"}</h2><p>Chia sẻ bài giảng, lưu lại kiến thức, học theo cách của bạn.</p></div><span className="class-section-icon" aria-hidden="true">▷</span></div>
    <div className="class-video-notice"><span aria-hidden="true">ⓘ</span><p>Video tải lên chỉ dành cho thành viên được cấp quyền. Quyền riêng tư của video YouTube do YouTube quản lý.</p></div>
    {success && <p role="status">{success}</p>}
    {error && <p role="alert" className="error-message">{error}</p>}
    {canManage && (mode === "upload" || (mode === "permissions" && editing)) && <form className="class-compose class-video-form" onSubmit={async event => {
      event.preventDefault(); if (!token || busy) return;
      setBusy(true); setError(""); setSuccess(""); setUploadProgress(0);
      try {
        let saved: ClassVideo;
        if (!editing && source === "sftp") {
          if (!file) throw new Error("Hãy chọn file video MP4 hoặc WebM.");
          saved = await classVideoApi.upload(token, classId, input, file, setUploadProgress);
        } else {
          saved = await classVideoApi.save(token, classId, input, editing);
        }
        // Preserve the created ID so retrying a metadata failure cannot upload a duplicate.
        setEditing(saved.id); setFile(undefined); setFileInputKey(k => k + 1);
        setVideos(current => [saved, ...current.filter(v => v.id !== saved.id)]);
        try { await classVideoApi.setCategory(token, classId, saved.id, category); }
        catch { throw new Error("Video đã lưu nhưng chưa cập nhật được chủ đề. Bấm Lưu thay đổi để thử lại."); }
        setInput(emptyInput); setCategory(""); setEditing(undefined); setFile(undefined); setFileInputKey(k => k + 1); await refresh(); setSuccess("Đã lưu video. Bạn có thể xem tại tab Video và chỉnh quyền tại tab Phân quyền.");
      } catch (err) { setError(err instanceof Error ? err.message : "Không lưu được video"); }
      finally { setBusy(false); }
    }}>
      <div className="class-compose-title"><h3>{editing ? "Chỉnh sửa video" : "Chia sẻ video mới"}</h3><p>{editing ? "Cập nhật thông tin và quyền xem của video." : "Một bài giảng hay có thể truyền cảm hứng cho cả nhóm."}</p></div>
      <label className="class-field-wide">Tiêu đề video<input required placeholder="Ví dụ: Ôn tập chương 1 — Những kiến thức cần nhớ" maxLength={200} value={input.title} onChange={e => setInput({ ...input, title: e.target.value })} /></label>
      <label>Chủ đề<input maxLength={100} placeholder="Ví dụ: Giao tiếp hàng ngày" list="class-video-categories" value={category} onChange={e => setCategory(e.target.value)} /><datalist id="class-video-categories">{[...new Set(videos.map(v => v.category).filter(Boolean))].map(c => <option key={c} value={c} />)}</datalist></label>
      {!editing && <label>Nguồn video<select value={source} onChange={e => setSource(e.target.value as "sftp" | "youtube")} disabled={busy}>
        <option value="sftp">Tải video riêng tư lên</option><option value="youtube">Liên kết YouTube</option>
      </select></label>}
      {source === "youtube" ? <label>Liên kết YouTube<input required type="url" placeholder="https://www.youtube.com/watch?v=..." value={input.url} onChange={e => setInput({ ...input, url: e.target.value })} /></label>
        : !editing && <label className="class-upload-field class-field-wide"><span className="class-upload-symbol" aria-hidden="true">↑</span><strong>{file ? file.name : "Chọn video để tải lên"}</strong><span>MP4 hoặc WebM · Tối đa 1 GiB</span><input aria-label="Chọn file video MP4 hoặc WebM" key={fileInputKey} required type="file" accept="video/mp4,video/webm,.mp4,.webm" disabled={busy} onChange={e => setFile(e.target.files?.[0])} /></label>}
      <label>Ai được xem?<select value={input.audience} onChange={e => setInput({ ...input, audience: e.target.value as ClassVideoInput["audience"] })}>
        <option value="all">Tất cả thành viên nhóm</option><option value="selected">Chọn thành viên</option>
      </select></label>
      {input.audience === "selected" && <fieldset className="class-field-wide"><legend>Thành viên được xem (chủ nhóm và giáo viên luôn có quyền)</legend>
        {members.filter(m => m.role === "student").map(m => <label key={m.userId} style={{ display: "block" }}>
          <input type="checkbox" checked={input.viewerIds.includes(m.userId)} onChange={e => setInput({ ...input, viewerIds: e.target.checked ? [...input.viewerIds, m.userId] : input.viewerIds.filter(id => id !== m.userId) })} /> Thành viên #{m.userId}
        </label>)}
      </fieldset>}
      {source === "sftp" && !editing && <small className="class-field-wide">Video sẽ được xử lý sau khi tải lên để sẵn sàng phát cho nhóm.</small>}
      {busy && source === "sftp" && !editing && <progress className="class-field-wide" value={uploadProgress} max={100} aria-label="Tiến độ tải video" />}
      <div className="class-form-actions class-field-wide"><button className="primary-button" disabled={busy}>{busy ? source === "sftp" && !editing ? uploadProgress === 100 ? "Đang hoàn tất tải lên..." : `Đang tải video... ${uploadProgress}%` : "Đang lưu..." : editing ? "Lưu thay đổi" : "+ Thêm video"}</button>
        {editing && <button type="button" className="ghost-button" disabled={busy} onClick={() => { setEditing(undefined); setInput(emptyInput); setSource("sftp"); setCategory(""); }}>Hủy</button>}
      </div>
    </form>}
    {mode !== "upload" && <>
    <div className="class-list-heading"><h3>{mode === "permissions" ? "Quản lý quyền xem" : "Tất cả video"}</h3><span>{videos.length} video</span></div><label className="video-search">Tìm video<input type="search" placeholder="Tìm theo tiêu đề…" value={search} onChange={e => setSearch(e.target.value)} /></label>
    {loading ? <p role="status">Đang tải video...</p> : videos.length === 0 && !error ? <div className="empty-state"><span className="class-empty-icon" aria-hidden="true">▷</span><h3>Góc học tập đang chờ bạn</h3><p>Những video được chia sẻ với bạn sẽ xuất hiện tại đây.</p></div> : null}
    {!loading && videos.length > 0 && filtered.length === 0 && <p>Không tìm thấy video phù hợp.</p>}
    {categories.map(name => <section key={name} className="video-category"><h3>{name}</h3><ul className="video-library-grid">
      {filtered.filter(v => (v.category || "Video khác") === name).map(video => <li key={video.id} className="video-library-card">
        <button className="video-library-open" disabled={video.processingState === "processing" || video.processingState === "failed"} onClick={async () => {
          if (!token) return;
          setError(""); setPlaying(undefined);
          try {
            const current = await classVideoApi.list(token, classId);
            setVideos(current);
            const allowed = current.find(v => v.id === video.id);
            if (!allowed) throw new Error("Bạn không còn quyền xem video này.");
            if (allowed.processingState === "processing") throw new Error("Video đang được mã hóa. Hãy chờ xử lý hoàn tất.");
            if (allowed.processingState === "failed") throw new Error("Xử lý video thất bại. Hãy xóa và tải lại video.");
            const url = allowed.source === "sftp" ? await classVideoApi.playback(token, classId, allowed.id) : "";
            setPlaybackURL(url); setPlaying(allowed);
          } catch (err) { setError(err instanceof Error ? err.message : "Không mở được video"); }
        }}><span className="video-thumbnail">{video.youtubeId ? <img src={`https://i.ytimg.com/vi/${video.youtubeId}/hqdefault.jpg`} alt="" loading="lazy" /> : <span className="video-thumbnail-art" aria-hidden="true">▷</span>}<span className="video-thumbnail-play" aria-hidden="true">▶</span></span><strong>{video.title}</strong><small>{video.source === "sftp" ? "Bài giảng của nhóm" : "YouTube"}</small></button>
        {video.processingState === "processing" && <VideoProcessingProgress video={video} />}
        {video.processingState === "failed" && <span role="status">Xử lý thất bại. Hãy xóa và tải lại video.</span>}
        {canManage && mode === "permissions" && <>
          <span className="class-audience-badge">{video.audience === "all" ? "Cả nhóm" : `${video.viewerIds.length} người được chọn`}</span>
          <button className="ghost-button" disabled={busy} onClick={() => { setEditing(video.id); setCategory(video.category || ""); setSource(video.source === "sftp" ? "sftp" : "youtube"); setInput({ title: video.title, url: video.youtubeId ? `https://www.youtube.com/watch?v=${video.youtubeId}` : "", audience: video.audience, viewerIds: video.viewerIds }); }}>Sửa / Quyền xem</button>
          <button className="danger-button" disabled={busy} onClick={async () => {
            if (!token || !confirm("Xóa video khỏi nhóm?")) return;
            setBusy(true); setError("");
            try { await classVideoApi.remove(token, classId, video.id); if (editing === video.id) { setEditing(undefined); setInput(emptyInput); } await refresh(); }
            catch (err) { setError(err instanceof Error ? err.message : "Không xóa được video"); }
            finally { setBusy(false); }
          }}>Xóa</button>
        </>}
      </li>)}
    </ul></section>)}
    </>}
    {playing && mode !== "upload" && <dialog ref={playerDialog} className="video-modal-backdrop" aria-label={playing.title} onCancel={() => setPlaying(undefined)}><section className="class-video-player">
      <h3>{playing.title}</h3><button className="ghost-button" onClick={() => setPlaying(undefined)}>Đóng video</button>
      {playing.source === "sftp" ? <PrivateVideoPlayer key={playing.id} title={playing.title} url={playbackURL} token={token ?? ""} classId={classId} videoId={playing.id} onError={setError} />
        : <iframe key={playing.id} title={playing.title} src={`https://www.youtube-nocookie.com/embed/${playing.youtubeId}?rel=0`} style={{ width: "100%", aspectRatio: "16 / 9", border: 0 }} allow="encrypted-media; picture-in-picture; fullscreen" allowFullScreen referrerPolicy="strict-origin-when-cross-origin" />}
    </section></dialog>}
  </div>;
}
