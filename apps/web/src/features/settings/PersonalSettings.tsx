import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { studySetApi } from "../../lib/api";
import { mediaApi, type MediaItem } from "../../lib/api/media";
import type { StudySet, Visibility } from "../../types";
import {
  personalContentLabels,
  readVisibilityDefault,
  saveVisibilityDefault,
  type PersonalContentKind,
} from "./visibilityDefaults";
import "./personal-settings.css";

const pageSize = 20;
const kinds: PersonalContentKind[] = ["study", "post", "video"];
const errorText = (error: unknown) =>
  error instanceof Error ? error.message : "Không thực hiện được thao tác.";

function VisibilityOptions() {
  return <><option value="public">Công khai</option><option value="private">Riêng tư</option></>;
}

function ContentPermission({ item, token }: { item: StudySet | MediaItem; token: string }) {
  const client = useQueryClient();
  const [visibility, setVisibility] = useState(item.visibility);
  const [viewers, setViewers] = useState("viewerIds" in item ? item.viewerIds.join(", ") : "");
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");

  return (
    <form className="personal-permission-row" onSubmit={async (event) => {
      event.preventDefault();
      if (busy) return;
      setBusy(true);
      setError("");
      setStatus("");
      try {
        if ("kind" in item) {
          const viewerIds = visibility === "selected" ? viewers.trim().split(/[\s,]+/).map(Number) : [];
          if (viewerIds.some(id => !Number.isSafeInteger(id) || id < 1)) {
            throw new Error("Nhập các ID người xem hợp lệ, cách nhau bằng dấu phẩy.");
          }
          await mediaApi.save(token, {
            kind: item.kind, title: item.title, body: item.body,
            visibility, viewerIds: [...new Set(viewerIds)],
          }, item.id);
        } else {
          await studySetApi.update(token, item.id, {
            title: item.title, description: item.description,
            termLanguage: item.termLanguage, definitionLanguage: item.definitionLanguage, visibility,
          });
        }
        setStatus("Đã lưu quyền xem.");
        const queryKeys = "kind" in item
          ? ["media", "posts-feed", "post-detail"]
          : ["studySets"];
        await Promise.all(["personal-permissions", ...queryKeys].map(key =>
          client.invalidateQueries({ queryKey: [key] }),
        ));
      } catch (err) {
        setError(errorText(err));
      } finally {
        setBusy(false);
      }
    }}>
      <strong>{item.title}</strong>
      <select aria-label={`Quyền xem của ${item.title}`} value={visibility} disabled={busy} onChange={event => {
        setVisibility(event.target.value as MediaItem["visibility"]);
        setStatus("");
      }}>
        <VisibilityOptions />
        {"kind" in item && <option value="selected">Người được chọn</option>}
      </select>
      {visibility === "selected" && (
        <input aria-label={`ID người xem của ${item.title}`} placeholder="ID người xem, ví dụ 12, 34"
          value={viewers} onChange={event => { setViewers(event.target.value); setStatus(""); }} disabled={busy} required />
      )}
      <button type="submit" className="ghost-button" disabled={busy}>{busy ? "Đang lưu…" : "Lưu quyền"}</button>
      {status && <span role="status">{status}</span>}
      {error && <span role="alert">{error}</span>}
    </form>
  );
}

export function PersonalSettings({ userId, token }: { userId: number; token: string }) {
  const [kind, setKind] = useState<PersonalContentKind>("study");
  const [page, setPage] = useState(1);
  const [notice, setNotice] = useState("");
  const [defaults, setDefaults] = useState(() => ({
    study: readVisibilityDefault(userId, "study"),
    post: readVisibilityDefault(userId, "post"),
    video: readVisibilityDefault(userId, "video"),
  }));
  const list = useQuery({
    queryKey: ["personal-permissions", userId, kind, page],
    queryFn: async ({ signal }) => {
      if (kind === "study") {
        const results = await studySetApi.list(token, { page, per_page: pageSize, sort: "created" }, signal);
        return { items: results.items, hasNext: page < results.totalPages };
      }
      const results = await mediaApi.ownContent(token, kind, (page - 1) * pageSize, pageSize + 1, signal);
      return { items: results.slice(0, pageSize), hasNext: results.length > pageSize };
    },
  });

  return (
    <div className="workspace-page personal-settings">
      <header><h1>Cài đặt cá nhân</h1></header>
      <section className="personal-permission-panel">
        <h2>Phân quyền mặc định</h2>
        <p>Áp dụng khi tạo nội dung mới trên trình duyệt này.</p>
        <div className="personal-permission-defaults">
          {kinds.map(type => (
            <label key={type}>{personalContentLabels[type]}
              <select value={defaults[type]} onChange={event => {
                const visibility = event.target.value as Visibility;
                try {
                  saveVisibilityDefault(userId, type, visibility);
                  setDefaults(current => ({ ...current, [type]: visibility }));
                  setNotice(`Đã lưu quyền mặc định cho ${personalContentLabels[type].toLowerCase()}.`);
                } catch {
                  setNotice("Trình duyệt không lưu được quyền mặc định.");
                }
              }}>
                <VisibilityOptions />
              </select>
            </label>
          ))}
        </div>
        {notice && <p role="status">{notice}</p>}
      </section>
      <section className="personal-permission-panel">
        <h2>Quyền xem từng nội dung</h2>
        <label>Loại nội dung
          <select value={kind} onChange={event => { setKind(event.target.value as PersonalContentKind); setPage(1); }}>
            {kinds.map(type => <option key={type} value={type}>{personalContentLabels[type]}</option>)}
          </select>
        </label>
        {list.isPending && <p role="status">Đang tải nội dung…</p>}
        {list.isError && <p role="alert">{errorText(list.error)} <button className="ghost-button" onClick={() => void list.refetch()}>Thử lại</button></p>}
        {list.data?.items.length === 0 && <p>Chưa có nội dung ở trang này.</p>}
        {list.data?.items.map(item => <ContentPermission key={`${kind}:${item.id}`} item={item} token={token} />)}
        {(page > 1 || list.data?.hasNext) && <div className="personal-permission-pagination">
          <button className="ghost-button" disabled={page === 1} onClick={() => setPage(page - 1)}>Trang trước</button>
          <span>Trang {page}</span>
          <button className="ghost-button" disabled={!list.data?.hasNext} onClick={() => setPage(page + 1)}>Trang sau</button>
        </div>}
      </section>
    </div>
  );
}
