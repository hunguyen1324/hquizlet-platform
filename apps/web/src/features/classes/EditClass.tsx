import { useEffect, useState } from 'react';
import { useAuth } from '../auth/AuthContext';
import { classApi, classAccessApi, memberApi } from '../../lib/api';
import type { ClassDetail, ClassMember } from '../../types';
import { ClassInviteQr } from './ClassInviteQr';
import { ClassRoles } from './ClassRoles';
import { ClassVideos } from './ClassVideos';
import './class-detail.css';
import './class-settings.css';
type Props = {
    cls: ClassDetail;
    onSaved: (cls: ClassDetail) => void;
    onCancel: () => void;
};
export function EditClass({ cls, onSaved, onCancel }: Props) {
    const { token } = useAuth();
    const [inviteCode, setInviteCode] = useState(cls.inviteCode);
    const [name, setName] = useState(cls.name);
    const [description, setDescription] = useState(cls.description || '');
    const [actions, setActions] = useState<string[]>();
    const [members, setMembers] = useState<ClassMember[]>([]);
    const [section, setSection] = useState('general');
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const [success, setSuccess] = useState('');
    const can = (action: string) => actions?.includes(action) ?? false;
    useEffect(() => {
        let active = true;
        if (token)
            classAccessApi.me(token, cls.id).then(async (d) => {
                if (!active)
                    return;
                setActions(d.actions);
                if (!d.actions.includes('class:Update'))
                    setSection(d.actions.includes('class-access:Manage') ? 'roles' : 'videos');
                if (d.actions.includes('class-member:List')) {
                    const m = await memberApi.list(token, cls.id);
                    if (active)
                        setMembers(m);
                }
            }).catch(e => {
                if (active)
                    setError(e.message);
            });
        return () => { active = false; };
    }, [token, cls.id]);
    return <div className="group-detail-page class-settings-page">
  <button className="ghost-button group-back" onClick={onCancel}>← Quay lại lớp học</button>
  <header className="settings-header"><div><p className="group-eyebrow">QUẢN TRỊ KHÔNG GIAN HỌC TẬP</p><h1>Cài đặt lớp học</h1><p>{cls.name} · Quản lý thông tin, vai trò và quyền truy cập.</p></div><span className="settings-header-icon" aria-hidden="true">⚙</span></header>
  {error && <p className="error-message" role="alert">{error}</p>}{success && <p className="settings-success" role="status">{success}</p>}
  {!actions && !error ? <p role="status">Đang tải cài đặt…</p> : <div className="settings-layout">
   <nav className="settings-navigation" aria-label="Cài đặt lớp">
    {can('class:Update') && <button className={section === 'general' ? 'active' : ''} onClick={() => setSection('general')}><span aria-hidden="true">▤</span>Thông tin chung<small>Tên và mô tả lớp học</small></button>}
    {can('class-access:Manage') && <button className={section === 'roles' ? 'active' : ''} onClick={() => setSection('roles')}><span aria-hidden="true">♧</span>Vai trò & phân quyền<small>Quyền thao tác và gán thành viên</small></button>}
    {(can('class-video:Update') || can('class-video:Delete') || can('class-video:ManageAudience')) && <button className={section === 'videos' ? 'active' : ''} onClick={() => setSection('videos')}><span aria-hidden="true">▷</span>Quản lý video<small>Nội dung và người được xem</small></button>}
   </nav>
   <main className="settings-main">
    {section === 'general' && can('class:Update') && <form className="settings-card" onSubmit={async (e) => {
                    e.preventDefault();
                    if (!token || busy)
                        return;
                    setBusy(true);
                    setError('');
                    try {
                        const updated = await classApi.update(token, cls.id, { name: name.trim(), description: description.trim() });
                        onSaved({ ...cls, ...updated });
                    }
                    catch (e) {
                        setError(e instanceof Error ? e.message : 'Không lưu được cài đặt');
                    }
                    finally {
                        setBusy(false);
                    }
                }}>
     <div className="settings-section-title"><div><h2>Thông tin chung</h2><p>Giúp thành viên nhận biết lớp và mục tiêu học tập.</p></div></div>
     <label>Tên lớp *<input required maxLength={120} value={name} disabled={busy} onChange={e => setName(e.target.value)}/></label>
     <label>Mô tả<textarea rows={5} maxLength={5000} value={description} disabled={busy} onChange={e => setDescription(e.target.value)} placeholder="Mô tả mục tiêu, nội dung và cách học của lớp…"/></label>
     <div className="settings-row-actions"><button className="primary-button" disabled={busy || !name.trim()}>{busy ? 'Đang lưu…' : 'Lưu thay đổi'}</button><button type="button" className="ghost-button" disabled={busy} onClick={onCancel}>Hủy</button></div>
     {can('class:ResetInvite') && <div className="settings-invite"><h3>Mã tham gia lớp</h3><p>Mã hiện tại: <strong>{inviteCode}</strong></p><ClassInviteQr inviteCode={inviteCode}/><button type="button" className="ghost-button" disabled={busy} onClick={async () => {
                        if (!token || !confirm('Đổi mã mời? Mã cũ sẽ không còn dùng được.'))
                            return;
                        setBusy(true);
                        setError('');
                        try {
                            const d = await classApi.resetInviteCode(token, cls.id);
                            setInviteCode(d.inviteCode);
                            setSuccess(`Mã tham gia mới: ${d.inviteCode}`);
                        }
                        catch (e) {
                            setError(e instanceof Error ? e.message : 'Không đổi được mã');
                        }
                        finally {
                            setBusy(false);
                        }
                    }}>Tạo mã mời mới</button></div>}
    </form>}
    {section === 'roles' && can('class-access:Manage') && <ClassRoles classId={cls.id}/>}
    {section === 'videos' && (can('class-video:Update') || can('class-video:Delete') || can('class-video:ManageAudience')) && <ClassVideos classId={cls.id} members={members} canManage={can('class-video:Update')} canDelete={can('class-video:Delete')} canManageAudience={can('class-video:ManageAudience')} mode="permissions"/>}
   </main>
  </div>}
 </div>;
}
