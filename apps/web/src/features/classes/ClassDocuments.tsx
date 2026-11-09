import { useEffect, useState } from "react";
import { classDocumentApi, type ClassDocument } from "../../lib/api";
import { useAuth } from "../auth/AuthContext";

export function ClassDocuments({ classId, canManage }: { classId: number; canManage: boolean }) {
  const { token } = useAuth();
  const [documents, setDocuments] = useState<ClassDocument[]>([]);
  const [file, setFile] = useState<File>();
  const [inputKey, setInputKey] = useState(0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let active = true;
    if (!token) return;
    classDocumentApi.list(token, classId).then(d => { if (active) setDocuments(d); })
      .catch(e => { if (active) setError(e.message); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [token, classId]);
  return <div className="tab-content">
    <div className="class-section-heading"><div><h2>Tài liệu của nhóm</h2><p>Giáo trình, bài tập và tài liệu để cả nhóm cùng học.</p></div></div>
    {error && <p role="alert" className="error-message">{error}</p>}
    {canManage && <form className="class-compose document-upload" onSubmit={async e => {
      e.preventDefault(); if (!token || !file || busy) return;
      setBusy(true); setError("");
      try {
        const created = await classDocumentApi.upload(token, classId, file);
        setDocuments(d => [created, ...d]); setFile(undefined); setInputKey(k => k + 1);
      } catch (e) { setError(e instanceof Error ? e.message : "Không tải lên được tài liệu."); }
      finally { setBusy(false); }
    }}>
      <label className="class-upload-field"><span className="class-upload-symbol" aria-hidden="true">↑</span><strong>{file?.name || "Chọn tài liệu để tải lên"}</strong><span>PDF, Word, PowerPoint, Excel · Tối đa 20 MiB / file</span><input key={inputKey} type="file" required disabled={busy} accept=".pdf,.doc,.docx,.ppt,.pptx,.xls,.xlsx" onChange={e => setFile(e.target.files?.[0])} /></label>
      <p>Tất cả thành viên trong nhóm có thể tải tài liệu. Chủ nhóm và giáo viên có thể thêm, xóa file.</p>
      <button className="primary-button" disabled={busy || !file}>{busy ? "Đang tải lên…" : "Tải tài liệu lên"}</button>
    </form>}
    {loading ? <p role="status">Đang tải tài liệu…</p> : !documents.length && !error ? <div className="empty-state"><h3>Chưa có tài liệu</h3><p>Các file được chia sẻ trong nhóm sẽ xuất hiện tại đây.</p></div> : null}
    <ul className="group-resource-list">{documents.map(d => <li className="group-resource-item" key={d.id}>
      <span className="document-type">{d.name.split(".").pop()?.toUpperCase()}</span>
      <div className="class-member-copy"><strong>{d.name}</strong><small>{(d.sizeBytes / 1024 / 1024).toLocaleString("vi-VN", { maximumFractionDigits: 2 })} MiB · {new Date(d.createdAt).toLocaleDateString("vi-VN")}</small></div>
      <button className="ghost-button" disabled={busy} onClick={async () => {
        if (!token) return; setBusy(true); setError("");
        try { await classDocumentApi.download(token, classId, d); }
        catch (e) { setError(e instanceof Error ? e.message : "Không tải được file."); }
        finally { setBusy(false); }
      }}>↓ Tải xuống</button>
      {canManage && <button className="danger-button" disabled={busy} onClick={async () => {
        if (!token || !confirm(`Xóa tài liệu “${d.name}”?`)) return; setBusy(true); setError("");
        try { await classDocumentApi.remove(token, classId, d.id); setDocuments(items => items.filter(v => v.id !== d.id)); }
        catch (e) { setError(e instanceof Error ? e.message : "Không xóa được tài liệu."); }
        finally { setBusy(false); }
      }}>Xóa</button>}
    </li>)}</ul>
  </div>;
}
