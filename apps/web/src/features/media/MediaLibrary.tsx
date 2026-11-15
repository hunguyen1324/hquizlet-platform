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
    dialog.current?.showModal();
    if (item.source === "upload") void refresh();
    return () => {
      dialog.current?.close();
      prev?.focus();
    };
  }, []);
  return createPortal(
    <dialog
      ref={dialog}
      className="media-watch"
      onClose={onClose}
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
    dialog.current?.showModal();
    return () => {
      dialog.current?.close();
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
          <h2 id="media-editor-title">
            {item
              ? "Chỉnh sửa"
              : kind === "video"
                ? "Chia sẻ video"
                : kind === "post"
                  ? "Viết bài đăng"
                  : "Tạo danh sách phát"}
          </h2>
          <button
            type="button"
            disabled={busy}
            aria-label="Đóng"
            onClick={() => dialog.current?.close()}
          >
            ×
          </button>
        </header>
        <label>
          Tiêu đề
          <input
            required
            maxLength={200}
            disabled={busy}
            value={input.title}
            onChange={(e) => setInput({ ...input, title: e.target.value })}
          />
        </label>
        {kind === "video" && !item && (
          <>
            <label>
              Nguồn
              <select
                disabled={busy}
                value={source}
                onChange={(e) => setSource(e.target.value)}
              >
                <option value="link">
                  YouTube / Facebook / TikTok / MP4 / WebM
                </option>
                <option value="upload">Tải file lên</option>
              </select>
            </label>
            {source === "link" ? (
              <label>
                Liên kết công khai
                <input
                  required
                  type="url"
                  disabled={busy}
                  placeholder="Dán URL đầy đủ của video"
                  value={input.url}
                  onChange={(e) => setInput({ ...input, url: e.target.value })}
                />
              </label>
            ) : (
              <label>
                File MP4 hoặc WebM
                <input
                  required
                  type="file"
                  disabled={busy}
                  accept="video/mp4,video/webm"
                  onChange={(e) => setFile(e.target.files?.[0])}
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
            rows={5}
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
          Nội dung tại khu vực chung mặc định mọi người đều thấy. Quyền của
          video gốc vẫn do nhà cung cấp quản lý.
        </p>
        {busy && source === "upload" && (
          <progress max={100} value={progress} aria-label="Tiến độ tải video" />
        )}
        {error && <p role="alert">{error}</p>}
        <button className="media-primary" disabled={busy}>
          {busy
            ? source === "upload"
              ? `Đang tải ${progress}%…`
              : "Đang lưu…"
            : "Lưu"}
        </button>
      </form>
    </dialog>,
    document.body,
  );
}
export function MediaLibrary() {
  const { token, user } = useAuth();
  const client = useQueryClient();
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
      <header className="media-heading">
        <div>
          <p className="media-eyebrow">HỌC CÙNG CỘNG ĐỒNG</p>
          <h1>Video & bài đăng</h1>
          <p>
            Khám phá bài giảng, lưu danh sách phát và chia sẻ điều bạn học được.
          </p>
        </div>
        <div className="media-actions">
          <button
            className="media-primary"
            onClick={() => setEditor({ kind: "video" })}
          >
            ＋ Video
          </button>
          <button onClick={() => setEditor({ kind: "playlist" })}>
            ＋ Danh sách phát
          </button>
          <button onClick={() => setEditor({ kind: "post" })}>
            Viết bài đăng
          </button>
        </div>
      </header>
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
      <div className="media-toolbar">
        <nav aria-label="Loại nội dung">
          {(
            [
              ["video", "Video"],
              ["playlist", "Danh sách phát"],
              ["post", "Bài đăng"],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              aria-pressed={kind === value}
              onClick={() => {
                setKind(value);
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
          <span aria-hidden="true">⌕</span>
          <input
            type="search"
            aria-label="Tìm video, danh sách phát hoặc bài đăng"
            placeholder="Tìm kiếm theo tiêu đề, nội dung…"
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
      {notice && <p role="status">{notice}</p>}
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
          {feed.data?.map((item) => (
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
                <h2>{item.title}</h2>
                <small>
                  #{item.ownerId} ·{" "}
                  {new Date(item.createdAt).toLocaleDateString("vi-VN")} ·{" "}
                  {item.visibility === "public"
                    ? "Công khai"
                    : item.visibility === "private"
                      ? "Riêng tư"
                      : "Người được chọn"}
                </small>
                {item.body && <p>{item.body}</p>}
                <div className="media-card-actions">
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
                      onClick={() => void updatePlaylist(playlist!, item, true)}
                    >
                      Gỡ khỏi danh sách
                    </button>
                  )}
                  {item.ownerId === user?.id && (
                    <>
                      <button
                        onClick={() => setEditor({ kind: item.kind, item })}
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
              </div>
            </article>
          ))}
        </div>
      )}
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
