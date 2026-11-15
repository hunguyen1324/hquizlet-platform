import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  useInfiniteQuery,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import type { MediaItem, MediaInput } from "../../lib/api/media";
import { postsApi } from "./postsApi";
import { postInput, postTitle } from "./postInput";
import "./posts.css";
const message = (e: unknown) =>
  e instanceof Error
    ? e.message
    : "Không thực hiện được thao tác. Vui lòng thử lại.";
function PostComposer({
  post,
  onClose,
  onSaved,
}: {
  post?: MediaItem;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { token, user } = useAuth();
  const dialog = useRef<HTMLDialogElement>(null);
  const [body, setBody] = useState(post ? post.body || post.title : ""),
    [visibility, setVisibility] = useState<MediaInput["visibility"]>(
      post?.visibility || "public",
    ),
    [viewers, setViewers] = useState(post?.viewerIds.join(", ") || ""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const dirty =
    body !== (post ? post.body || post.title : "") ||
    visibility !== (post?.visibility || "public") ||
    viewers !== (post?.viewerIds.join(", ") || "");
  function close() {
    if (
      !busy &&
      (!dirty || confirm("Bài viết chưa được lưu. Bạn muốn bỏ thay đổi?"))
    )
      dialog.current?.close();
  }
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    dialog.current?.showModal();
    return () => {
      dialog.current?.close();
      document.body.style.overflow = overflow;
      previous?.focus();
    };
  }, []);
  return createPortal(
    <dialog
      className="post-composer"
      ref={dialog}
      aria-labelledby="post-compose-title"
      onClose={onClose}
      onCancel={(e) => {
        e.preventDefault();
        close();
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          const r = e.currentTarget.getBoundingClientRect();
          if (
            e.clientX < r.left ||
            e.clientX > r.right ||
            e.clientY < r.top ||
            e.clientY > r.bottom
          )
            close();
        }
      }}
    >
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          if (busy) return;
          setError("");
          try {
            const input = postInput(body, visibility, viewers, post?.title);
            setBusy(true);
            await postsApi.save(token, input, post?.id);
            onSaved();
            onClose();
          } catch (e) {
            setError(message(e));
          } finally {
            setBusy(false);
          }
        }}
      >
        <header>
          <h2 id="post-compose-title">
            {post ? "Chỉnh sửa bài viết" : "Tạo bài viết"}
          </h2>
          <button
            type="button"
            aria-label="Đóng bài viết"
            disabled={busy}
            onClick={close}
          >
            ×
          </button>
        </header>
        <div className="post-composer-content">
          <div className="post-composer-author">
            <span className="post-avatar" aria-hidden="true">
              {user?.name?.slice(0, 1).toUpperCase() || "B"}
            </span>
            <div>
              <strong>{user?.name || "Bạn"}</strong>
              <label className="post-audience">
                <span className="sr-only">Ai được xem bài viết?</span>
                <select
                  aria-label="Ai được xem bài viết?"
                  disabled={busy}
                  value={visibility}
                  onChange={(e) =>
                    setVisibility(e.target.value as MediaInput["visibility"])
                  }
                >
                  <option value="public">◎ Công khai</option>
                  <option value="private">▣ Chỉ mình tôi</option>
                  <option value="selected">♧ Người được chọn</option>
                </select>
              </label>
            </div>
          </div>
          <label className="post-body-label">
            <span className="sr-only">Nội dung bài viết</span>
            <textarea
              autoFocus
              aria-label="Nội dung bài viết"
              required
              maxLength={10000}
              disabled={busy}
              rows={7}
              value={body}
              placeholder={`${user?.name?.split(" ").pop() || "Bạn"} ơi, bạn đang nghĩ gì?`}
              onChange={(e) => setBody(e.target.value)}
            />
          </label>
          {visibility === "selected" && (
            <label className="post-viewers-label">
              Mã người dùng được xem
              <input
                disabled={busy}
                value={viewers}
                onChange={(e) => setViewers(e.target.value)}
                placeholder="Ví dụ: 12, 25, 38"
              />
              <small>
                Cách nhau bằng dấu phẩy. Bạn luôn xem được bài của mình.
              </small>
            </label>
          )}
          <div className="post-composer-hint">
            <span>
              {visibility === "public"
                ? "Mọi tài khoản HQuizlet có thể xem bài viết này."
                : visibility === "private"
                  ? "Bài viết này chỉ hiển thị với bạn."
                  : "Chỉ những người được chọn xem được bài viết."}
            </span>
            <span>
              {Array.from(body).length.toLocaleString("vi-VN")}/10.000
            </span>
          </div>
          {error && (
            <p className="post-error" role="alert">
              {error}
            </p>
          )}
        </div>
        <footer>
          <button type="submit" disabled={busy || !body.trim()}>
            {busy ? "Đang đăng…" : post ? "Lưu thay đổi" : "Đăng"}
          </button>
        </footer>
      </form>
    </dialog>,
    document.body,
  );
}
function PostCard({
  post,
  onEdit,
  onRemoved,
  onNotice,
}: {
  post: MediaItem;
  onEdit: (post: MediaItem) => void;
  onRemoved: () => void;
  onNotice: (text: string) => void;
}) {
  const { token, user } = useAuth();
  const [expanded, setExpanded] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const own = post.ownerId === user?.id;
  const author = own ? user?.name || "Bạn" : `Thành viên #${post.ownerId}`;
  const text = post.body || post.title;
  const long = text.length > 650;
  const date = new Date(post.createdAt);
  async function copyLink() {
    try {
      const url = new URL(window.location.href);
      url.search = "";
      url.searchParams.set("post", String(post.id));
      await navigator.clipboard.writeText(url.toString());
      onNotice("Đã sao chép liên kết bài viết.");
    } catch {
      setError(
        "Không sao chép được liên kết. Hãy kiểm tra quyền clipboard của trình duyệt.",
      );
    }
  }
  return (
    <article className="post-card" aria-label={`Bài viết của ${author}`}>
      <header>
        <span className="post-avatar" aria-hidden="true">
          {own
            ? (user?.name || "Bạn").slice(0, 1).toUpperCase()
            : String(post.ownerId).slice(-2)}
        </span>
        <div className="post-author">
          <strong>{author}</strong>
          <p>
            <time
              dateTime={post.createdAt}
              title={date.toLocaleString("vi-VN")}
            >
              {date.toLocaleDateString("vi-VN", {
                day: "numeric",
                month: "long",
              })}{" "}
              lúc{" "}
              {date.toLocaleTimeString("vi-VN", {
                hour: "2-digit",
                minute: "2-digit",
              })}
            </time>
            <span aria-hidden="true"> · </span>
            <span>
              {post.visibility === "public"
                ? "Công khai"
                : post.visibility === "private"
                  ? "Chỉ mình tôi"
                  : "Người được chọn"}
            </span>
          </p>
        </div>
        <details className="post-menu">
          <summary aria-label={`Tùy chọn bài viết của ${author}`}>⋯</summary>
          <div>
            <button onClick={() => void copyLink()}>Sao chép liên kết</button>
            {own && (
              <>
                <button disabled={busy} onClick={() => onEdit(post)}>
                  Chỉnh sửa bài viết
                </button>
                <button
                  className="post-delete"
                  disabled={busy}
                  onClick={async () => {
                    if (busy || !confirm("Xóa bài viết này?")) return;
                    setBusy(true);
                    setError("");
                    try {
                      await postsApi.remove(token, post.id);
                      onRemoved();
                    } catch (e) {
                      setError(message(e));
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  {busy ? "Đang xóa…" : "Xóa bài viết"}
                </button>
              </>
            )}
          </div>
        </details>
      </header>
      {post.body && post.title !== postTitle(post.body) && (
        <h2>{post.title}</h2>
      )}
      <div
        className={`post-text ${text.length < 160 ? "post-text-short" : ""}`}
      >
        {long && !expanded ? text.slice(0, 650) + "…" : text}
      </div>
      {long && (
        <button
          className="post-read-more"
          aria-expanded={expanded}
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? "Thu gọn" : "Xem thêm"}
        </button>
      )}
      {error && (
        <p className="post-error" role="alert">
          {error}
        </p>
      )}
    </article>
  );
}
export function PostsFeed() {
  const { token, user } = useAuth();
  const client = useQueryClient();
  const [params, setParams] = useSearchParams();
  const rawId = Number(params.get("post"));
  const postId = Number.isSafeInteger(rawId) && rawId > 0 ? rawId : 0;
  const [filter, setFilter] = useState<"all" | "mine">("all"),
    [search, setSearch] = useState(""),
    [query, setQuery] = useState(""),
    [composer, setComposer] = useState<{ post?: MediaItem }>(),
    [notice, setNotice] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => setQuery(search.trim()), 250);
    return () => clearTimeout(timer);
  }, [search]);
  const feed = useInfiniteQuery({
    queryKey: ["posts-feed", user?.id, filter, query],
    initialPageParam: 0,
    queryFn: ({ pageParam, signal }) =>
      postsApi.list(token, query, filter === "mine", pageParam, signal),
    getNextPageParam: (last, pages) =>
      last.length === 20 ? pages.length * 20 : undefined,
    enabled: !!token && !postId,
  });
  const selected = useQuery({
    queryKey: ["post-detail", user?.id, postId],
    queryFn: ({ signal }) => postsApi.get(token, postId, signal),
    enabled: !!token && !!postId,
    retry: false,
  });
  async function reload() {
    await Promise.all([
      client.invalidateQueries({ queryKey: ["posts-feed"] }),
      client.invalidateQueries({ queryKey: ["post-detail"] }),
    ]);
  }
  const posts = postId
    ? selected.data
      ? [selected.data]
      : []
    : feed.data?.pages.flat() || [];
  const loading = postId ? selected.isPending : feed.isPending;
  const error = postId ? selected.error : feed.error;
  return (
    <section className="posts-page">
      <header className="posts-page-heading">
        <div>
          <p>CỘNG ĐỒNG HQUIZLET</p>
          <h1>Bài đăng</h1>
          <span>
            Chia sẻ câu chuyện, kiến thức và những điều bạn học mỗi ngày.
          </span>
        </div>
      </header>
      <div className="posts-layout">
        <aside className="posts-side">
          <nav aria-label="Lọc bài đăng">
            <button
              aria-pressed={filter === "all" && !postId}
              onClick={() => {
                setFilter("all");
                setParams({});
              }}
            >
              ☷ <span>Bảng tin</span>
            </button>
            <button
              aria-pressed={filter === "mine" && !postId}
              onClick={() => {
                setFilter("mine");
                setParams({});
              }}
            >
              ♙ <span>Bài viết của tôi</span>
            </button>
          </nav>
          <p>
            Nội dung mới mặc định công khai. Bạn có thể chọn người xem khi đăng
            bài.
          </p>
        </aside>
        <div className="posts-feed" aria-label="Bảng tin cộng đồng">
          <div className="posts-compose-prompt">
            <span className="post-avatar" aria-hidden="true">
              {user?.name?.slice(0, 1).toUpperCase() || "B"}
            </span>
            <button onClick={() => setComposer({})}>
              {user?.name?.split(" ").pop() || "Bạn"} ơi, bạn đang nghĩ gì?
            </button>
            <button
              className="posts-compose-action"
              aria-label="Tạo bài viết"
              onClick={() => setComposer({})}
            >
              ✎
            </button>
          </div>
          <div className="posts-feed-toolbar">
            {postId ? (
              <button className="posts-back" onClick={() => setParams({})}>
                ← Quay lại bảng tin
              </button>
            ) : (
              <h2>
                {filter === "mine" ? "Bài viết của tôi" : "Bài viết mới nhất"}
              </h2>
            )}
            {!postId && (
              <label className="posts-search">
                <svg
                  width="16"
                  height="16"
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
                  aria-label="Tìm bài viết"
                  placeholder="Tìm bài viết…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </label>
            )}
          </div>
          {notice && (
            <p className="posts-notice" role="status">
              {notice}
              <button aria-label="Ẩn thông báo" onClick={() => setNotice("")}>
                ×
              </button>
            </p>
          )}
          {error && (
            <p role="alert" className="post-error">
              {message(error)}{" "}
              <button
                onClick={() =>
                  postId ? void selected.refetch() : void feed.refetch()
                }
              >
                Thử lại
              </button>
            </p>
          )}
          {loading ? (
            <div className="posts-empty" role="status">
              Đang tải bài viết…
            </div>
          ) : !error && !posts.length ? (
            <div className="posts-empty">
              <span aria-hidden="true">✎</span>
              <h2>
                {query
                  ? "Chưa tìm thấy bài viết"
                  : filter === "mine"
                    ? "Bạn chưa có bài viết nào"
                    : "Câu chuyện đầu tiên đang chờ bạn"}
              </h2>
              <p>
                {query
                  ? "Thử từ khóa khác để tìm nội dung bạn quan tâm."
                  : "Chia sẻ điều bạn vừa học hoặc một câu chuyện với cộng đồng."}
              </p>
              {!query && (
                <button onClick={() => setComposer({})}>Tạo bài viết</button>
              )}
            </div>
          ) : (
            posts.map((post) => (
              <PostCard
                key={post.id}
                post={post}
                onEdit={(post) => setComposer({ post })}
                onNotice={setNotice}
                onRemoved={() => {
                  setNotice("Đã xóa bài viết.");
                  if (postId) setParams({});
                  void reload();
                }}
              />
            ))
          )}
          {!postId && feed.hasNextPage && (
            <button
              className="posts-load-more"
              disabled={feed.isFetchingNextPage}
              onClick={() => void feed.fetchNextPage()}
            >
              {feed.isFetchingNextPage ? "Đang tải…" : "Xem thêm bài viết"}
            </button>
          )}
        </div>
        <aside className="posts-guide">
          <span aria-hidden="true">✦</span>
          <h2>Mỗi chia sẻ đều có giá trị</h2>
          <p>
            Một cách học mới, một câu hỏi hay, một câu chuyện nhỏ. Bắt đầu từ
            điều bạn muốn chia sẻ.
          </p>
          <button onClick={() => setComposer({})}>Viết bài mới →</button>
        </aside>
      </div>
      {composer && (
        <PostComposer
          key={composer.post?.id || "new"}
          post={composer.post}
          onClose={() => setComposer(undefined)}
          onSaved={() => {
            setNotice(
              composer.post
                ? "Đã cập nhật bài viết."
                : "Bài viết của bạn đã được đăng.",
            );
            setComposer(undefined);
            if (!composer.post) {
              setFilter("all");
              setSearch("");
              setQuery("");
              setParams({});
            }
            void reload();
          }}
        />
      )}
    </section>
  );
}
