import { useEffect, useState } from 'react';
import { classAssignmentApi, type ClassAssignment, type AssignmentInput } from '../../lib/api';
import { useAuth } from '../auth/AuthContext';
const empty = (): AssignmentInput => ({ title: '', instructions: '', dueAt: null, status: 'draft', deleteFileIds: [] });
const localDate = (date: string | null) => {
    if (!date)
        return '';
    const d = new Date(date);
    return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
};
export function ClassAssignments({ classId, actions }: {
    classId: number;
    actions: string[];
}) {
    const { token } = useAuth();
    const [items, setItems] = useState<ClassAssignment[]>([]);
    const [loading, setLoading] = useState(true);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');
    const [filter, setFilter] = useState('all');
    const [search, setSearch] = useState('');
    const [editing, setEditing] = useState<ClassAssignment | null>();
    const [input, setInput] = useState<AssignmentInput>(empty);
    const [files, setFiles] = useState<File[]>([]);
    const [fileKey, setFileKey] = useState(0);
    const can = (verb: string) => actions.includes(`class-assignment:${verb}`);
    useEffect(() => {
        let active = true;
        if (token)
            classAssignmentApi.list(token, classId).then(d => {
                if (active)
                    setItems(d);
            }).catch(e => {
                if (active)
                    setError(e.message);
            }).finally(() => {
                if (active)
                    setLoading(false);
            });
        return () => { active = false; };
    }, [token, classId]);
    function open(item: ClassAssignment | null) { setEditing(item); setInput(item ? { title: item.title, instructions: item.instructions, dueAt: item.dueAt, status: item.status, deleteFileIds: [] } : empty()); setFiles([]); setFileKey(k => k + 1); setError(''); setSuccess(''); }
    async function save(status: 'draft' | 'published') {
        if (!token || busy)
            return;
        setBusy(true);
        setError('');
        setSuccess('');
        try {
            const saved = await classAssignmentApi.save(token, classId, { ...input, status }, files, editing?.id);
            setItems(previous => [saved, ...previous.filter(a => a.id !== saved.id)]);
            setEditing(undefined);
            setFiles([]);
            setSuccess(status === 'published' ? 'Đã giao bài tập cho lớp.' : 'Đã lưu bản nháp. Học viên chưa thấy bài này.');
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Không lưu được bài tập');
        }
        finally {
            setBusy(false);
        }
    }
    const filtered = items.filter(a => (a.title + ' ' + a.instructions).toLocaleLowerCase('vi').includes(search.toLocaleLowerCase('vi')) && (filter === 'all' || filter === a.status || (filter === 'overdue' && a.status === 'published' && !!a.dueAt && new Date(a.dueAt) < new Date())));
    return <section className="tab-content assignments-panel">
  <div className="settings-section-title"><div><p className="group-eyebrow">HỌC TẬP CÓ KẾ HOẠCH</p><h2>Bài tập về nhà</h2><p>Đọc hướng dẫn, tải tài liệu và theo dõi hạn làm bài tại một nơi.</p></div>{can('Create') && editing === undefined && <button className="primary-button" onClick={() => open(null)}>+ Tạo bài tập</button>}</div>
  {error && <p className="error-message" role="alert">{error}</p>}{success && <p className="settings-success" role="status">{success}</p>}
  {editing !== undefined && <form className="settings-card assignment-editor" onSubmit={e => { e.preventDefault(); void save(input.status); }}>
   <div className="settings-section-title"><div><h3>{editing ? 'Chỉnh sửa bài tập' : 'Bài tập mới'}</h3><p>Bài nháp chỉ hiển thị với người có quyền quản lý bài tập.</p></div><span className="settings-chip">{input.status === 'draft' ? 'Bản nháp' : 'Đã giao'}</span></div>
   <label>Tiêu đề bài tập *<input autoFocus required maxLength={200} value={input.title} disabled={busy} onChange={e => setInput({ ...input, title: e.target.value })} placeholder="Ví dụ: Ôn tập chương 2"/></label>
   <label>Hướng dẫn<textarea rows={7} maxLength={20000} value={input.instructions} disabled={busy} onChange={e => setInput({ ...input, instructions: e.target.value })} placeholder="Nhập yêu cầu, câu hỏi và hướng dẫn làm bài…"/></label>
   <label>Hạn làm bài (không bắt buộc)<input type="datetime-local" value={localDate(input.dueAt)} disabled={busy} onChange={e => setInput({ ...input, dueAt: e.target.value ? new Date(e.target.value).toISOString() : null })}/></label>
   {!!editing?.files.length && <div><h4>File đã đính kèm</h4>{editing.files.map(f => <label className="assignment-existing-file" key={f.id}><input type="checkbox" disabled={busy} checked={!input.deleteFileIds.includes(f.id)} onChange={e => setInput({ ...input, deleteFileIds: e.target.checked ? input.deleteFileIds.filter(id => id !== f.id) : [...input.deleteFileIds, f.id] })}/>{f.name}<small>Bỏ chọn để xóa khi lưu</small></label>)}</div>}
   <label className="class-upload-field"><span className="class-upload-symbol" aria-hidden="true">↑</span><strong>Đính kèm tài liệu bài tập</strong><span>PDF, Word, Excel, PowerPoint · 20 MiB/file · tối đa 5 file, tổng 40 MiB</span><input key={fileKey} type="file" multiple accept=".pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx" disabled={busy} onChange={e => {
                const selected = Array.from(e.target.files || []);
                const kept = (editing?.files || []).filter(f => !input.deleteFileIds.includes(f.id));
                if (selected.length + kept.length > 5 || selected.some(f => f.size === 0 || f.size > 20 * 1024 * 1024) || selected.reduce((sum, f) => sum + f.size, 0) + kept.reduce((sum, f) => sum + f.sizeBytes, 0) > 40 * 1024 * 1024) {
                    setError('Tối đa 5 file; mỗi file từ 1 byte đến 20 MiB; tổng dung lượng tối đa 40 MiB.');
                    setFiles([]);
                    setFileKey(k => k + 1);
                    return;
                }
                setFiles(selected);
                setError('');
            }}/></label>
   {!!files.length && <ul className="assignment-new-files">{files.map((f, i) => <li key={i}>{f.name} <small>({(f.size / 1024 / 1024).toFixed(2)} MiB)</small></li>)}</ul>}
   <div className="settings-row-actions"><button type="submit" className="primary-button" disabled={busy || !input.title.trim()}>{busy ? 'Đang lưu…' : input.status === 'published' ? 'Lưu bài đã giao' : 'Lưu bản nháp'}</button>
    {can('Publish') && <button type="button" className="ghost-button" disabled={busy || !input.title.trim()} onClick={() => void save(input.status === 'draft' ? 'published' : 'draft')}>{input.status === 'draft' ? 'Giao bài ngay' : 'Chuyển về bản nháp'}</button>}
    <button type="button" className="ghost-button" disabled={busy} onClick={() => setEditing(undefined)}>Hủy</button></div>
  </form>}
  <div className="assignment-toolbar"><label>Tìm bài tập<input type="search" placeholder="Tìm theo tiêu đề, nội dung…" value={search} onChange={e => setSearch(e.target.value)}/></label><label>Trạng thái<select value={filter} onChange={e => setFilter(e.target.value)}><option value="all">Tất cả bài tập</option><option value="published">Đã giao</option><option value="draft">Bản nháp</option><option value="overdue">Đã qua hạn</option></select></label><span>{filtered.length} bài tập</span></div>
  {loading ? <p role="status">Đang tải bài tập…</p> : filtered.length === 0 ? <div className="empty-state"><span className="class-empty-icon" aria-hidden="true">▤</span><h3>Chưa có bài tập phù hợp</h3><p>{can('Create') ? 'Tạo bài tập đầu tiên, viết hướng dẫn và đính kèm tài liệu cho lớp.' : 'Bài tập đã giao sẽ xuất hiện ở đây.'}</p></div> : null}
  <div className="assignment-list">{filtered.map(a => <article className="assignment-card" key={a.id}>
   <div className="assignment-card-top"><span className={`assignment-status ${a.status}`}>{a.status === 'draft' ? 'Bản nháp' : 'Đã giao'}</span><small>Tạo {new Date(a.createdAt).toLocaleDateString('vi-VN')}</small></div>
   <h3>{a.title}</h3><p className={`assignment-due ${a.dueAt && new Date(a.dueAt) < new Date() ? 'overdue' : ''}`}>{a.dueAt ? `Hạn làm bài: ${new Date(a.dueAt).toLocaleString('vi-VN')}` : 'Không đặt hạn làm bài'}</p>
   {a.instructions && <p className="assignment-instructions">{a.instructions}</p>}
   {!!a.files.length && <div className="assignment-attachments"><h4>Tài liệu đính kèm ({a.files.length})</h4>{a.files.map(f => <button className="assignment-file" key={f.id} disabled={busy} onClick={async () => {
                        if (!token)
                            return;
                        setBusy(true);
                        setError('');
                        try {
                            await classAssignmentApi.download(token, classId, a.id, f);
                        }
                        catch (e) {
                            setError(e instanceof Error ? e.message : 'Không tải được file');
                        }
                        finally {
                            setBusy(false);
                        }
                    }}><span className="document-type">{f.name.split('.').pop()?.toUpperCase()}</span><span><strong>{f.name}</strong><small>{(f.sizeBytes / 1024 / 1024).toLocaleString('vi-VN', { maximumFractionDigits: 2 })} MiB</small></span><span aria-hidden="true">↓</span></button>)}</div>}
   {(can('Update') || can('Delete')) && <div className="settings-row-actions">{can('Update') && (a.status === 'draft' || can('Publish')) && <button className="ghost-button" disabled={busy} onClick={() => open(a)}>Chỉnh sửa</button>}{can('Delete') && <button className="danger-button" disabled={busy} onClick={async () => {
                        if (!token || !confirm(`Xóa bài tập “${a.title}” và các file đính kèm?`))
                            return;
                        setBusy(true);
                        setError('');
                        try {
                            await classAssignmentApi.remove(token, classId, a.id);
                            setItems(items => items.filter(item => item.id !== a.id));
                            setSuccess('Đã xóa bài tập.');
                        }
                        catch (e) {
                            setError(e instanceof Error ? e.message : 'Không xóa được bài tập');
                        }
                        finally {
                            setBusy(false);
                        }
                    }}>Xóa bài tập</button>}</div>}
  </article>)}</div>
 </section>;
}
