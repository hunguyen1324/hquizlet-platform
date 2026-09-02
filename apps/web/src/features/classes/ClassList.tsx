// ClassList — Phase 7 [P7-FE-CLASS-01]
// Displays list of classes the user is a member of.

import { useEffect, useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { classApi } from "../../lib/api";
import type { ClassSummary } from "../../types";

type Props = {
  onCreate: () => void;
  onJoin: () => void;
  onSelect: (id: number) => void;
};

export function ClassList({ onCreate, onJoin, onSelect }: Props) {
  const { token } = useAuth();
  const [classes, setClasses] = useState<ClassSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    setLoading(true);
    classApi
      .list(token)
      .then((data) => setClasses(data))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [token]);

  if (loading) return <div className="loading-overlay"><span>Đang tải danh sách lớp...</span></div>;
  if (error) return <div className="error-state"><p>Lỗi: {error}</p></div>;

  return (
    <div className="class-list-page">
      <div className="class-list-header">
        <h1>Lớp học</h1>
        <div className="class-actions">
          <button className="primary-button" onClick={onCreate}>Tạo lớp mới</button>
          <button className="ghost-button" onClick={onJoin}>Tham gia lớp</button>
        </div>
      </div>

      {classes.length === 0 ? (
        <div className="empty-state">
          <p>Bạn chưa có lớp học nào.</p>
          <p>Tạo lớp mới hoặc tham gia lớp bằng mã mời!</p>
        </div>
      ) : (
        <div className="class-cards">
          {classes.map((cls) => (
            <div key={cls.id} className="class-card" onClick={() => onSelect(cls.id)}>
              <h3>{cls.name}</h3>
              {cls.description && <p className="class-desc">{cls.description}</p>}
              <div className="class-meta">
                <span className="role-badge" data-role={cls.myRole}>{cls.myRole}</span>
                <span>{cls.memberCount} thành viên</span>
                <span>{cls.studySetCount} học phần</span>
              </div>
              {cls.myRole === "owner" && (
                <div className="class-invite-code">
                  <small>Mã mời: {cls.inviteCode}</small>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
