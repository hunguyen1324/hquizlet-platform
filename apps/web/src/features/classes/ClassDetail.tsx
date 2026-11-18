// ClassDetail — Phase 7 [P7-FE-CLASS-03]
// Class detail page with Study Sets and Members tabs.
import { useEffect, useState } from "react";
import { MangaLibrary } from "../manga/MangaLibrary";
import { ClassAssignments } from "./ClassAssignments";
import "./class-settings.css";
import { ClassDocuments } from "./ClassDocuments";
import { ClassVideos } from "./ClassVideos";
import { useAuth } from "../auth/AuthContext";
import { classApi, memberApi, classStudySetApi, classAccessApi } from "../../lib/api";
import type { ClassDetail as ClassDetailType, ClassMember, ClassStudySet } from "../../types";
import "./class-detail.css";
import { ClassInviteQr } from "./ClassInviteQr";
const roleLabels = { owner: "Chủ nhóm", teacher: "Giáo viên", student: "Học viên" };
function TabIcon({ kind }: {
    kind: "videos" | "study-sets" | "members";
}) {
    return <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    {kind === "videos" ? <><rect x="3" y="4" width="18" height="16" rx="4"/><path d="m10 8 6 4-6 4z"/></> : kind === "study-sets" ? <><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20M6.5 3H20v19H6.5A2.5 2.5 0 0 1 4 19.5v-14A2.5 2.5 0 0 1 6.5 3Z"/><path d="M8 7h8M8 11h5"/></> : <><circle cx="9" cy="8" r="3"/><path d="M3 21v-3a6 6 0 0 1 12 0v3M16 5a3 3 0 0 1 0 6M21 21v-3a6 6 0 0 0-4-5.65"/></>}
  </svg>;
}
type Props = {
    classId: number;
    onBack: () => void;
    onEdit: (cls: ClassDetailType) => void;
    onDelete: () => void;
    onStartLive?: () => void;
    onOpenSet?: (id: number) => void;
    /** Phase 3: navigates to the class dashboard route. */
    onOpenDashboard?: () => void;
};
type Tab = "videos" | "upload" | "manga" | "study-sets" | "members" | "documents" | "assignments";
export function ClassDetail({ classId, onBack, onEdit, onDelete, onStartLive, onOpenSet, onOpenDashboard }: Props) {
    const { token } = useAuth();
    const [cls, setCls] = useState<ClassDetailType | null>(null);
    const [members, setMembers] = useState<ClassMember[]>([]);
    const [studySets, setStudySets] = useState<ClassStudySet[]>([]);
    const [tab, setTab] = useState<Tab>("study-sets");
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [actions, setActions] = useState<string[]>([]);
    const can = (action: string) => actions.includes(action);
    const isOwner = cls?.myRole === "owner";
    const isTeacher = cls?.myRole === "teacher" || isOwner;
    useEffect(() => {
        if (!token)
            return;
        loadData();
    }, [token, classId]);
    async function loadData() {
        if (!token)
            return;
        setLoading(true);
        try {
            setError(null);
            const [clsData, access] = await Promise.all([classApi.get(token, classId), classAccessApi.me(token, classId)]);
            setActions(access.actions);
            const [membersData, studySetsData] = await Promise.all([
                access.actions.includes("class-member:List") ? memberApi.list(token, classId) : Promise.resolve([]),
                access.actions.includes("class-content:ReadStudySet") ? classStudySetApi.list(token, classId) : Promise.resolve([])
            ]);
            const available: Tab[] = [];
            if (access.actions.includes("class-content:ViewVideo")) available.push("videos");
            if (access.actions.includes("class-content:UploadVideo")) available.push("upload");
            if (access.actions.includes("class-assignment:Read") || access.actions.includes("class-assignment:Create") || access.actions.includes("class-assignment:Update") || access.actions.includes("class-assignment:Delete") || access.actions.includes("class-assignment:Publish"))
                available.push("assignments");
            if (access.actions.includes("class-manga:Read")) available.push("manga");
            if (access.actions.includes("class-content:ReadStudySet"))
                available.push("study-sets");
            if (access.actions.includes("class-member:List"))
                available.push("members");
            if (access.actions.includes("class-document:Read"))
                available.push("documents");
            setTab(current => available.includes(current) ? current : available[0] || "study-sets");
            setCls(clsData);
            setMembers(membersData);
            setStudySets(studySetsData);
        }
        catch (err: any) {
            setError(err.message);
        }
        finally {
            setLoading(false);
        }
    }
    if (loading)
        return <div className="loading-overlay"><span>Đang tải lớp học...</span></div>;
    if (error)
        return <div className="error-state"><p>Lỗi: {error}</p><button onClick={onBack}>Quay lại</button></div>;
    if (!cls)
        return null;
    return (<div className="group-detail-page">
      <button className="ghost-button group-back" onClick={onBack}>← Nhóm học</button>
      <div className="group-detail-hero">
        <div className="group-avatar group-avatar-large" aria-hidden="true">
          {cls.name.trim().split(/\s+/).slice(0, 2).map((word) => word[0]).join("").toUpperCase()}
        </div>
        <div className="group-detail-copy">
          <p className="group-eyebrow">KHÔNG GIAN HỌC TẬP</p>
          <h1>{cls.name}</h1>
          <p>{cls.description || "Không gian học tập chung của nhóm"}</p>
          <div className="class-meta">
          <span className="role-badge" data-role={cls.myRole}>{roleLabels[cls.myRole]}</span>
          <span>{cls.memberCount} thành viên</span>
          <span>{cls.studySetCount} học phần</span>
          {can("class:ResetInvite") && <button className="group-code" onClick={() => void navigator.clipboard?.writeText(cls.inviteCode)} title="Sao chép mã mời">⌁ {cls.inviteCode}</button>}
          </div>
          {can("class:ResetInvite") && <ClassInviteQr inviteCode={cls.inviteCode} />}
        </div>
        <div className="class-detail-actions">
          {can("class:ViewDashboard") && onOpenDashboard && (<button className="ghost-button" onClick={onOpenDashboard}>Tổng quan</button>)}
          {can("live:Host") && onStartLive && <button className="primary-button" onClick={onStartLive}>▶ Chạy live</button>}
          {(can("class:Update") || can("class-access:Manage") || can("class-video:Update") || can("class-video:Delete") || can("class-video:ManageAudience")) && <button className="ghost-button" onClick={() => onEdit(cls)}>⚙ Cài đặt</button>}
          {can("class:Delete") && <button className="danger-button" onClick={onDelete}>Xóa nhóm</button>}
          {!isOwner && (<button className="ghost-button" onClick={async () => {
                if (token && confirm("Bạn có chắc muốn rời lớp?")) {
                    await memberApi.leave(token, classId);
                    onBack();
                }
            }}>Rời lớp</button>)}
        </div>
      </div>

      <div className="class-tabs" aria-label="Nội dung nhóm">
        {can("class-content:ViewVideo") && <button aria-pressed={tab === "videos"} className={tab === "videos" ? "active" : ""} onClick={() => setTab("videos")}><TabIcon kind="videos"/>Video</button>}
        {can("class-content:UploadVideo") && <button aria-pressed={tab === "upload"} className={tab === "upload" ? "active" : ""} onClick={() => setTab("upload")}>↑ Tải video</button>}
        {can("class-manga:Read") && <button aria-pressed={tab === "manga"} className={tab === "manga" ? "active" : ""} onClick={() => setTab("manga")}>Truyện</button>}
        {can("class-content:ReadStudySet") && <button aria-pressed={tab === "study-sets"} className={tab === "study-sets" ? "active" : ""} onClick={() => setTab("study-sets")}>
          <TabIcon kind="study-sets"/>Học phần <span className="class-tab-count">{studySets.length}</span>
        </button>}
        {can("class-member:List") && <button aria-pressed={tab === "members"} className={tab === "members" ? "active" : ""} onClick={() => setTab("members")}>
          <TabIcon kind="members"/>Thành viên <span className="class-tab-count">{members.length}</span>
        </button>}
        {can("class-document:Read") && <button aria-pressed={tab === "documents"} className={tab === "documents" ? "active" : ""} onClick={() => setTab("documents")}><TabIcon kind="study-sets"/>Tài liệu</button>}
        {(actions.some(a => a.startsWith("class-assignment:"))) && <button aria-pressed={tab === "assignments"} className={tab === "assignments" ? "active" : ""} onClick={() => setTab("assignments")}><TabIcon kind="study-sets"/>Bài tập</button>}

      </div>

      {((tab === "videos" && can("class-content:ViewVideo")) || (tab === "upload" && can("class-content:UploadVideo"))) && <ClassVideos key={classId} classId={classId} members={members} canManage={can("class-content:UploadVideo")} canManageAudience={can("class-video:ManageAudience")} canSetCategory={can("class-video:Update")} canEditSubtitles={can("class-video:Update")} mode={tab as "videos" | "upload"}/>}
      {tab === "manga" && can("class-manga:Read") && <MangaLibrary key={classId} classId={classId} canCreate={can("class-manga:Create")} />}
      {tab === "documents" && can("class-document:Read") && <ClassDocuments key={classId} classId={classId} canManage={can("class-document:Create")} canDelete={can("class-document:Delete")} canUpdate={can("class-document:Update")}/>}
      {tab === "assignments" && <ClassAssignments key={classId} classId={classId} actions={actions}/>}
      {tab === "study-sets" && (<div className="tab-content">
          <div className="class-section-heading"><div><h2>Học phần của nhóm</h2><p>Tập hợp kiến thức, cùng nhau tiến bộ mỗi ngày.</p></div><span className="class-section-icon"><TabIcon kind="study-sets"/></span></div>
          {can("class-content:AssignStudySet") && (<AddStudySetForm classId={classId} onAdded={loadData}/>)}
          {studySets.length === 0 ? (<div className="empty-state"><span className="class-empty-icon"><TabIcon kind="study-sets"/></span><h3>Khởi đầu từ một học phần</h3><p>{isTeacher ? "Thêm học phần đầu tiên để cả nhóm cùng ôn tập và chinh phục mục tiêu." : "Tài liệu của nhóm sẽ xuất hiện ở đây khi được chia sẻ."}</p></div>) : (<ul className="study-set-list group-resource-list">
              {studySets.map((ss) => (<li key={ss.studySetId} className="study-set-item group-resource-item">
                  <button className="group-resource-main" onClick={() => onOpenSet?.(ss.studySetId)}>
                    <span className="group-resource-icon">▤</span>
                    <span>
                    <strong>{ss.title || `Học phần #${ss.studySetId}`}</strong>
                    <small>{ss.flashcardCount != null ? `${ss.flashcardCount} thẻ` : "Bộ tài liệu"}</small>
                    </span>
                  </button>
                  {can("class-content:UnassignStudySet") && (<button className="ghost-button danger-button small" onClick={async () => {
                            if (token && confirm("Xóa học phần khỏi lớp?")) {
                                await classStudySetApi.remove(token, classId, ss.studySetId);
                                loadData();
                            }
                        }}>Xóa</button>)}
                </li>))}
            </ul>)}
        </div>)}

      {tab === "members" && (<div className="tab-content">
          <div className="class-section-heading"><div><h2>Đồng đội học tập</h2><p>{members.length} thành viên cùng chia sẻ và khám phá kiến thức.</p></div><span className="class-section-icon"><TabIcon kind="members"/></span></div>
          {can("class-member:Add") && (<AddMemberForm classId={classId} onAdded={loadData} canAssignTeacher={can("class-member:UpdateRole")}/>)}
          <ul className="member-list">
            {members.map((m) => (<li key={m.userId} className="member-item">
                <span className="class-member-avatar" data-role={m.role} aria-hidden="true">{String(m.userId).padStart(2, "0")}</span>
                <div className="class-member-copy"><strong>{m.name || `Thành viên #${m.userId}`}</strong><span className="member-joined">Tham gia {new Date(m.joinedAt).toLocaleDateString("vi-VN")}</span></div>
                <span className="role-badge" data-role={m.role}>{roleLabels[m.role]}</span>
                {can("class-member:Remove") && m.role !== "owner" && <button className="danger-button" onClick={async () => {
                        if (token && confirm("Xóa thành viên khỏi lớp?")) {
                            try {
                                await memberApi.remove(token, classId, m.userId);
                                await loadData();
                            }
                            catch (e) {
                                setError(e instanceof Error ? e.message : "Không xóa được thành viên");
                            }
                        }
                    }}>Xóa</button>}

              </li>))}
          </ul>
        </div>)}
    </div>);
}
// --- Inline sub-forms ---
function AddStudySetForm({ classId, onAdded }: {
    classId: number;
    onAdded: () => void;
}) {
    const { token } = useAuth();
    const [studySetId, setStudySetId] = useState("");
    const [dueAt, setDueAt] = useState("");
    const [error, setError] = useState<string | null>(null);
    async function handleAdd() {
        if (!token || !studySetId.trim())
            return;
        const id = parseInt(studySetId, 10);
        if (isNaN(id) || id <= 0) {
            setError("ID học phần không hợp lệ");
            return;
        }
        try {
            await classStudySetApi.add(token, classId, id, dueAt ? new Date(dueAt).toISOString() : undefined);
            setStudySetId("");
            setError(null);
            onAdded();
        }
        catch (err: any) {
            setError(err.message);
        }
    }
    return (<div className="class-compose">
      <div className="class-compose-title"><h3>Chia sẻ học phần</h3><p>Thêm học phần vào thư viện chung của nhóm.</p></div>
      <div className="inline-form">
      <label>ID học phần<input type="number" min="1" value={studySetId} onChange={(e) => setStudySetId(e.target.value)} placeholder="Nhập ID học phần"/></label>
      <label>Hạn làm bài <span>(tuỳ chọn)</span><input type="datetime-local" value={dueAt} onChange={e => setDueAt(e.target.value)}/></label><button className="primary-button small" onClick={handleAdd}>+ Thêm học phần</button>
      {error && <span className="error-inline">{error}</span>}
      </div>
    </div>);
}
function AddMemberForm({ classId, onAdded, canAssignTeacher }: {
    classId: number;
    onAdded: () => void;
    canAssignTeacher: boolean;
}) {
    const { token } = useAuth();
    const [userId, setUserId] = useState("");
    const [role, setRole] = useState("student");
    const [error, setError] = useState<string | null>(null);
    async function handleAdd() {
        if (!token || !userId.trim())
            return;
        const id = parseInt(userId, 10);
        if (isNaN(id) || id <= 0) {
            setError("User ID không hợp lệ");
            return;
        }
        try {
            await memberApi.add(token, classId, { userId: id, role });
            setUserId("");
            setError(null);
            onAdded();
        }
        catch (err: any) {
            setError(err.message);
        }
    }
    return (<div className="class-compose">
      <div className="class-compose-title"><h3>Mở rộng nhóm học</h3><p>Mời thêm đồng đội và phân vai trò phù hợp.</p></div>
      <div className="inline-form">
      <label>ID thành viên<input type="number" min="1" value={userId} onChange={(e) => setUserId(e.target.value)} placeholder="Nhập ID người dùng"/></label>
      <label>Vai trò<select value={role} onChange={(e) => setRole(e.target.value)}>
        <option value="student">Học viên</option>
        {canAssignTeacher && <option value="teacher">Giáo viên</option>}
      </select></label>
      <button className="primary-button small" onClick={handleAdd}>+ Thêm thành viên</button>
      {error && <span className="error-inline">{error}</span>}
      </div>
    </div>);
}
