import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "../auth/AuthContext";
import { mediaApi, type MediaItem, type MediaInput } from "../../lib/api/media";
import { gatewayUrl } from "../../lib/api/gateway";
import "./media.css";
import { classApi } from "../../lib/api";
import { ClassVideoLibrary } from "./ClassVideoLibrary";
const empty = (kind: MediaItem["kind"]): MediaInput => ({
  kind,
  title: "",
  body: "",
  url: "",
  visibility: "public",
  viewerIds: [],
});
const fail = (e: unknown) =>
  e instanceof Error ? e.message : "Không thực hiện được thao tác.";
function videoPreview(raw: string) {
  try {
    const url = new URL(raw);
    if (url.protocol !== "https:")
      return { label: "Liên kết video", thumbnail: "" };
    const host = url.hostname.toLowerCase();
    const id =
      host === "youtu.be"
        ? url.pathname.slice(1)
        : ["youtube.com", "www.youtube.com", "m.youtube.com"].includes(host)
          ? url.searchParams.get("v") || url.pathname.split("/")[2]
          : "";
    if (id && /^[a-zA-Z0-9_-]{11}$/.test(id))
      return {
        label: "YouTube",
        thumbnail: `https://i.ytimg.com/vi/${id}/hqdefault.jpg`,
      };
    return {
      label: ["tiktok.com", "www.tiktok.com"].includes(host)
        ? "TikTok"
        : ["facebook.com", "www.facebook.com", "m.facebook.com"].includes(host)
          ? "Facebook"
          : "Liên kết video",
      thumbnail: "",
    };
  } catch {
    return { label: "Xem trước video", thumbnail: "" };
  }
}
function MediaPlayer({
  item,
  onClose,
}: {
  item: MediaItem;
  onClose: () => void;
}) {
  const { token } = useAuth();
  const dialog = useRef<HTMLDialogElement>(null);
  const video = useRef<HTMLVideoElement>(null);
  const [url, setURL] = useState(item.embedUrl),
    [error, setError] = useState("");
  const resume = useRef(0),
    retry = useRef(false);
  async function refresh() {
    try {
      const result = await mediaApi.playback(token, item.id);
      setURL(`${gatewayUrl}${result.url}`);
    } catch (e) {
      setError(fail(e));
    }
  }
  useEffect(() => {
    const prev = document.activeElement as HTMLElement;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    dialog.current?.showModal();
    if (item.source === "upload") void refresh();
    return () => {
      dialog.current?.close();
      document.body.style.overflow = overflow;
      prev?.focus();
    };
  }, []);
  return createPortal(
    <dialog
      ref={dialog}
      className="media-watch"
      onClose={onClose}
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          const r = e.currentTarget.getBoundingClientRect();
          if (
            e.clientX < r.left ||
            e.clientX > r.right ||
            e.clientY < r.top ||
            e.clientY > r.bottom
          )
            e.currentTarget.close();
        }
      }}
      aria-label={item.title}
    >
      <header>
        <h2>{item.title}</h2>
        <button
          type="button"
          aria-label="Đóng video"
          onClick={() => dialog.current?.close()}
        >
          ×
        </button>
      </header>
      {error && <p role="alert">{error}</p>}
      {url ? (
        item.source === "direct" || item.source === "upload" ? (
          <video
            ref={video}
            src={url}
            controls
            autoPlay
            playsInline
            onLoadedMetadata={() => {
              if (video.current) video.current.currentTime = resume.current;
            }}
            onError={() => {
              if (item.source === "upload" && !retry.current) {
                retry.current = true;
                resume.current = video.current?.currentTime ?? 0;
                void refresh();
              } else
                setError(
                  "Không phát được video. Thử mở lại hoặc kiểm tra nguồn.",
                );
            }}
          />
        ) : (
          <iframe
            src={url}
            title={item.title}
            allow="autoplay; encrypted-media; picture-in-picture; fullscreen"
            allowFullScreen
            referrerPolicy="strict-origin-when-cross-origin"
          />
        )
      ) : (
        <p role="status">Đang mở video…</p>
      )}
      <footer>
        <p>{item.body}</p>
        {item.url && (
          <a href={item.url} target="_blank" rel="noreferrer">
            Mở nguồn gốc ↗
          </a>
        )}
        <small>
          Video nhúng phụ thuộc quyền công khai và khả năng cho phép nhúng của
          nguồn.
        </small>
      </footer>
    </dialog>,
    document.body,
  );
}
function MediaEditor({
  kind,
  item,
  onClose,
  onSaved,
}: {
  kind: MediaItem["kind"];
  item?: MediaItem;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { token } = useAuth();
  const dialog = useRef<HTMLDialogElement>(null);
  const [input, setInput] = useState<MediaInput>(item ?? empty(kind)),
    [file, setFile] = useState<File>(),
    [source, setSource] = useState("link"),
    [viewers, setViewers] = useState(item?.viewerIds.join(", ") ?? ""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [progress, setProgress] = useState(0);
  useEffect(() => {
    const prev = document.activeElement as HTMLElement;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    dialog.current?.showModal();
    return () => {
      dialog.current?.close();
      document.body.style.overflow = overflow;
      prev?.focus();
    };
  }, []);
  return createPortal(
    <dialog
      ref={dialog}
      className="media-editor"
      aria-labelledby="media-editor-title"
      onCancel={(e) => {
        if (busy) e.preventDefault();
      }}
      onClose={onClose}
      onClick={(e) => {
        if (!busy && e.target === e.currentTarget) {
          const r = e.currentTarget.getBoundingClientRect();
          if (
            e.clientX < r.left ||
            e.clientX > r.right ||
            e.clientY < r.top ||
            e.clientY > r.bottom
          )
            e.currentTarget.close();
        }
      }}
    >
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          if (busy) return;
          setBusy(true);
          setError("");
          try {
            const ids = viewers.trim()
              ? viewers.split(/[\s,]+/).map(Number)
              : [];
            if (ids.some((id) => !Number.isSafeInteger(id) || id < 1))
              throw Error(
                "Mã người xem phải là số nguyên dương, cách nhau bằng dấu phẩy.",
              );
            const payload = {
              ...input,
              viewerIds: input.visibility === "selected" ? ids : [],
            };
            let id = item?.id;
            if (!item && kind === "video" && source === "upload") {
              if (!file) throw Error("Chọn file video.");
              id = (await mediaApi.upload(token, payload, file, setProgress))
                .id;
            } else await mediaApi.save(token, payload, id);
            onSaved();
            onClose();
          } catch (e) {
            setError(fail(e));
          } finally {
            setBusy(false);
          }
        }}
      >
        <header>
          <div>
            <p className="media-dialog-eyebrow">KHÔNG GIAN CỘNG ĐỒNG</p>
            <h2 id="media-editor-title">
              {item
                ? "Chỉnh sửa"
                : kind === "video"
                  ? "Chia sẻ video"
                  : kind === "post"
                    ? "Viết bài đăng"
                    : "Tạo danh sách phát"}
            </h2>
            <p className="media-dialog-subtitle">
              {kind === "video"
                ? "Một video hay, thêm một điều mới để học."
                : kind === "post"
                  ? "Chia sẻ kiến thức và bắt đầu một cuộc trò chuyện."
                  : "Gom những video yêu thích vào một nơi."}
            </p>
          </div>
          <button
            type="button"
            disabled={busy}
            aria-label="Đóng"
            onClick={() => dialog.current?.close()}
          >
            ×
          </button>
        </header>
        <div
          className={`media-editor-body ${kind === "video" ? "has-preview" : ""}`}
        >
          <div className="media-editor-fields">
            <label>
              Tiêu đề
              <input
                required
                autoFocus
                placeholder={
                  kind === "video"
                    ? "Ví dụ: Luyện nghe tiếng Nhật mỗi ngày"
                    : kind === "playlist"
                      ? "Ví dụ: Tiếng Nhật cho người mới bắt đầu"
                      : "Bạn muốn chia sẻ điều gì?"
                }
                maxLength={200}
                disabled={busy}
                value={input.title}
                onChange={(e) => setInput({ ...input, title: e.target.value })}
              />
            </label>
            {kind === "video" && !item && (
              <>
                <div
                  className="media-source-choice"
                  role="group"
                  aria-label="Chọn nguồn video"
                >
                  <button
                    type="button"
                    disabled={busy}
                    aria-pressed={source === "link"}
                    onClick={() => setSource("link")}
                  >
                    <span aria-hidden="true">↗</span> Dán liên kết
                  </button>
                  <button
                    type="button"
                    disabled={busy}
                    aria-pressed={source === "upload"}
                    onClick={() => setSource("upload")}
                  >
                    <span aria-hidden="true">↑</span> Tải từ máy
                  </button>
                </div>
                {source === "link" ? (
                  <label>
                    Liên kết video
                    <input
                      required
                      type="url"
                      disabled={busy}
                      placeholder="https://www.youtube.com/watch?v=…"
                      value={input.url}
                      onChange={(e) =>
                        setInput({ ...input, url: e.target.value })
                      }
                    />
                    <small>
                      YouTube, Facebook, TikTok hoặc liên kết MP4 / WebM công
                      khai.
                    </small>
                  </label>
                ) : (
                  <label className="media-file-picker">
                    <span className="media-upload-icon" aria-hidden="true">
                      ↑
                    </span>
                    <strong>{file ? file.name : "Chọn video từ máy"}</strong>
                    <span>MP4 hoặc WebM · Tối đa 1 GiB</span>
                    <input
                      required
                      type="file"
                      disabled={busy}
                      accept="video/mp4,video/webm"
                      aria-label="Chọn file video"
                      onChange={(e) => {
                        const chosen = e.target.files?.[0];
                        if (chosen && chosen.size > 1073741824) {
                          setError(
                            "Video vượt quá 1 GiB. Hãy chọn file nhỏ hơn.",
                          );
                          e.target.value = "";
                          setFile(undefined);
                        } else {
                          setFile(chosen);
                          setError("");
                        }
                      }}
                    />
                    <small>Tối đa 1 GiB. Giữ cửa sổ mở đến khi hoàn tất.</small>
                  </label>
                )}
              </>
            )}
            <label>
              {kind === "post" ? "Nội dung bài đăng" : "Mô tả"}
              <textarea
                required={kind === "post"}
                maxLength={10000}
                rows={kind === "post" ? 7 : 3}
                placeholder={
                  kind === "post"
                    ? "Viết nội dung bạn muốn chia sẻ…"
                    : "Giới thiệu ngắn để mọi người dễ tìm thấy nội dung…"
                }
                disabled={busy}
                value={input.body}
                onChange={(e) => setInput({ ...input, body: e.target.value })}
              />
            </label>
            <label>
              Ai được xem?
              <select
                value={input.visibility}
                disabled={busy}
                onChange={(e) =>
                  setInput({
                    ...input,
                    visibility: e.target.value as MediaInput["visibility"],
                  })
                }
              >
                <option value="public">Mọi người trên HQuizlet</option>
                <option value="private">Chỉ mình tôi</option>
                <option value="selected">Chọn người xem</option>
              </select>
            </label>
            {input.visibility === "selected" && (
              <label>
                Mã người dùng được xem
                <input
                  placeholder="Ví dụ: 12, 25, 38"
                  value={viewers}
                  disabled={busy}
                  onChange={(e) => setViewers(e.target.value)}
                />
              </label>
            )}
            <p className="media-hint">
              {input.visibility === "public"
                ? "Mọi tài khoản HQuizlet có thể xem nội dung này."
                : input.visibility === "private"
                  ? "Nội dung này chỉ hiển thị với bạn."
                  : "Chỉ những người bạn chọn được xem nội dung này."}
            </p>
          </div>
          {kind === "video" && (
            <aside className="media-editor-preview">
              <p>XEM TRƯỚC</p>
              <div className="media-preview-image">
                {source === "link" &&
                videoPreview(input.url || "").thumbnail ? (
                  <img
                    src={videoPreview(input.url || "").thumbnail}
                    alt="Ảnh đại diện video"
                  />
                ) : (
                  <span aria-hidden="true">▷</span>
                )}
              </div>
              <h3>{input.title.trim() || "Tiêu đề video của bạn"}</h3>
              <span className="media-preview-source">
                {source === "upload"
                  ? "Video từ máy"
                  : videoPreview(input.url || "").label}
              </span>
              <small>
                {source === "upload"
                  ? "Video sẽ sẵn sàng trong thư viện sau khi tải lên."
                  : "Video phát từ nguồn gốc. Nguồn cần cho phép xem công khai và nhúng."}
              </small>
            </aside>
          )}
        </div>
        <footer className="media-editor-footer">
          <div aria-live="polite">
            {busy && source === "upload" && (
              <>
                <span>
                  {progress === 100
                    ? "Đang hoàn tất…"
                    : `Đang tải video · ${progress}%`}
                </span>
                <progress
                  max={100}
                  value={progress}
                  aria-label="Tiến độ tải video"
                />
              </>
            )}
            {error && <p role="alert">{error}</p>}
          </div>
          <div className="media-editor-footer-actions">
            <button
              type="button"
              disabled={busy}
              onClick={() => dialog.current?.close()}
            >
              Hủy
            </button>
            <button
              type="submit"
              className="media-primary"
              disabled={
                busy ||
                !input.title.trim() ||
                (!item &&
                  kind === "video" &&
                  (source === "link" ? !input.url?.trim() : !file))
              }
            >
              {busy
                ? "Đang lưu…"
                : item
                  ? "Lưu thay đổi"
                  : kind === "video"
                    ? "Chia sẻ video"
                    : kind === "post"
                      ? "Đăng bài"
                      : "Tạo danh sách phát"}
            </button>
          </div>
        </footer>
      </form>
    </dialog>,
    document.body,
  );
}
export function MediaLibrary() {
  const { token, user } = useAuth();
  const client = useQueryClient();
  const [tab, setTab] = useState<"home" | MediaItem["kind"]>("home");
  const [kind, setKind] = useState<MediaItem["kind"]>("video"),
    [search, setSearch] = useState(""),
    [query, setQuery] = useState(""),
    [offset, setOffset] = useState(0),
    [playlist, setPlaylist] = useState<MediaItem>(),
    [playing, setPlaying] = useState<MediaItem>(),
    [editor, setEditor] = useState<{
      kind: MediaItem["kind"];
      item?: MediaItem;
    }>(),
    [saving, setSaving] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(search.trim());
      setOffset(0);
    }, 250);
    return () => clearTimeout(timer);
  }, [search]);
  const [scope, setScope] = useState(0);
  const classes = useQuery({
    queryKey: ["media-classes", user?.id],
    queryFn: () => classApi.list(token),
    enabled: !!token,
  });
  const key = ["media", user?.id, kind, query, playlist?.id ?? 0, offset];
  const feed = useQuery({
    queryKey: key,
    queryFn: ({ signal }) =>
      mediaApi.list(token, kind, query, playlist?.id, offset, signal),
    enabled: !!token,
  });
  const playlists = useQuery({
    queryKey: ["media-playlists", user?.id],
    queryFn: ({ signal }) => mediaApi.ownPlaylists(token, signal),
    enabled: !!token,
  });
  const ownPlaylists =
    playlists.data?.filter((p) => p.ownerId === user?.id) ?? [];
  async function reload() {
    await Promise.all([
      client.invalidateQueries({ queryKey: ["media"] }),
      client.invalidateQueries({ queryKey: ["media-playlists"] }),
    ]);
  }
  async function updatePlaylist(
    target: MediaItem,
    video: MediaItem,
    remove = false,
  ) {
    if (saving) return;
    setSaving(true);
    setError("");
    try {
      await mediaApi.changePlaylistItem(token, target.id, video.id, remove);
      setNotice(
        remove
          ? "Đã gỡ video khỏi danh sách phát."
          : `Đã lưu vào ${target.title}.`,
      );
      await reload();
    } catch (e) {
      setError(fail(e));
    } finally {
      setSaving(false);
    }
  }
  if (scope > 0)
    return (
      <section className="media-library">
        <div className="media-scope">
          <label>
            Không gian video{" "}
            <select
              aria-label="Không gian video"
              value={scope}
              onChange={(e) => setScope(Number(e.target.value))}
            >
              <option value={0}>Cộng đồng · mọi người</option>
              {classes.data?.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name} · thành viên lớp
                </option>
              ))}
            </select>
          </label>
        </div>
        <ClassVideoLibrary key={scope} classId={scope} />
      </section>
    );
  return (
    <section className="media-library">
      <div className="media-channel-banner" aria-hidden="true">
        <div>
          <span>HỌC MỖI NGÀY</span>
          <strong>
            Một video.
            <br />
            Một điều mới.
          </strong>
        </div>
        <div className="media-banner-art">
          <span>あ</span>
          <span>▶</span>
          <span>学</span>
        </div>
      </div>
      <header className="media-heading">
        <div className="media-channel-profile">
          <img
            className="media-channel-avatar"
            src={`${import.meta.env.BASE_URL}logo.svg`}
            alt=""
          />
          <div>
            <p className="media-eyebrow">CỘNG ĐỒNG HQUIZLET</p>
            <h1>Video</h1>
            <p>Khám phá video học tập và lưu bài giảng vào danh sách phát.</p>
          </div>
        </div>
        <div className="media-actions">
          <button
            className="media-primary"
            onClick={() => setEditor({ kind: "video" })}
          >
            ＋ Chia sẻ video
          </button>
        </div>
        <div className="media-scope">
          <label>
            Không gian video{" "}
            <select
              aria-label="Không gian video"
              value={scope}
              onChange={(e) => setScope(Number(e.target.value))}
            >
              <option value={0}>Cộng đồng · mọi người</option>
              {classes.data?.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name} · thành viên lớp
                </option>
              ))}
            </select>
          </label>
        </div>
      </header>
      <div className="media-toolbar">
        <nav aria-label="Loại nội dung">
          {(
            [
              ["home", "Trang chủ"],
              ["video", "Video"],
              ["playlist", "Danh sách phát"],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              aria-pressed={tab === value}
              onClick={() => {
                setTab(value);
                setKind(value === "home" ? "video" : value);
                setPlaylist(undefined);
                setOffset(0);
                setError("");
              }}
            >
              {label}
            </button>
          ))}
        </nav>
        <label className="media-search">
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.8"
            aria-hidden="true"
          >
            <circle cx="10.5" cy="10.5" r="6.5" />
            <path d="m16 16 4 4" />
          </svg>
          <input
            type="search"
            aria-label="Tìm video hoặc danh sách phát"
            placeholder="Tìm trong thư viện…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          {search && (
            <button aria-label="Xóa tìm kiếm" onClick={() => setSearch("")}>
              ×
            </button>
          )}
        </label>
      </div>
      {playlist && (
        <div className="media-playlist-heading">
          <button
            onClick={() => {
              setPlaylist(undefined);
              setKind("playlist");
              setOffset(0);
            }}
          >
            ← Danh sách phát
          </button>
          <h2>{playlist.title}</h2>
          <p>{playlist.body}</p>
        </div>
      )}
      {!playlist && (
        <div className="media-section-heading">
          <div>
            <h2>
              {query
                ? `Kết quả cho “${query}”`
                : tab === "home"
                  ? "Video mới nhất"
                  : kind === "video"
                    ? "Tất cả video"
                    : kind === "playlist"
                      ? "Danh sách phát"
                      : "Bài đăng cộng đồng"}
            </h2>
            <p>
              {kind === "video"
                ? "Khám phá, học hỏi và lưu lại những video hữu ích."
                : kind === "playlist"
                  ? "Sắp xếp hành trình học của bạn theo từng chủ đề."
                  : "Kiến thức và câu chuyện từ cộng đồng."}
            </p>
          </div>
          {tab === "home" ? (
            <button
              onClick={() => {
                setTab("video");
                setKind("video");
                setOffset(0);
              }}
            >
              Xem tất cả →
            </button>
          ) : kind === "playlist" ? (
            <button onClick={() => setEditor({ kind: "playlist" })}>
              ＋ Tạo danh sách phát
            </button>
          ) : (
            <span className="media-sort-label">Mới nhất</span>
          )}
        </div>
      )}
      {notice && (
        <p className="media-notice" role="status">
          {notice}
          <button aria-label="Ẩn thông báo" onClick={() => setNotice("")}>
            ×
          </button>
        </p>
      )}
      {(error || feed.error) && (
        <p role="alert">
          {error || fail(feed.error)}{" "}
          <button onClick={() => void feed.refetch()}>Thử lại</button>
        </p>
      )}
      {feed.isPending ? (
        <p role="status">Đang tải nội dung…</p>
      ) : feed.data?.length === 0 ? (
        <div className="media-empty">
          <h2>{query ? "Chưa tìm thấy kết quả" : "Chưa có nội dung"}</h2>
          <p>
            {query
              ? "Thử từ khóa khác hoặc xóa tìm kiếm."
              : "Nội dung được chia sẻ với bạn sẽ xuất hiện ở đây."}
          </p>
        </div>
      ) : (
        <div
          className={`media-grid ${kind === "post" ? "media-post-grid" : ""}`}
        >
          {(tab === "home" && !query ? feed.data?.slice(0, 8) : feed.data)?.map(
            (item) => (
              <article
                className={`media-card media-card-${item.kind}`}
                key={item.id}
              >
                {item.kind !== "post" && (
                  <button
                    className="media-open"
                    aria-label={`${item.kind === "playlist" ? "Mở danh sách phát" : "Xem video"}: ${item.title}`}
                    onClick={() => {
                      if (item.kind === "playlist") {
                        setTab("video");
                        setPlaylist(item);
                        setKind("video");
                        setOffset(0);
                        setSearch("");
                        setQuery("");
                      } else setPlaying(item);
                    }}
                  >
                    <span className="media-thumbnail">
                      {item.thumbnailUrl ? (
                        <img src={item.thumbnailUrl} loading="lazy" alt="" />
                      ) : (
                        <span className="media-placeholder" aria-hidden="true">
                          {item.kind === "playlist" ? "☷" : "▷"}
                        </span>
                      )}
                      <span className="media-source">
                        {item.kind === "playlist"
                          ? "Danh sách phát"
                          : item.source === "upload"
                            ? "Video tải lên"
                            : item.source}
                      </span>
                      <span className="media-play" aria-hidden="true">
                        {item.kind === "playlist" ? "☷" : "▶"}
                      </span>
                    </span>
                  </button>
                )}
                <div className="media-copy">
                  <h2>
                    {item.kind === "post" ? (
                      item.title
                    ) : (
                      <button
                        className="media-title-button"
                        onClick={() => {
                          if (item.kind === "playlist") {
                            setPlaylist(item);
                            setTab("video");
                            setKind("video");
                            setOffset(0);
                            setSearch("");
                            setQuery("");
                          } else setPlaying(item);
                        }}
                      >
                        {item.title}
                      </button>
                    )}
                  </h2>
                  <small>
                    {item.ownerId === user?.id
                      ? user.name
                      : `Thành viên #${item.ownerId}`}{" "}
                    · {new Date(item.createdAt).toLocaleDateString("vi-VN")} ·{" "}
                    {item.visibility === "public"
                      ? "Công khai"
                      : item.visibility === "private"
                        ? "Riêng tư"
                        : "Người được chọn"}
                  </small>
                  {item.body && <p>{item.body}</p>}
                  {(item.kind === "video" || item.ownerId === user?.id) && (
                    <details className="media-card-menu">
                      <summary aria-label={`Tùy chọn: ${item.title}`}>
                        ⋮
                      </summary>
                      <div className="media-card-actions">
                        {item.kind === "video" && !ownPlaylists.length && (
                          <button
                            onClick={() => setEditor({ kind: "playlist" })}
                          >
                            ＋ Tạo danh sách phát
                          </button>
                        )}
                        {item.kind === "video" && ownPlaylists.length > 0 && (
                          <select
                            aria-label={`Lưu ${item.title} vào danh sách phát`}
                            disabled={saving}
                            value=""
                            onChange={(e) => {
                              const target = ownPlaylists.find(
                                (p) => p.id === Number(e.target.value),
                              );
                              if (target) void updatePlaylist(target, item);
                            }}
                          >
                            <option value="">＋ Lưu vào…</option>
                            {ownPlaylists.map((p) => (
                              <option value={p.id} key={p.id}>
                                {p.title}
                              </option>
                            ))}
                          </select>
                        )}
                        {playlist?.ownerId === user?.id && (
                          <button
                            disabled={saving}
                            onClick={() =>
                              void updatePlaylist(playlist!, item, true)
                            }
                          >
                            Gỡ khỏi danh sách
                          </button>
                        )}
                        {item.ownerId === user?.id && (
                          <>
                            <button
                              onClick={() =>
                                setEditor({ kind: item.kind, item })
                              }
                            >
                              Sửa
                            </button>
                            <button
                              disabled={saving}
                              className="media-danger"
                              onClick={async () => {
                                if (saving || !confirm(`Xóa “${item.title}”?`))
                                  return;
                                setSaving(true);
                                setError("");
                                try {
                                  await mediaApi.remove(token, item.id);
                                  await reload();
                                } catch (e) {
                                  setError(fail(e));
                                } finally {
                                  setSaving(false);
                                }
                              }}
                            >
                              Xóa
                            </button>
                          </>
                        )}
                      </div>
                    </details>
                  )}
                </div>
              </article>
            ),
          )}
        </div>
      )}
      {tab === "home" && !query && ownPlaylists.length > 0 && (
        <section className="media-home-shelf">
          <div className="media-section-heading">
            <h2>Danh sách phát của bạn</h2>
            <button
              onClick={() => {
                setTab("playlist");
                setKind("playlist");
                setOffset(0);
              }}
            >
              Xem tất cả →
            </button>
          </div>
          <div className="media-playlist-shelf">
            {ownPlaylists.slice(0, 4).map((p) => (
              <button
                key={p.id}
                className="media-playlist-tile"
                onClick={() => {
                  setPlaylist(p);
                  setTab("video");
                  setKind("video");
                  setOffset(0);
                }}
              >
                <span aria-hidden="true">☷</span>
                <strong>{p.title}</strong>
                <small>Mở danh sách phát →</small>
              </button>
            ))}
          </div>
        </section>
      )}
      {(tab !== "home" || !!query) && (
        <div className="media-pagination">
          <button
            disabled={offset === 0 || feed.isFetching}
            onClick={() => setOffset(Math.max(0, offset - 60))}
          >
            ← Trước
          </button>
          <span>Trang {offset / 60 + 1}</span>
          <button
            disabled={!feed.data || feed.data.length < 60 || feed.isFetching}
            onClick={() => setOffset(offset + 60)}
          >
            Tiếp →
          </button>
        </div>
      )}
      {playing && (
        <MediaPlayer
          key={playing.id}
          item={playing}
          onClose={() => setPlaying(undefined)}
        />
      )}{" "}
      {editor && (
        <MediaEditor
          key={`${editor.kind}-${editor.item?.id ?? "new"}`}
          {...editor}
          onSaved={() => void reload()}
          onClose={() => setEditor(undefined)}
        />
      )}
    </section>
  );
}
