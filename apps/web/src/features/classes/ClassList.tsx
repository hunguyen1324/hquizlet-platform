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
    <div className="group-page">
      <div className="group-page-header">
        <div>
          <p className="group-eyebrow">Không gian cộng tác</p>
          <h1>Nhóm học</h1>
          <p>Học bằng flashcard, thi quiz và theo dõi tiến độ cùng nhau.</p>
        </div>
        <div className="group-header-actions">
          <button className="ghost-button" onClick={onJoin}>⌁ Tham gia bằng mã</button>
          <button className="primary-button" onClick={onCreate}>＋ Tạo nhóm</button>
        </div>
      </div>

      <div className="group-filterbar" aria-label="Bộ lọc nhóm học">
        <button className="active">Nhóm của tôi</button>
        <span>{classes.length} nhóm</span>
      </div>

      {classes.length === 0 ? (
        <div className="empty-state">
          <p>Bạn chưa có lớp học nào.</p>
          <p>Tạo lớp mới hoặc tham gia lớp bằng mã mời!</p>
        </div>
      ) : (
        <div className="group-card-list">
          {classes.map((cls) => (
            <button key={cls.id} className="group-card" onClick={() => onSelect(cls.id)}>
              <span className="group-avatar" aria-hidden="true">
                {cls.name.trim().split(/\s+/).slice(0, 2).map((word) => word[0]).join("").toUpperCase()}
              </span>
              <span className="group-card-copy">
                <span className="group-card-title-row">
                  <strong>{cls.name}</strong>
                  <span className="role-badge" data-role={cls.myRole}>{cls.myRole}</span>
                </span>
                <span className="group-card-description">{cls.description || "Không gian học tập chung của nhóm"}</span>
                <span className="group-card-stats">
                  <span>♙ {cls.memberCount} thành viên</span>
                  <span>▤ {cls.studySetCount} tài liệu</span>
                  {cls.myRole === "owner" && <span className="group-code">{cls.inviteCode}</span>}
                </span>
              </span>
              <span className="group-card-arrow" aria-hidden="true">→</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
