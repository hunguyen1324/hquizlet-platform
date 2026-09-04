// ClassDetail — Phase 7 [P7-FE-CLASS-03]
// Class detail page with Study Sets and Members tabs.

import { useEffect, useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { classApi, memberApi, classStudySetApi } from "../../lib/api";
import type { ClassDetail as ClassDetailType, ClassMember, ClassStudySet } from "../../types";

type Props = {
  classId: number;
  onBack: () => void;
  onEdit: (cls: ClassDetailType) => void;
  onDelete: () => void;
  onStartLive?: () => void;
  onOpenSet?: (id: number) => void;
};

type Tab = "study-sets" | "members";

export function ClassDetail({ classId, onBack, onEdit, onDelete, onStartLive, onOpenSet }: Props) {
  const { token, user } = useAuth();
  const [cls, setCls] = useState<ClassDetailType | null>(null);
  const [members, setMembers] = useState<ClassMember[]>([]);
  const [studySets, setStudySets] = useState<ClassStudySet[]>([]);
  const [tab, setTab] = useState<Tab>("study-sets");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const isOwner = cls?.myRole === "owner";
  const isTeacher = cls?.myRole === "teacher" || isOwner;

  useEffect(() => {
    if (!token) return;
    loadData();
  }, [token, classId]);

  async function loadData() {
    if (!token) return;
    setLoading(true);
    try {
      const [clsData, membersData, studySetsData] = await Promise.all([
        classApi.get(token, classId),
        memberApi.list(token, classId),
        classStudySetApi.list(token, classId),
      ]);
      setCls(clsData);
      setMembers(membersData);
      setStudySets(studySetsData);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  if (loading) return <div className="loading-overlay"><span>Đang tải lớp học...</span></div>;
  if (error) return <div className="error-state"><p>Lỗi: {error}</p><button onClick={onBack}>Quay lại</button></div>;
  if (!cls) return null;

  return (
    <div className="group-detail-page">
      <button className="ghost-button group-back" onClick={onBack}>← Nhóm học</button>
      <div className="group-detail-hero">
        <div className="group-avatar group-avatar-large" aria-hidden="true">
          {cls.name.trim().split(/\s+/).slice(0, 2).map((word) => word[0]).join("").toUpperCase()}
        </div>
        <div className="group-detail-copy">
          <h1>{cls.name}</h1>
          <p>{cls.description || "Không gian học tập chung của nhóm"}</p>
          <div className="class-meta">
          <span className="role-badge" data-role={cls.myRole}>{cls.myRole}</span>
          <span>{cls.memberCount} thành viên</span>
          <span>{cls.studySetCount} tài liệu</span>
          {isOwner && <button className="group-code" onClick={() => void navigator.clipboard?.writeText(cls.inviteCode)} title="Sao chép mã mời">⌁ {cls.inviteCode}</button>}
          </div>
        </div>
        <div className="class-detail-actions">
          {isTeacher && onStartLive && <button className="primary-button" onClick={onStartLive}>▶ Chạy live</button>}
          {isOwner && (
            <>
              <button className="ghost-button" onClick={() => onEdit(cls)}>Cài đặt</button>
              <button className="danger-button" onClick={onDelete}>Xóa nhóm</button>
            </>
          )}
          {!isOwner && (
            <button className="ghost-button" onClick={async () => {
              if (token && confirm("Bạn có chắc muốn rời lớp?")) {
                await memberApi.leave(token, classId);
                onBack();
              }
            }}>Rời lớp</button>
          )}
        </div>
      </div>

      <div className="class-tabs">
        <button className={tab === "study-sets" ? "active" : ""} onClick={() => setTab("study-sets")}>
          Tài liệu ({studySets.length})
        </button>
        <button className={tab === "members" ? "active" : ""} onClick={() => setTab("members")}>
          Thành viên ({members.length})
        </button>
      </div>

      {tab === "study-sets" && (
        <div className="tab-content">
          {isTeacher && (
            <AddStudySetForm classId={classId} onAdded={loadData} />
          )}
          {studySets.length === 0 ? (
            <div className="empty-state"><p>Chưa có học phần nào trong lớp.</p></div>
          ) : (
            <ul className="study-set-list group-resource-list">
              {studySets.map((ss) => (
                <li key={ss.studySetId} className="study-set-item group-resource-item">
                  <button className="group-resource-main" onClick={() => onOpenSet?.(ss.studySetId)}>
                    <span className="group-resource-icon">▤</span>
                    <span>
                    <strong>{ss.title || `Học phần #${ss.studySetId}`}</strong>
                    <small>{ss.flashcardCount != null ? `${ss.flashcardCount} thẻ` : "Bộ tài liệu"}</small>
                    </span>
                  </button>
                  {isTeacher && (
                    <button className="ghost-button danger-button small" onClick={async () => {
                      if (token && confirm("Xóa học phần khỏi lớp?")) {
                        await classStudySetApi.remove(token, classId, ss.studySetId);
                        loadData();
                      }
                    }}>Xóa</button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {tab === "members" && (
        <div className="tab-content">
          {isTeacher && (
            <AddMemberForm classId={classId} onAdded={loadData} />
          )}
          <ul className="member-list">
            {members.map((m) => (
              <li key={m.userId} className="member-item">
                <span className="role-badge" data-role={m.role}>{m.role}</span>
                <span>User #{m.userId}</span>
                <span className="member-joined">tham gia {new Date(m.joinedAt).toLocaleDateString()}</span>
                {isOwner && m.role !== "owner" && (
                  <div className="member-actions">
                    {m.role === "student" && (
                      <button className="ghost-button small" onClick={async () => {
                        if (token) {
                          await memberApi.updateRole(token, classId, m.userId, "teacher");
                          loadData();
                        }
                      }}>Thăng teacher</button>
                    )}
                    {m.role === "teacher" && (
                      <button className="ghost-button small" onClick={async () => {
                        if (token) {
                          await memberApi.updateRole(token, classId, m.userId, "student");
                          loadData();
                        }
                      }}>Hạ student</button>
                    )}
                    <button className="ghost-button danger-button small" onClick={async () => {
                      if (token && confirm("Xóa thành viên khỏi lớp?")) {
                        await memberApi.remove(token, classId, m.userId);
                        loadData();
                      }
                    }}>Xóa</button>
                  </div>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

// --- Inline sub-forms ---

function AddStudySetForm({ classId, onAdded }: { classId: number; onAdded: () => void }) {
  const { token } = useAuth();
  const [studySetId, setStudySetId] = useState("");
  const [error, setError] = useState<string | null>(null);

  async function handleAdd() {
    if (!token || !studySetId.trim()) return;
    const id = parseInt(studySetId, 10);
    if (isNaN(id) || id <= 0) {
      setError("ID học phần không hợp lệ");
      return;
    }
    try {
      await classStudySetApi.add(token, classId, id);
      setStudySetId("");
      setError(null);
      onAdded();
    } catch (err: any) {
      setError(err.message);
    }
  }

  return (
    <div className="inline-form">
      <input type="number" value={studySetId} onChange={(e) => setStudySetId(e.target.value)} placeholder="ID học phần" />
      <button className="primary-button small" onClick={handleAdd}>Thêm học phần</button>
      {error && <span className="error-inline">{error}</span>}
    </div>
  );
}

function AddMemberForm({ classId, onAdded }: { classId: number; onAdded: () => void }) {
  const { token } = useAuth();
  const [userId, setUserId] = useState("");
  const [role, setRole] = useState("student");
  const [error, setError] = useState<string | null>(null);

  async function handleAdd() {
    if (!token || !userId.trim()) return;
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
    } catch (err: any) {
      setError(err.message);
    }
  }

  return (
    <div className="inline-form">
      <input type="number" value={userId} onChange={(e) => setUserId(e.target.value)} placeholder="User ID" />
      <select value={role} onChange={(e) => setRole(e.target.value)}>
        <option value="student">Student</option>
        <option value="teacher">Teacher</option>
      </select>
      <button className="primary-button small" onClick={handleAdd}>Thêm thành viên</button>
      {error && <span className="error-inline">{error}</span>}
    </div>
  );
}
