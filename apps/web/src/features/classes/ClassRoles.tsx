import { useEffect, useRef, useState } from 'react';
import { classAccessApi, effectiveRoleActions, type ClassAccess, type ClassRole } from '../../lib/api';
import { useAuth } from '../auth/AuthContext';
const emptyRole = (): ClassRole => ({ key: '', name: '', description: '', managed: false, allow: [], deny: [] });
export function ClassRoles({ classId }: {
    classId: number;
}) {
    const editorRef = useRef<HTMLFormElement>(null);
    const { token, user } = useAuth();
    const [data, setData] = useState<ClassAccess>();
    const [section, setSection] = useState<'roles' | 'members'>('roles');
    const [draft, setDraft] = useState<ClassRole>();
    const [selectedUser, setSelectedUser] = useState('');
    const [keys, setKeys] = useState<string[]>([]);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');
    const [query, setQuery] = useState('');
    useEffect(() => {
        let active = true;
        if (token)
            classAccessApi.get(token, classId).then(d => {
                if (active)
                    setData(d);
            }).catch(e => {
                if (active)
                    setError(e.message);
            });
        return () => { active = false; };
    }, [token, classId]);
    async function refresh() {
        if (token)
            setData(await classAccessApi.get(token, classId));
    }
    async function perform(work: () => Promise<unknown>, message: string) {
        if (busy)
            return;
        setBusy(true);
        setError('');
        setSuccess('');
        try {
            await work();
            await refresh();
            setSuccess(message);
        }
        catch (e) {
            setError(e instanceof Error ? e.message : 'Không lưu được thay đổi');
        }
        finally {
            setBusy(false);
        }
    }
    useEffect(() => {
        if (draft)
            editorRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }, [draft?.key, draft?.managed]);
    if (!data)
        return <div role={error ? 'alert' : 'status'}>{error || 'Đang tải vai trò và quyền…'}</div>;
    const groups = [...new Set(data.catalog.map(a => a.group))];
    const selected = data.members.find(m => String(m.userId) === selectedUser);
    const actions = effectiveRoleActions(keys, data.roles);
    return <section className="class-role-panel">
  <div className="settings-section-title"><div><h2>Vai trò và quyền truy cập</h2><p>Tạo role, chọn quyền rồi gán cho thành viên trong lớp này.</p></div>{section === 'roles' && <button className="primary-button" disabled={busy} onClick={() => { setDraft(emptyRole()); setSuccess(''); }}>+ Tạo vai trò</button>}</div>
  <div className="settings-notice">Quyền từ chối được ưu tiên khi một người có nhiều role. Role được gán ở đây thay thế vai trò giáo viên / học viên cũ. Chủ lớp luôn giữ toàn quyền.</div>
  <div className="settings-subtabs"><button className={section === 'roles' ? 'active' : ''} onClick={() => setSection('roles')}>Vai trò ({data.roles.length})</button><button className={section === 'members' ? 'active' : ''} onClick={() => { setSection('members'); setDraft(undefined); }}>Gán cho thành viên ({data.members.length})</button></div>
  {error && <p className="error-message" role="alert">{error}</p>}{success && <p className="settings-success" role="status">{success}</p>}
  {section === 'roles' && <>
   <label className="settings-search">Tìm vai trò<input value={query} onChange={e => setQuery(e.target.value)} placeholder="Tên hoặc mô tả vai trò…"/></label>
   <div className="role-card-grid">{data.roles.filter(r => (r.name + ' ' + r.description).toLocaleLowerCase('vi').includes(query.toLocaleLowerCase('vi'))).map(role => <article className={`role-card ${draft?.key === role.key && draft?.key ? 'selected' : ''}`} key={role.key}>
    <div className="role-card-heading"><span className="role-card-icon" aria-hidden="true">♧</span><span className="settings-chip">{role.managed ? 'Hệ thống' : 'Tùy chỉnh'}</span></div>
    <h3>{role.name}</h3><p>{role.description}</p><small>{role.allow.length} quyền cho phép · {role.deny.length} từ chối · {data.members.filter(m => m.roleKeys.includes(role.key)).length} thành viên</small>
    <div className="settings-row-actions"><button className="ghost-button" disabled={busy} onClick={() => setDraft({ ...role, allow: [...role.allow], deny: [...role.deny] })}>{role.managed ? 'Xem quyền' : 'Chỉnh sửa'}</button>{role.key !== 'owner' && <button className="ghost-button" disabled={busy} onClick={() => setDraft({ ...role, key: '', name: `Bản sao ${role.name}`, managed: false })}>Nhân bản</button>}
    {!role.managed && <button className="danger-button" disabled={busy} onClick={() => {
                        if (confirm(`Xóa vai trò “${role.name}”?`))
                            void perform(() => classAccessApi.removeRole(token!, classId, role.key), 'Đã xóa vai trò.');
                    }}>Xóa</button>}</div>
   </article>)}</div>
   {draft && <form ref={editorRef} className="role-editor settings-card" onSubmit={e => { e.preventDefault(); void perform(async () => { await classAccessApi.saveRole(token!, classId, draft, draft.key || undefined); setDraft(undefined); }, 'Đã lưu vai trò. Quyền mới áp dụng cho các thành viên được gán.'); }}>
    <div className="settings-section-title"><div><h3>{draft.managed ? `Quyền của ${draft.name}` : draft.key ? 'Chỉnh sửa vai trò' : 'Tạo vai trò mới'}</h3><p>Không chọn nghĩa là role này không cấp quyền đó.</p></div><button type="button" className="ghost-button" onClick={() => setDraft(undefined)}>Đóng</button></div>
    <div className="settings-form-grid"><label>Tên vai trò<input value={draft.name} maxLength={80} required disabled={draft.managed || busy} onChange={e => setDraft({ ...draft, name: e.target.value })}/></label><label>Mô tả<input value={draft.description} maxLength={500} disabled={draft.managed || busy} onChange={e => setDraft({ ...draft, description: e.target.value })}/></label></div>
    <div className="permission-matrix">{groups.map(group => <fieldset key={group}><legend>{group}</legend>{data.catalog.filter(a => a.group === group).map(action => <div className="permission-matrix-row" key={action.key}><div><strong>{action.name}</strong><code>{action.key}</code></div><select aria-label={`Quyền ${action.name}`} disabled={draft.managed || busy} value={draft.deny.includes(action.key) ? 'deny' : draft.allow.includes(action.key) ? 'allow' : 'none'} onChange={e => setDraft({ ...draft, allow: [...draft.allow.filter(k => k !== action.key), ...(e.target.value === 'allow' ? [action.key] : [])], deny: [...draft.deny.filter(k => k !== action.key), ...(e.target.value === 'deny' ? [action.key] : [])] })}><option value="none">Không cấp</option><option value="allow">Cho phép</option><option value="deny">Từ chối</option></select></div>)}</fieldset>)}</div>
    {!draft.managed && <div className="settings-row-actions"><button className="primary-button" disabled={busy}>{busy ? 'Đang lưu…' : 'Lưu vai trò'}</button><button type="button" className="ghost-button" disabled={busy} onClick={() => setDraft(undefined)}>Hủy</button></div>}
   </form>}
  </>}
  {section === 'members' && <div className="settings-card">
   <label>Chọn thành viên<select value={selectedUser} disabled={busy} onChange={e => { setSelectedUser(e.target.value); setKeys(data.members.find(m => String(m.userId) === e.target.value)?.roleKeys || []); setError(''); setSuccess(''); }}><option value="">Chọn người được gán vai trò</option>{data.members.map(m => <option key={m.userId} value={m.userId}>{m.name || `Thành viên #${m.userId}`}{m.email ? ` · ${m.email}` : ""}{m.legacyRole === 'owner' ? ' · Chủ lớp' : ''}{m.userId === user?.id ? ' · Bạn' : ''}</option>)}</select></label>
   {selected && <><h3>Vai trò của {selected.name || `thành viên #${selected.userId}`}</h3><p>{selected.legacyRole === 'owner' ? 'Chủ lớp luôn giữ toàn quyền.' : selected.userId === user?.id ? 'Bạn không thể tự thay đổi role của mình.' : 'Có thể chọn nhiều vai trò. Bỏ tất cả sẽ thu hồi các quyền trong lớp.'}</p>
    <div className="role-assignment-grid">{data.roles.filter(r => r.key !== 'owner').map(role => <label className="role-choice" key={role.key}><input type="checkbox" disabled={busy || selected.legacyRole === 'owner' || selected.userId === user?.id} checked={keys.includes(role.key)} onChange={e => setKeys(e.target.checked ? [...keys, role.key] : keys.filter(k => k !== role.key))}/><span><strong>{role.name}</strong><small>{role.description}</small></span></label>)}</div>
    <details className="effective-permissions"><summary>Quyền có hiệu lực ({selected.legacyRole === 'owner' ? data.catalog.length : actions.size})</summary><ul>{data.catalog.filter(a => selected.legacyRole === 'owner' || actions.has(a.key)).map(a => <li key={a.key}>{a.group}: {a.name}</li>)}</ul></details>
    {selected.legacyRole !== 'owner' && selected.userId !== user?.id && <button className="primary-button" disabled={busy} onClick={() => void perform(() => classAccessApi.assign(token!, classId, selected.userId, keys), 'Đã cập nhật vai trò cho thành viên.')}>{busy ? 'Đang lưu…' : 'Lưu phân quyền'}</button>}
   </>}
  </div>}
 </section>;
}
