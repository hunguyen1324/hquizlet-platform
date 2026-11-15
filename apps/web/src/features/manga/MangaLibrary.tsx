import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  mangaApi,
  type MangaVolume,
  type MangaPageResponse,
} from "../../lib/api/manga";
import { useAuth } from "../auth/AuthContext";
import { prepareMangaImport, type MangaImport } from "./mokuro";
import { groupMangaSeries, volumeLabel } from "./mangaSeries";
import "./manga.css";
const message = (e: unknown) =>
  e instanceof Error ? e.message : "Không thực hiện được thao tác.";
function MangaCover({
  token,
  volume,
}: {
  token: string;
  volume?: MangaVolume;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false),
    [url, setURL] = useState("");
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "150px" },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    if (!visible || !volume) return;
    const controller = new AbortController();
    let local = "";
    mangaApi
      .image(token, volume.id, 0, controller.signal)
      .then((blob) => {
        if (controller.signal.aborted) return;
        local = URL.createObjectURL(blob);
        setURL(local);
      })
      .catch(() => {});
    return () => {
      controller.abort();
      if (local) URL.revokeObjectURL(local);
    };
  }, [token, volume?.id, visible]);
  return (
    <div className="manga-series-cover" ref={ref}>
      {url ? (
        <img src={url} alt="" loading="lazy" />
      ) : (
        <div className="manga-cover-art" aria-hidden="true">
          <span>漫</span>
          <small>HQUIZLET LIBRARY</small>
        </div>
      )}
    </div>
  );
}
export function MangaLibrary({
  classId,
  canCreate = true,
}: {
  classId?: number;
  canCreate?: boolean;
}) {
  const { token, user } = useAuth();
  const [volumes, setVolumes] = useState<MangaVolume[]>([]),
    [legacy, setLegacy] = useState<MangaVolume[]>([]),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [loading, setLoading] = useState(true),
    [busy, setBusy] = useState(false),
    [imports, setImports] = useState<MangaImport[]>([]),
    [opened, setOpened] = useState<MangaVolume>(),
    [settings, setSettings] = useState<MangaVolume>();
  const [selectedKey, setSelectedKey] = useState(""),
    [search, setSearch] = useState(""),
    [importOpen, setImportOpen] = useState(false),
    [targetKey, setTargetKey] = useState(""),
    [newTitle, setNewTitle] = useState("");
  const input = useRef<HTMLInputElement>(null),
    folder = useRef<HTMLInputElement>(null),
    abort = useRef<AbortController>(),
    prepareID = useRef(0),
    importPanel = useRef<HTMLDivElement>(null);
  const settingsDialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    if (!settings) return;
    const previous = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    settingsDialog.current?.showModal();
    return () => {
      document.body.style.overflow = overflow;
      previous?.focus();
    };
  }, [!!settings]);
  const [sourceFiles, setSourceFiles] = useState<File[]>([]),
    [imageFiles, setImageFiles] = useState<File[]>([]);
  const series = useMemo(() => groupMangaSeries(volumes), [volumes]);
  const selectedSeries = series.find((s) => s.key === selectedKey),
    targetSeries = series.find((s) => s.key === targetKey);
  const selectableSeries = series.filter(
    (s) => classId || s.ownerId === user?.id,
  );
  const filteredSeries = series.filter((s) =>
    s.title
      .toLocaleLowerCase("vi")
      .includes(search.trim().toLocaleLowerCase("vi")),
  );
  const readerSeries = series.find((s) =>
    s.volumes.some((v) => v.id === opened?.id),
  );
  async function reload() {
    try {
      setVolumes(await mangaApi.list(token, classId));
    } catch (e) {
      setError(message(e));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    void reload();
    return () => {
      abort.current?.abort();
      prepareID.current++;
    };
  }, [token, classId]);
  useEffect(() => {
    if (importOpen) folder.current?.setAttribute("webkitdirectory", "");
  }, [importOpen]);
  useEffect(() => {
    if (classId && canCreate)
      mangaApi
        .list(token)
        .then((items) => setLegacy(items.filter((v) => v.ownerId === user?.id)))
        .catch(() => {});
  }, [token, classId, canCreate, user?.id]);
  function startImport(key = "") {
    if (busy) return;
    setTargetKey(key);
    setNewTitle("");
    setImportOpen(true);
    setError("");
    setImports([]);
    setSourceFiles([]);
    setImageFiles([]);
    prepareID.current++;
    requestAnimationFrame(() =>
      importPanel.current?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      }),
    );
  }
  async function prepare(sources: File[], images: File[]) {
    const id = ++prepareID.current;
    setImports([]);
    setError("");
    if (!sources.length || !images.length) return;
    setNotice("Đang kiểm tra HTML và ảnh…");
    try {
      const parsed = await prepareMangaImport(
        [...sources, ...images.filter((f) => /\.(?:jpe?g|png)$/i.test(f.name))],
        (source, name) => mangaApi.inspect(token, source, name),
      );
      if (id !== prepareID.current) return;
      const pending = parsed.filter(
        (p) =>
          !volumes.some(
            (v) =>
              v.ownerId === user?.id &&
              v.status === "ready" &&
              v.sourceUuid === p.sourceUuid,
          ),
      );
      setImports(pending);
      setNotice(
        pending.length
          ? "Đã kiểm tra file. Đặt tên truyện và tên tập rồi bấm Nhập."
          : "Các tập này đã có trong thư viện.",
      );
    } catch (e) {
      if (id === prepareID.current) setError(message(e));
    }
  }
  async function upload() {
    if (busy || !imports.length) return;
    const title = targetSeries?.title || newTitle.trim();
    if (!title) {
      setError("Nhập tên truyện hoặc chọn truyện có sẵn.");
      return;
    }
    setBusy(true);
    setError("");
    const controller = new AbortController();
    abort.current = controller;
    try {
      for (const volume of imports) {
        if (controller.signal.aborted)
          throw Error(
            "Đã dừng nhập. Chọn lại file để tiếp tục phần đang nhập.",
          );
        const created = await mangaApi.create(
          token,
          {
            title,
            volume: volume.volume,
            sourceUuid: volume.sourceUuid,
            pageCount: volume.pages.length,
            htmlSource: volume.htmlSource,
            sourceName: volume.sourceName,
            seriesId: targetSeries?.id || undefined,
          },
          classId,
        );
        const detail = await mangaApi.detail(token, created.id);
        const existing = new Set(detail.pages);
        for (let i = 0; i < volume.pages.length; i++) {
          if (existing.has(i)) continue;
          setNotice(
            `${title} · ${volume.volume}: đang tải trang ${i + 1}/${volume.pages.length}`,
          );
          await mangaApi.upload(
            token,
            created.id,
            i,
            volume.pages[i].page,
            volume.pages[i].file,
            controller.signal,
          );
        }
        await mangaApi.complete(token, created.id);
        await reload();
      }
      setImports([]);
      setImportOpen(false);
      setNotice("Đã thêm tập vào truyện. Bạn có thể chọn tập để đọc ngay.");
    } catch (e) {
      setError(
        controller.signal.aborted
          ? "Đã dừng nhập. Các trang đã tải được giữ lại để tiếp tục."
          : message(e),
      );
      await reload();
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="manga-library">
      <header className="manga-heading">
        <div>
          <span>TỦ SÁCH TIẾNG NHẬT</span>
          <h1>{classId ? "Truyện của nhóm" : "Thư viện truyện"}</h1>
          <p>
            {classId
              ? "Một góc đọc chung dành cho thành viên nhóm."
              : "Đọc, khám phá và học tiếng Nhật qua từng câu chuyện."}
          </p>
        </div>
        {canCreate && (
          <button
            className="manga-add-story"
            onClick={() => startImport()}
            disabled={busy}
          >
            ＋ Thêm truyện
          </button>
        )}
      </header>
      <div className="manga-library-toolbar">
        <span>
          {series.length} truyện <span aria-hidden="true">·</span>{" "}
          {volumes.length} tập
        </span>
        <label className="manga-library-search">
          <svg
            width="17"
            height="17"
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
            aria-label="Tìm truyện"
            placeholder="Tìm truyện trong thư viện…"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setSelectedKey("");
            }}
          />
        </label>
      </div>
      {error && (
        <p role="alert" className="manga-error">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="manga-notice">
          {notice}
        </p>
      )}
      {importOpen && canCreate && (
        <div className="manga-import-panel" ref={importPanel}>
          <header>
            <div>
              <p className="manga-eyebrow">THÊM NỘI DUNG</p>
              <h2>
                {targetSeries
                  ? `Thêm tập vào ${targetSeries.title}`
                  : "Thêm truyện và tập mới"}
              </h2>
              <p>
                Mỗi file HTML tương ứng một tập. Có thể chọn nhiều tập cùng lúc.
              </p>
            </div>
            <button
              disabled={busy}
              aria-label="Đóng nhập truyện"
              onClick={() => {
                prepareID.current++;
                setImportOpen(false);
                setImports([]);
              }}
            >
              ×
            </button>
          </header>
          <div className="manga-import-target">
            <label>
              Thuộc truyện
              <select
                disabled={busy}
                value={targetKey}
                onChange={(e) => setTargetKey(e.target.value)}
              >
                <option value="">＋ Tạo truyện mới</option>
                {selectableSeries.map((s) => (
                  <option key={s.key} value={s.key}>
                    {s.title} · {s.volumes.length} tập
                  </option>
                ))}
              </select>
            </label>
            {!targetSeries && (
              <label>
                Tên truyện
                <input
                  autoFocus
                  maxLength={200}
                  disabled={busy}
                  value={newTitle}
                  placeholder="Ví dụ: Một câu chuyện ở Tokyo"
                  onChange={(e) => setNewTitle(e.target.value)}
                />
              </label>
            )}
          </div>
          <div className="manga-import-steps">
            <label className="manga-import-step">
              <span>01</span>
              <strong>
                {sourceFiles.length
                  ? `${sourceFiles.length} file đã chọn`
                  : "Chọn tập truyện"}
              </strong>
              <small>HTML hoặc Mokuro · một file cho mỗi tập</small>
              <input
                ref={input}
                type="file"
                multiple
                accept=".html,.htm,.mokuro"
                disabled={busy}
                aria-label="Chọn file HTML truyện"
                onChange={(e) => {
                  const files = Array.from(e.target.files || []);
                  setSourceFiles(files);
                  void prepare(files, imageFiles);
                }}
              />
            </label>
            <label className="manga-import-step">
              <span>02</span>
              <strong>
                {imageFiles.length
                  ? `${imageFiles.length} ảnh đã chọn`
                  : "Chọn thư mục ảnh"}
              </strong>
              <small>Ảnh JPEG/PNG đi kèm các tập đã chọn</small>
              <input
                ref={folder}
                type="file"
                multiple
                disabled={busy}
                aria-label="Chọn thư mục ảnh truyện"
                onChange={(e) => {
                  const files = Array.from(e.target.files || []);
                  setImageFiles(files);
                  void prepare(sourceFiles, files);
                }}
              />
            </label>
          </div>
          {!!imports.length && (
            <div className="manga-import-review">
              <h3>Các tập sẽ thêm</h3>
              {imports.map((v, i) => (
                <div key={v.sourceUuid}>
                  <span className="manga-volume-index">
                    {String(i + 1).padStart(2, "0")}
                  </span>
                  <label>
                    Tên tập
                    <input
                      disabled={busy}
                      maxLength={200}
                      value={v.volume}
                      onChange={(e) =>
                        setImports((all) =>
                          all.map((x, n) =>
                            n === i ? { ...x, volume: e.target.value } : x,
                          ),
                        )
                      }
                    />
                  </label>
                  <small>
                    {v.pages.length} trang ·{" "}
                    {(
                      v.pages.reduce((sum, p) => sum + p.file.size, 0) / 1048576
                    ).toFixed(1)}{" "}
                    MiB
                  </small>
                </div>
              ))}
            </div>
          )}
          <footer>
            <p>
              {classId
                ? "Mặc định chia sẻ với thành viên nhóm."
                : "Mặc định chia sẻ với mọi tài khoản HQuizlet."}{" "}
              File gốc trên máy được giữ nguyên.
            </p>
            {busy ? (
              <button onClick={() => abort.current?.abort()}>Dừng nhập</button>
            ) : (
              <button
                className="manga-add-story"
                disabled={
                  !imports.length ||
                  (!targetSeries && !newTitle.trim()) ||
                  imports.some((v) => !v.volume.trim())
                }
                onClick={() => void upload()}
              >
                Nhập {imports.length || ""} tập
              </button>
            )}
          </footer>
        </div>
      )}
      {classId && canCreate && legacy.length > 0 && (
        <details className="manga-legacy">
          <summary>Đưa tập từ thư viện chung vào nhóm</summary>
          <select
            aria-label="Đưa truyện cũ vào lớp"
            value=""
            disabled={busy}
            onChange={async (e) => {
              const id = Number(e.target.value);
              if (!id) return;
              setBusy(true);
              setError("");
              try {
                await mangaApi.move(token, id, classId);
                setLegacy((all) => all.filter((v) => v.id !== id));
                await reload();
              } catch (e) {
                setError(message(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            <option value="">Chọn tập truyện…</option>
            {legacy.map((v) => (
              <option key={v.id} value={v.id}>
                {v.title} · {v.volume}
              </option>
            ))}
          </select>
        </details>
      )}
      {loading ? (
        <p role="status">Đang tải thư viện…</p>
      ) : !volumes.length ? (
        <div className="manga-empty">
          <span aria-hidden="true">▤</span>
          <h2>Tủ sách đang chờ câu chuyện đầu tiên</h2>
          <p>Thêm một truyện, rồi nhập từng tập hoặc nhiều tập cùng lúc.</p>
          {canCreate && (
            <button onClick={() => startImport()}>
              ＋ Thêm truyện đầu tiên
            </button>
          )}
        </div>
      ) : selectedSeries ? (
        <section className="manga-series-detail">
          <button
            className="manga-back-library"
            onClick={() => setSelectedKey("")}
          >
            ← Tất cả truyện
          </button>
          <header>
            <MangaCover
              token={token}
              volume={selectedSeries.volumes.find((v) => v.status === "ready")}
            />
            <div>
              <p className="manga-eyebrow">DANH SÁCH TẬP</p>
              <h2>{selectedSeries.title}</h2>
              <p>
                {selectedSeries.volumes.length} tập ·{" "}
                {selectedSeries.volumes.reduce(
                  (sum, v) => sum + v.pageCount,
                  0,
                )}{" "}
                trang
              </p>
              {canCreate &&
                (classId || selectedSeries.ownerId === user?.id) && (
                  <button
                    className="manga-add-story"
                    disabled={busy}
                    onClick={() => startImport(selectedSeries.key)}
                  >
                    ＋ Thêm tập
                  </button>
                )}
            </div>
          </header>
          <div className="manga-volume-list">
            {selectedSeries.volumes.map((v, i) => (
              <article key={v.id}>
                <span className="manga-volume-index">
                  {String(i + 1).padStart(2, "0")}
                </span>
                <div>
                  <h3>{volumeLabel(v, i)}</h3>
                  <small>
                    {v.uploaded}/{v.pageCount} trang ·{" "}
                    {v.status === "importing"
                      ? "Đang nhập"
                      : v.visibility === "private"
                        ? "Riêng tư"
                        : classId
                          ? "Thành viên nhóm"
                          : "Công khai"}
                  </small>
                </div>
                <div className="manga-volume-actions">
                  <button
                    disabled={v.status !== "ready"}
                    onClick={() => setOpened(v)}
                  >
                    Đọc tập →
                  </button>
                  {(v.canEdit || v.canDelete) && (
                    <button
                      aria-label={`Cài đặt ${volumeLabel(v, i)}`}
                      onClick={() => setSettings({ ...v })}
                    >
                      ⚙
                    </button>
                  )}
                </div>
              </article>
            ))}
          </div>
        </section>
      ) : (
        <div className="manga-series-grid">
          {filteredSeries.map((s) => (
            <article className="manga-series-card" key={s.key}>
              <button
                className="manga-series-open"
                onClick={() => setSelectedKey(s.key)}
              >
                <MangaCover
                  token={token}
                  volume={s.volumes.find((v) => v.status === "ready")}
                />
                <span className="manga-series-count">
                  {s.volumes.length} tập
                </span>
                <h2>{s.title}</h2>
              </button>
              <p>
                {s.volumes.reduce((sum, v) => sum + v.pageCount, 0)} trang ·{" "}
                {s.volumes.some((v) => v.status === "importing")
                  ? "Có tập đang nhập"
                  : classId
                    ? "Trong nhóm"
                    : "Thư viện chung"}
              </p>
              <footer>
                <button onClick={() => setSelectedKey(s.key)}>
                  Xem các tập →
                </button>
                {canCreate && (classId || s.ownerId === user?.id) && (
                  <button
                    aria-label={`Thêm tập vào ${s.title}`}
                    disabled={busy}
                    onClick={() => startImport(s.key)}
                  >
                    ＋ Thêm tập
                  </button>
                )}
              </footer>
            </article>
          ))}
          {!filteredSeries.length && (
            <div className="manga-empty">
              <h2>Không tìm thấy truyện</h2>
              <p>Thử tên khác hoặc xóa từ khóa tìm kiếm.</p>
              <button onClick={() => setSearch("")}>Xóa tìm kiếm</button>
            </div>
          )}
        </div>
      )}
      {settings && (
        <dialog
          ref={settingsDialog}
          onCancel={(e) => {
            if (busy) e.preventDefault();
          }}
          onClose={() => setSettings(undefined)}
          className="manga-settings"
          aria-label="Cài đặt tập truyện"
        >
          <header>
            <h2>Cài đặt tập truyện</h2>
            <button
              aria-label="Đóng cài đặt"
              disabled={busy}
              onClick={() => setSettings(undefined)}
            >
              ×
            </button>
          </header>
          <label>
            Thuộc truyện
            <select
              disabled={!settings.canEdit || busy}
              value={settings.seriesId || 0}
              onChange={(e) => {
                const target = series.find(
                  (s) => s.id === Number(e.target.value),
                );
                setSettings({
                  ...settings,
                  seriesId: target?.id || 0,
                  title: target?.title || settings.title,
                });
              }}
            >
              <option value={0}>
                ＋ Tạo truyện mới / đổi tên truyện cho tập này
              </option>
              {series
                .filter((s) => s.id > 0 && (classId || s.ownerId === user?.id))
                .map((s) => (
                  <option value={s.id} key={s.key}>
                    {s.title}
                  </option>
                ))}
            </select>
          </label>
          {!settings.seriesId && (
            <label>
              Tên truyện
              <input
                disabled={!settings.canEdit || busy}
                value={settings.title}
                maxLength={200}
                onChange={(e) =>
                  setSettings({ ...settings, title: e.target.value })
                }
              />
            </label>
          )}
          <label>
            Tên tập
            <input
              disabled={!settings.canEdit || busy}
              value={settings.volume}
              maxLength={200}
              onChange={(e) =>
                setSettings({ ...settings, volume: e.target.value })
              }
            />
          </label>
          <label>
            Quyền xem
            <select
              disabled={!settings.canEdit || busy}
              value={settings.visibility}
              onChange={(e) =>
                setSettings({
                  ...settings,
                  visibility: e.target.value as "private" | "shared",
                })
              }
            >
              <option value="private">Riêng tư · chỉ chủ tập</option>
              <option value="shared">
                {classId ? "Thành viên nhóm" : "Mọi người trên HQuizlet"}
              </option>
            </select>
          </label>
          <p>
            Thay đổi áp dụng cho tập này. Các tập khác giữ nguyên quyền xem.
          </p>
          <footer>
            <button
              className="manga-add-story"
              disabled={busy || !settings.canEdit || !settings.title.trim()}
              onClick={async () => {
                setBusy(true);
                setError("");
                try {
                  await mangaApi.update(token, settings);
                  setSettings(undefined);
                  await reload();
                } catch (e) {
                  setError(message(e));
                } finally {
                  setBusy(false);
                }
              }}
            >
              Lưu thay đổi
            </button>
            <button
              className="manga-danger"
              disabled={busy || !settings.canDelete}
              onClick={async () => {
                if (
                  !confirm(
                    "Xóa tập này? Các tập khác trong truyện được giữ nguyên.",
                  )
                )
                  return;
                setBusy(true);
                try {
                  await mangaApi.remove(token, settings.id);
                  setSettings(undefined);
                  await reload();
                } catch (e) {
                  setError(message(e));
                } finally {
                  setBusy(false);
                }
              }}
            >
              Xóa tập
            </button>
          </footer>
        </dialog>
      )}
      {opened && (
        <MangaReader
          key={opened.id}
          token={token}
          volume={opened}
          volumeOptions={readerSeries?.volumes.filter(
            (v) => v.status === "ready",
          )}
          onSelectVolume={(v) => setOpened(v)}
          onClose={() => setOpened(undefined)}
        />
      )}
    </section>
  );
}
function MangaImage({
  token,
  volume,
  index,
  onSelect,
  version,
}: {
  token: string;
  volume: MangaVolume;
  index: number;
  onSelect: (index: number, data: MangaPageResponse, block: number) => void;
  version: number;
}) {
  const [data, setData] = useState<MangaPageResponse>(),
    [url, setURL] = useState(""),
    [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    let local = "";
    setData(undefined);
    setURL("");
    setError("");
    Promise.all([
      mangaApi.page(token, volume.id, index, controller.signal),
      mangaApi.image(token, volume.id, index, controller.signal),
    ])
      .then(([page, blob]) => {
        if (controller.signal.aborted) return;
        local = URL.createObjectURL(blob);
        setData(page);
        setURL(local);
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(message(e));
      });
    return () => {
      controller.abort();
      if (local) URL.revokeObjectURL(local);
    };
  }, [token, volume.id, index, version]);
  if (error) return <p role="alert">{error}</p>;
  if (!data || !url)
    return (
      <p role="status" className="manga-image-loading">
        Đang tải trang {index + 1}…
      </p>
    );
  return (
    <figure
      className="manga-page"
      style={
        {
          aspectRatio: `${data.page.img_width}/${data.page.img_height}`,
          "--page-ratio": data.page.img_width / data.page.img_height,
        } as import("react").CSSProperties
      }
    >
      <img src={url} alt={`Trang ${index + 1}`} draggable={false} />
      {data.page.blocks.map((b, i) => (
        <button
          key={i}
          className="manga-region"
          aria-label={`Xem chữ OCR trang ${index + 1}, khối ${i + 1}`}
          onClick={() => onSelect(index, data, i)}
          style={{
            left: `${(b.box[0] / data.page.img_width) * 100}%`,
            top: `${(b.box[1] / data.page.img_height) * 100}%`,
            width: `${((b.box[2] - b.box[0]) / data.page.img_width) * 100}%`,
            height: `${((b.box[3] - b.box[1]) / data.page.img_height) * 100}%`,
          }}
        >
          <span lang="ja">{b.lines.join("\n")}</span>
        </button>
      ))}
      <figcaption>Trang {index + 1}</figcaption>
    </figure>
  );
}
export function MangaReader({
  token,
  volume,
  onClose,
  volumeOptions,
  onSelectVolume,
}: {
  token: string;
  volume: MangaVolume;
  onClose: () => void;
  volumeOptions?: MangaVolume[];
  onSelectVolume?: (volume: MangaVolume) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const canvas = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = canvas.current;
    if (!el) return;
    const observer = new ResizeObserver(() => {
      const style = getComputedStyle(el);
      el.style.setProperty(
        "--canvas-height",
        `${el.clientHeight - parseFloat(style.paddingTop) - parseFloat(style.paddingBottom)}px`,
      );
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  const [index, setIndex] = useState(() => {
      const n = Number(localStorage.getItem(`manga-page-${volume.id}`));
      return Number.isInteger(n) && n >= 0 && n < volume.pageCount ? n : 0;
    }),
    [spread, setSpread] = useState(innerWidth >= 900 ? 2 : 1),
    [rtl, setRTL] = useState(true),
    [zoom, setZoom] = useState(0),
    [selection, setSelection] = useState<{
      index: number;
      data: MangaPageResponse;
      block: number;
    }>(),
    [text, setText] = useState(""),
    [dirty, setDirty] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false),
    [version, setVersion] = useState(0);
  const safeLeave = () =>
    !dirty || confirm("Chữ OCR chưa lưu. Bỏ thay đổi để tiếp tục?");
  function close() {
    if (!busy && safeLeave()) onClose();
  }
  function navigate(next: number) {
    if (busy || next < 0 || next >= volume.pageCount || !safeLeave()) return;
    setIndex(next);
    setSelection(undefined);
    setDirty(false);
    setNotice("");
    setError("");
    dialog.current?.querySelector(".manga-reader-canvas")?.scrollTo({ top: 0 });
  }
  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    dialog.current?.showModal();
    document.body.style.overflow = "hidden";
    return () => {
      dialog.current?.close();
      document.body.style.overflow = overflow;
      prev?.focus();
    };
  }, []);
  useEffect(() => {
    localStorage.setItem(`manga-page-${volume.id}`, String(index));
  }, [volume.id, index]);
  useEffect(() => {
    const unload = (e: BeforeUnloadEvent) => {
      if (dirty || busy) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", unload);
    return () => window.removeEventListener("beforeunload", unload);
  }, [dirty, busy]);
  async function save(restore = false) {
    if (!selection || busy) return;
    if (restore && !confirm("Khôi phục toàn bộ chữ OCR gốc của trang này?"))
      return;
    setBusy(true);
    setError("");
    try {
      const lines = selection.data.page.blocks.map((b, i) =>
        i === selection.block ? text.split("\n") : b.lines,
      );
      const result = await mangaApi.edit(
        token,
        volume.id,
        selection.index,
        selection.data.revision,
        lines,
        restore,
      );
      setSelection({ ...selection, data: result });
      setText(result.page.blocks[selection.block]?.lines.join("\n") || "");
      setDirty(false);
      setVersion((v) => v + 1);
      setNotice(
        restore ? "Đã khôi phục OCR gốc của trang." : "Đã lưu chữ OCR.",
      );
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return createPortal(
    <dialog
      className="manga-reader"
      ref={dialog}
      aria-labelledby="manga-reader-title"
      onCancel={(e) => {
        e.preventDefault();
        close();
      }}
      onKeyDown={(e) => {
        if ((e.target as HTMLElement).closest("button,input,select,textarea"))
          return;
        if (e.key === "ArrowRight") {
          e.preventDefault();
          navigate(index + (rtl ? -spread : spread));
        }
        if (e.key === "ArrowLeft") {
          e.preventDefault();
          navigate(index + (rtl ? spread : -spread));
        }
      }}
    >
      <header>
        <button aria-label="Đóng truyện" onClick={close}>
          ✕
        </button>
        <div>
          <h2 id="manga-reader-title">{volume.title}</h2>
          <small>{volume.volume}</small>
        </div>
        {volumeOptions && volumeOptions.length > 1 && (
          <label>
            <span className="manga-control-label">Tập</span>
            <select
              aria-label="Chọn tập truyện"
              disabled={busy}
              value={volume.id}
              onChange={(e) => {
                const next = volumeOptions.find(
                  (v) => v.id === Number(e.target.value),
                );
                if (next && safeLeave()) onSelectVolume?.(next);
              }}
            >
              {volumeOptions.map((v, i) => (
                <option value={v.id} key={v.id}>
                  {volumeLabel(v, i)}
                </option>
              ))}
            </select>
          </label>
        )}
        <nav aria-label="Điều khiển trang">
          <button disabled={busy || index === 0} onClick={() => navigate(0)}>
            «
          </button>
          <button
            aria-label="Trang trước"
            disabled={busy || index === 0}
            onClick={() => navigate(Math.max(0, index - spread))}
          >
            ‹
          </button>
          <label>
            Trang
            <input
              aria-label="Số trang"
              type="number"
              min={1}
              max={volume.pageCount}
              value={index + 1}
              onChange={(e) => navigate(Number(e.target.value) - 1)}
            />
          </label>
          <span>/ {volume.pageCount}</span>
          <button
            aria-label="Trang tiếp theo"
            disabled={busy || index + spread >= volume.pageCount}
            onClick={() => navigate(index + spread)}
          >
            ›
          </button>
          <button
            disabled={busy || index === volume.pageCount - 1}
            onClick={() => navigate(volume.pageCount - 1)}
          >
            »
          </button>
        </nav>
        <label>
          <span className="manga-control-label">Chế độ</span>
          <select
            aria-label="Chế độ trang"
            value={spread}
            onChange={(e) => {
              if (safeLeave()) {
                setSpread(Number(e.target.value));
                setSelection(undefined);
                setDirty(false);
              }
            }}
          >
            <option value={1}>Một trang</option>
            <option value={2}>Hai trang</option>
          </select>
        </label>
        <button aria-pressed={rtl} onClick={() => setRTL(!rtl)}>
          {rtl ? "Phải → trái" : "Trái → phải"}
        </button>
        <button
          disabled={busy}
          onClick={async () => {
            if (
              dirty &&
              !confirm(
                "Chữ chưa lưu sẽ không có trong file xuất. Tiếp tục tải bản đã lưu?",
              )
            )
              return;
            setBusy(true);
            setError("");
            try {
              const blob = await mangaApi.exportHTML(token, volume.id);
              const url = URL.createObjectURL(blob);
              const a = document.createElement("a");
              a.href = url;
              a.download = volume.volume + "-edited.html";
              a.click();
              setTimeout(() => URL.revokeObjectURL(url), 1000);
            } catch (e) {
              setError(message(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          ↓ HTML đã sửa
        </button>
        <label>
          <span className="manga-control-label">Thu phóng</span>
          <select
            aria-label="Thu phóng"
            value={zoom}
            onChange={(e) => setZoom(Number(e.target.value))}
          >
            <option value={0}>Vừa trang</option>
            {[75, 100, 125, 150, 200].map((n) => (
              <option key={n} value={n}>
                {n}%
              </option>
            ))}
          </select>
        </label>
      </header>
      {error && !selection && (
        <p role="alert" className="manga-error">
          {error}
        </p>
      )}
      <div
        className={`manga-reader-workspace ${selection ? "has-editor" : ""}`}
      >
        <div className="manga-reader-canvas" ref={canvas}>
          <div
            className={`manga-spread ${spread === 2 ? "double" : ""} ${zoom === 0 ? "fit-page" : ""}`}
            style={{
              width: zoom === 0 ? "100%" : `${zoom}%`,
              direction: rtl ? "rtl" : "ltr",
            }}
          >
            {Array.from(
              { length: Math.min(spread, volume.pageCount - index) },
              (_, i) => (
                <MangaImage
                  key={index + i}
                  token={token}
                  volume={volume}
                  index={index + i}
                  version={version}
                  onSelect={(n, data, block) => {
                    if (busy || !safeLeave()) return;
                    setSelection({ index: n, data, block });
                    setText(data.page.blocks[block].lines.join("\n"));
                    setDirty(false);
                    setError("");
                    setNotice("");
                  }}
                />
              ),
            )}
          </div>
        </div>
        {selection && (
          <aside className="manga-ocr-editor">
            <header>
              <h3>Chữ OCR · trang {selection.index + 1}</h3>
              <button
                aria-label="Đóng bảng OCR"
                onClick={() => {
                  if (!busy && safeLeave()) {
                    setSelection(undefined);
                    setDirty(false);
                  }
                }}
              >
                ×
              </button>
            </header>
            <label>
              Khung thoại
              <select
                disabled={busy}
                value={selection.block}
                onChange={(e) => {
                  if (!safeLeave()) return;
                  const n = Number(e.target.value);
                  setSelection({ ...selection, block: n });
                  setText(selection.data.page.blocks[n].lines.join("\n"));
                  setDirty(false);
                }}
              >
                {selection.data.page.blocks.map((_, i) => (
                  <option value={i} key={i}>
                    Khối {i + 1}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Nội dung tiếng Nhật
              <textarea
                lang="ja"
                value={text}
                rows={10}
                readOnly={!volume.canEdit}
                disabled={busy}
                maxLength={100000}
                onChange={(e) => {
                  setText(e.target.value);
                  setDirty(true);
                  setNotice("");
                }}
              />
            </label>
            <p>
              {volume.canEdit
                ? "Chỉ sửa chữ OCR; ảnh gốc và vị trí khung thoại được giữ nguyên. Mỗi dòng là một dòng OCR."
                : "Bạn có thể chọn và sao chép chữ. Bạn cần quyền chỉnh sửa truyện để lưu thay đổi."}
            </p>
            {volume.canEdit && (
              <>
                <button
                  className="manga-save-ocr"
                  disabled={busy || !dirty}
                  onClick={() => void save()}
                >
                  {busy ? "Đang lưu…" : "Lưu chữ OCR"}
                </button>
                <button disabled={busy} onClick={() => void save(true)}>
                  Khôi phục OCR gốc của trang
                </button>
              </>
            )}
            {error && (
              <p role="alert" className="manga-error">
                {error}
              </p>
            )}
            {notice && (
              <p role="status" className="manga-notice">
                {notice}
              </p>
            )}
            <small>
              Phiên bản trang: {selection.data.revision}
              {dirty ? " · Chưa lưu" : ""}
            </small>
          </aside>
        )}
      </div>
    </dialog>,
    document.body,
  );
}
