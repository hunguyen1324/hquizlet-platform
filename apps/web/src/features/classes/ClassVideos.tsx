import { useEffect, useState } from "react";
import { classVideoApi, type ClassVideo, type ClassVideoInput } from "../../lib/api";
import type { ClassMember } from "../../types";
import { useAuth } from "../auth/AuthContext";

const emptyInput: ClassVideoInput = { title: "", url: "", audience: "all", viewerIds: [] };

export function ClassVideos({ classId, members, canManage }: { classId: number; members: ClassMember[]; canManage: boolean }) {
  const { token } = useAuth();
  const [videos, setVideos] = useState<ClassVideo[]>([]);
  const [input, setInput] = useState<ClassVideoInput>(emptyInput);
  const [editing, setEditing] = useState<number>();
  const [playing, setPlaying] = useState<ClassVideo>();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let active = true;
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

  return <div className="tab-content">
    <p>Video chỉ hiển thị trong nhóm cho người được cấp quyền. Link YouTube không công khai vẫn có thể được chia sẻ ra ngoài; không thể ngăn hoàn toàn việc tải hoặc quay màn hình.</p>
    {error && <p role="alert" className="error-message">{error}</p>}
    {canManage && <form onSubmit={async event => {
      event.preventDefault(); if (!token || busy) return;
      setBusy(true); setError("");
      try {
        await classVideoApi.save(token, classId, input, editing);
        setInput(emptyInput); setEditing(undefined); await refresh();
      } catch (err) { setError(err instanceof Error ? err.message : "Không lưu được video"); }
      finally { setBusy(false); }
    }} style={{ display: "grid", gap: 12, marginBlock: 20 }}>
      <label>Tiêu đề video<input required maxLength={200} value={input.title} onChange={e => setInput({ ...input, title: e.target.value })} /></label>
      <label>Liên kết YouTube<input required type="url" placeholder="https://www.youtube.com/watch?v=..." value={input.url} onChange={e => setInput({ ...input, url: e.target.value })} /></label>
      <label>Ai được xem?<select value={input.audience} onChange={e => setInput({ ...input, audience: e.target.value as ClassVideoInput["audience"] })}>
        <option value="all">Tất cả thành viên nhóm</option><option value="selected">Chọn thành viên</option>
      </select></label>
      {input.audience === "selected" && <fieldset><legend>Thành viên được xem (chủ nhóm và giáo viên luôn có quyền)</legend>
        {members.filter(m => m.role === "student").map(m => <label key={m.userId} style={{ display: "block" }}>
          <input type="checkbox" checked={input.viewerIds.includes(m.userId)} onChange={e => setInput({ ...input, viewerIds: e.target.checked ? [...input.viewerIds, m.userId] : input.viewerIds.filter(id => id !== m.userId) })} /> Thành viên #{m.userId}
        </label>)}
      </fieldset>}
      <div><button className="primary-button" disabled={busy}>{busy ? "Đang lưu..." : editing ? "Lưu thay đổi" : "Thêm video"}</button>
        {editing && <button type="button" className="ghost-button" onClick={() => { setEditing(undefined); setInput(emptyInput); }}>Hủy</button>}
      </div>
    </form>}
    {loading ? <p>Đang tải video...</p> : videos.length === 0 && !error ? <div className="empty-state">Chưa có video được chia sẻ với bạn.</div> : null}
    <ul className="group-resource-list" style={{ listStyle: "none", padding: 0 }}>
      {videos.map(video => <li key={video.id} className="group-resource-item" style={{ marginBottom: 12 }}>
        <button className="group-resource-main" onClick={async () => {
          if (!token) return;
          setError(""); setPlaying(undefined);
          try {
            const current = await classVideoApi.list(token, classId);
            setVideos(current);
            const allowed = current.find(v => v.id === video.id);
            if (!allowed) throw new Error("Bạn không còn quyền xem video này.");
            setPlaying(allowed);
          } catch (err) { setError(err instanceof Error ? err.message : "Không mở được video"); }
        }}>▶ {video.title}</button>
        {canManage && <>
          <span>{video.audience === "all" ? "Cả nhóm" : `${video.viewerIds.length} người được chọn`}</span>
          <button className="ghost-button" disabled={busy} onClick={() => { setEditing(video.id); setInput({ title: video.title, url: `https://www.youtube.com/watch?v=${video.youtubeId}`, audience: video.audience, viewerIds: video.viewerIds }); }}>Sửa / Quyền xem</button>
          <button className="danger-button" disabled={busy} onClick={async () => {
            if (!token || !confirm("Xóa video khỏi nhóm?")) return;
            setBusy(true); setError("");
            try { await classVideoApi.remove(token, classId, video.id); if (editing === video.id) { setEditing(undefined); setInput(emptyInput); } await refresh(); }
            catch (err) { setError(err instanceof Error ? err.message : "Không xóa được video"); }
            finally { setBusy(false); }
          }}>Xóa</button>
        </>}
      </li>)}
    </ul>
    {playing && <section aria-label={playing.title}>
      <h3>{playing.title}</h3><button className="ghost-button" onClick={() => setPlaying(undefined)}>Đóng video</button>
      <iframe key={playing.id} title={playing.title} src={`https://www.youtube-nocookie.com/embed/${playing.youtubeId}?rel=0`} style={{ width: "100%", aspectRatio: "16 / 9", border: 0 }} allow="encrypted-media; picture-in-picture; fullscreen" allowFullScreen referrerPolicy="strict-origin-when-cross-origin" />
    </section>}
  </div>;
}
