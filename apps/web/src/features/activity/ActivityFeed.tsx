// ActivityFeed — Phase 7 [P7-FE-ACT-01]
// Activity feed page showing recent activity across classes and study progress.

import { useEffect, useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { activityApi } from "../../lib/api";
import type { ActivityItem, ActivityFeedResponse } from "../../types";

type Props = {
  onBack: () => void;
};

const EVENT_ICONS: Record<string, string> = {
  "class.created": "📚",
  "class.updated": "✏️",
  "class.deleted": "🗑️",
  "class.member.joined": "👋",
  "class.member.added": "➕",
  "class.member.removed": "➖",
  "class.member.left": "🚪",
  "class.studyset.added": "📖",
  "class.studyset.removed": "📕",
  "study.progress": "🎯",
};

const EVENT_DESCRIPTIONS: Record<string, string> = {
  "class.created": "tạo lớp",
  "class.updated": "cập nhật lớp",
  "class.deleted": "xóa lớp",
  "class.member.joined": "tham gia lớp",
  "class.member.added": "được thêm vào lớp",
  "class.member.removed": "bị xóa khỏi lớp",
  "class.member.left": "rời lớp",
  "class.studyset.added": "thêm học phần vào lớp",
  "class.studyset.removed": "xóa học phần khỏi lớp",
  "study.progress": "hoàn thành bài học",
};

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "vừa xong";
  if (mins < 60) return `${mins} phút trước`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours} giờ trước`;
  const days = Math.floor(hours / 24);
  return `${days} ngày trước`;
}

export function ActivityFeed({ onBack }: Props) {
  const { token } = useAuth();
  const [items, setItems] = useState<ActivityItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);
  const [hasMore, setHasMore] = useState(false);

  useEffect(() => {
    loadFeed();
  }, [token]);

  async function loadFeed(cursor?: string) {
    if (!token) return;
    setLoading(!!cursor === false);
    setLoadingMore(!!cursor);
    try {
      const resp: ActivityFeedResponse = await activityApi.getFeed(token, cursor, 20);
      if (cursor) {
        setItems((prev) => [...prev, ...resp.items]);
      } else {
        setItems(resp.items);
      }
      setNextCursor(resp.nextCursor);
      setHasMore(resp.hasMore);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  }

  return (
    <div className="activity-feed-page">
      <div className="activity-header">
        <button className="ghost-button" onClick={onBack}>← Quay lại</button>
        <h1>Hoạt động gần đây</h1>
      </div>

      {loading && <div className="loading-overlay"><span>Đang tải...</span></div>}

      {error && <div className="error-state"><p>Lỗi: {error}</p></div>}

      {!loading && items.length === 0 && (
        <div className="empty-state">
          <p>Chưa có hoạt động nào.</p>
          <p>Bắt đầu học hoặc tham gia một lớp!</p>
        </div>
      )}

      <ul className="activity-list">
        {items.map((item) => {
          const meta = item.metadata as Record<string, any> | undefined;
          const className = meta?.className;
          const description = EVENT_DESCRIPTIONS[item.eventType] || item.eventType;
          const icon = EVENT_ICONS[item.eventType] || "📌";

          let text = description;
          if (className) text += ` "${className}"`;
          if (item.eventType === "study.progress" && meta) {
            text = `Hoàn thành ${meta.mode} — ${meta.score}/${meta.total}`;
          }

          return (
            <li key={item.id} className="activity-item">
              <span className="activity-icon">{icon}</span>
              <div className="activity-content">
                <span>{text}</span>
                <small>{timeAgo(item.occurredAt)}</small>
              </div>
            </li>
          );
        })}
      </ul>

      {hasMore && (
        <div className="load-more">
          <button className="ghost-button" onClick={() => loadFeed(nextCursor)} disabled={loadingMore}>
            {loadingMore ? "Đang tải..." : "Tải thêm"}
          </button>
        </div>
      )}
    </div>
  );
}
