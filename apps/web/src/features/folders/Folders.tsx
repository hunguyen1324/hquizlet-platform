import React, { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, folderApi, studySetApi } from "../../lib/api";
import type { FolderDetail, FolderSummary } from "../../lib/api";
import type { StudySet } from "../../types";
import { useAuth } from "../auth/AuthContext";

type Props = { onBack: () => void; onOpenSet: (id: number) => void };
type Mode = "list" | "create" | "detail" | "edit";

export function Folders({ onBack, onOpenSet }: Props) {
  const { token, logout } = useAuth();
  const [mode, setMode] = useState<Mode>("list");
  const [folders, setFolders] = useState<FolderSummary[]>([]);
  const [folder, setFolder] = useState<FolderDetail | null>(null);
  const [sets, setSets] = useState<StudySet[]>([]);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleError = useCallback((value: unknown) => {
    if (value instanceof ApiError && value.status === 401) {
      void logout();
      return;
    }
    setError(value instanceof Error ? value.message : "Không thể hoàn tất thao tác.");
  }, [logout]);

  const loadFolders = useCallback(async () => {
    setLoading(true); setError("");
    try { setFolders(await folderApi.listFolders(token)); }
    catch (value) { handleError(value); }
    finally { setLoading(false); }
  }, [handleError, token]);

  useEffect(() => { if (mode === "list") void loadFolders(); }, [loadFolders, mode]);

  async function openFolder(id: number) {
    setLoading(true); setError("");
    try { setFolder(await folderApi.getFolder(token, id)); setMode("detail"); }
    catch (value) { handleError(value); }
    finally { setLoading(false); }
  }

  async function openAddDialog() {
    setError("");
    try { const result = await studySetApi.list(token, { per_page: 100 }); setSets(result.items ?? []); }
    catch (value) { handleError(value); return; }
    const dialog = document.getElementById("add-study-set-dialog") as HTMLDialogElement | null;
    dialog?.showModal();
  }

  async function submitForm(event: React.FormEvent) {
    event.preventDefault();
    if (!title.trim()) { setError("Tiêu đề là bắt buộc."); return; }
    setSaving(true); setError("");
    try {
      if (mode === "edit" && folder) {
        await folderApi.updateFolder(token, folder.id, { title, description });
        await openFolder(folder.id);
      } else {
        const created = await folderApi.createFolder(token, { title, description });
        await openFolder(created.id);
      }
    } catch (value) { handleError(value); }
    finally { setSaving(false); }
  }

  async function addSet(studySetId: number) {
    if (!folder) return;
    try {
      await folderApi.addStudySetToFolder(token, folder.id, studySetId);
      (document.getElementById("add-study-set-dialog") as HTMLDialogElement | null)?.close();
      await openFolder(folder.id);
    } catch (value) { handleError(value); }
  }

  async function removeSet(studySetId: number) {
    if (!folder || !window.confirm("Gỡ học phần khỏi thư mục? Học phần gốc sẽ không bị xóa.")) return;
    try { await folderApi.removeStudySetFromFolder(token, folder.id, studySetId); await openFolder(folder.id); }
    catch (value) { handleError(value); }
  }

  async function deleteFolder() {
    if (!folder || !window.confirm(`Xóa thư mục “${folder.title}”? Các học phần gốc vẫn được giữ lại.`)) return;
    try { await folderApi.deleteFolder(token, folder.id); setFolder(null); setMode("list"); }
    catch (value) { handleError(value); }
  }

  const included = useMemo(() => new Set(folder?.studySets.map((set) => set.id) ?? []), [folder]);

  if (mode === "create" || mode === "edit") return (
    <section className="folder-page">
      <div className="page-heading"><div><p className="eyebrow">Thư mục</p><h1>{mode === "edit" ? "Sửa thư mục" : "Tạo thư mục"}</h1></div></div>
      <form className="panel stack" onSubmit={submitForm}>
        <label>Tiêu đề<input value={title} onChange={(event) => setTitle(event.target.value)} autoFocus /></label>
        <label>Mô tả (không bắt buộc)<textarea value={description} onChange={(event) => setDescription(event.target.value)} /></label>
        {error && <p className="message message--error" role="alert">{error}</p>}
        <div className="folder-actions"><button className="primary-button" disabled={saving}>{saving ? "Đang lưu..." : "Lưu thư mục"}</button><button type="button" className="ghost-button" onClick={() => setMode(folder ? "detail" : "list")}>Hủy</button></div>
      </form>
    </section>
  );

  if (mode === "detail" && folder) return (
    <section className="folder-page">
      <div className="page-heading"><div><p className="eyebrow">Thư mục</p><h1>{folder.title}</h1><p>{folder.description || "Chưa có mô tả"}</p></div><div className="folder-actions"><button className="ghost-button" onClick={() => { setTitle(folder.title); setDescription(folder.description); setMode("edit"); }}>Sửa</button><button className="danger" onClick={() => void deleteFolder()}>Xóa</button></div></div>
      <div className="folder-actions"><button className="ghost-button" onClick={() => setMode("list")}>← Tất cả thư mục</button><button className="primary-button" onClick={() => void openAddDialog()}>Thêm học phần</button></div>
      {error && <p className="message message--error" role="alert">{error}</p>}
      <section className="set-grid folder-grid">
        {folder.studySets.length === 0 ? <div className="empty-panel"><h2>Thư mục còn trống</h2><p>Thêm một học phần để bắt đầu sắp xếp thư viện.</p><button className="primary-button" onClick={() => void openAddDialog()}>Thêm học phần</button></div> : folder.studySets.map((set) => <article className="set-card" key={set.id}><span>{set.flashcardCount ?? 0} thẻ</span><strong>{set.title}</strong><small>{set.description || "Chưa có mô tả"}</small><div className="folder-actions"><button className="secondary-button" onClick={() => onOpenSet(set.id)}>Mở</button><button className="ghost-button" onClick={() => void removeSet(set.id)}>Gỡ</button></div></article>)}
      </section>
      <dialog id="add-study-set-dialog" className="folder-dialog"><form method="dialog"><div className="page-heading"><div><h2>Thêm học phần</h2><p>Chọn một học phần thuộc tài khoản của bạn.</p></div><button className="ghost-button" aria-label="Đóng">×</button></div></form><div className="stack">{sets.map((set) => <button className="mini-card" key={set.id} disabled={included.has(set.id)} onClick={() => void addSet(set.id)}><span>{set.title}</span><strong>{included.has(set.id) ? "Đã thêm" : "Thêm"}</strong></button>)}</div></dialog>
    </section>
  );

  return <section className="folder-page"><div className="page-heading"><div><p className="eyebrow">Thư viện</p><h1>Thư mục</h1><p>Sắp xếp học phần theo chủ đề mà không thay đổi dữ liệu gốc.</p></div><button className="primary-button" onClick={() => { setFolder(null); setTitle(""); setDescription(""); setError(""); setMode("create"); }}>Tạo thư mục</button></div><button className="ghost-button" onClick={onBack}>← Học phần</button>{error && <p className="message message--error" role="alert">{error} <button className="ghost-button" onClick={() => void loadFolders()}>Thử lại</button></p>}<section className="set-grid folder-grid">{loading ? <div className="loading-skeleton" aria-busy="true">{[1,2,3].map((id) => <div className="skeleton-row" key={id} />)}</div> : folders.length === 0 && !error ? <div className="empty-panel"><h2>Chưa có thư mục</h2><p>Tạo thư mục đầu tiên để nhóm các học phần liên quan.</p><button className="primary-button" onClick={() => setMode("create")}>Tạo thư mục</button></div> : folders.map((item) => <button className="set-card" key={item.id} onClick={() => void openFolder(item.id)}><span>{item.studySetCount} học phần</span><strong>{item.title}</strong><small>{item.description || "Chưa có mô tả"}</small></button>)}</section></section>;
}
