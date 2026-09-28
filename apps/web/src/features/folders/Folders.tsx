// features/folders/Folders.tsx — refactor: tạo thư mục dùng CreateFolderDialog
// Logic giống hquizlet gốc: list → modal tạo/sửa, detail inline.

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, folderApi, studySetApi } from "../../lib/api";
import type { FolderDetail, FolderSummary } from "../../lib/api";
import type { StudySet } from "../../types";
import { useAuth } from "../auth/AuthContext";
import { CreateFolderDialog } from "./CreateFolderDialog";

type Props = { onBack: () => void; onOpenSet: (id: number) => void };
type Mode = "list" | "detail";

export function Folders({ onBack, onOpenSet }: Props) {
  const { token, logout } = useAuth();
  const [mode, setMode] = useState<Mode>("list");
  const [folders, setFolders] = useState<FolderSummary[]>([]);
  const [folder, setFolder] = useState<FolderDetail | null>(null);
  const [sets, setSets] = useState<StudySet[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Dialog state — tạo thư mục mới
  const [createOpen, setCreateOpen] = useState(false);
  // Dialog state — sửa thư mục hiện tại
  const [editOpen, setEditOpen] = useState(false);

  const handleError = useCallback(
    (value: unknown) => {
      if (value instanceof ApiError && value.status === 401) {
        void logout();
        return;
      }
      setError(value instanceof Error ? value.message : "Không thể hoàn tất thao tác.");
    },
    [logout],
  );

  const loadFolders = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      setFolders(await folderApi.listFolders(token));
    } catch (value) {
      handleError(value);
    } finally {
      setLoading(false);
    }
  }, [handleError, token]);

  useEffect(() => {
    if (mode === "list") void loadFolders();
  }, [loadFolders, mode]);

  async function openFolder(id: number) {
    setLoading(true);
    setError("");
    try {
      setFolder(await folderApi.getFolder(token, id));
      setMode("detail");
    } catch (value) {
      handleError(value);
    } finally {
      setLoading(false);
    }
  }

  async function openAddDialog() {
    setError("");
    try {
      const result = await studySetApi.list(token, { per_page: 100 });
      setSets(result.items ?? []);
    } catch (value) {
      handleError(value);
      return;
    }
    const dialog = document.getElementById("add-study-set-dialog") as HTMLDialogElement | null;
    dialog?.showModal();
  }

  async function addSet(studySetId: number) {
    if (!folder) return;
    try {
      await folderApi.addStudySetToFolder(token, folder.id, studySetId);
      (
        document.getElementById("add-study-set-dialog") as HTMLDialogElement | null
      )?.close();
      await openFolder(folder.id);
    } catch (value) {
      handleError(value);
    }
  }

  async function removeSet(studySetId: number) {
    if (
      !folder ||
      !window.confirm("Gỡ học phần khỏi thư mục? Học phần gốc sẽ không bị xóa.")
    )
      return;
    try {
      await folderApi.removeStudySetFromFolder(token, folder.id, studySetId);
      await openFolder(folder.id);
    } catch (value) {
      handleError(value);
    }
  }

  async function handleDeleteFolder() {
    if (
      !folder ||
      !window.confirm(`Xóa thư mục "${folder.title}"? Các học phần gốc vẫn được giữ lại.`)
    )
      return;
    try {
      await folderApi.deleteFolder(token, folder.id);
      setFolder(null);
      setMode("list");
    } catch (value) {
      handleError(value);
    }
  }

  const included = useMemo(
    () => new Set((folder?.studySets ?? []).map((set) => set.id)),
    [folder],
  );

  // --- Detail view ---
  if (mode === "detail" && folder) {
    return (
      <section className="folder-page">
        <div className="page-heading">
          <div>
            <p className="eyebrow">Thư mục</p>
            <h1>{folder.title}</h1>
            <p>{folder.description || "Chưa có mô tả"}</p>
          </div>
          <div className="folder-actions">
            <button
              className="ghost-button"
              onClick={() => setEditOpen(true)}
            >
              Sửa
            </button>
            <button className="danger" onClick={() => void handleDeleteFolder()}>
              Xóa
            </button>
          </div>
        </div>

        <div className="folder-actions">
          <button className="ghost-button" onClick={() => setMode("list")}>
            ← Tất cả thư mục
          </button>
          <button className="primary-button" onClick={() => void openAddDialog()}>
            Thêm học phần
          </button>
        </div>

        {error && (
          <p className="message message--error" role="alert">
            {error}
          </p>
        )}

        <section className="set-grid folder-grid">
          {(folder.studySets ?? []).length === 0 ? (
            <div className="empty-panel">
              <h2>Thư mục còn trống</h2>
              <p>Thêm một học phần để bắt đầu sắp xếp thư viện.</p>
              <button className="primary-button" onClick={() => void openAddDialog()}>
                Thêm học phần
              </button>
            </div>
          ) : (
            (folder.studySets ?? []).map((set) => (
              <article className="set-card" key={set.id}>
                <span>{set.flashcardCount ?? 0} thẻ</span>
                <strong>{set.title}</strong>
                <small>{set.description || "Chưa có mô tả"}</small>
                <div className="folder-actions">
                  <button className="secondary-button" onClick={() => onOpenSet(set.id)}>
                    Mở
                  </button>
                  <button className="ghost-button" onClick={() => void removeSet(set.id)}>
                    Gỡ
                  </button>
                </div>
              </article>
            ))
          )}
        </section>

        {/* Dialog thêm học phần vào thư mục */}
        <dialog id="add-study-set-dialog" className="folder-dialog">
          <form method="dialog">
            <div className="page-heading">
              <div>
                <h2>Thêm học phần</h2>
                <p>Chọn một học phần thuộc tài khoản của bạn.</p>
              </div>
              <button className="ghost-button" aria-label="Đóng">
                ×
              </button>
            </div>
          </form>
          <div className="stack">
            {sets.map((set) => (
              <button
                className="mini-card"
                key={set.id}
                disabled={included.has(set.id)}
                onClick={() => void addSet(set.id)}
              >
                <span>{set.title}</span>
                <strong>{included.has(set.id) ? "Đã thêm" : "Thêm"}</strong>
              </button>
            ))}
          </div>
        </dialog>

        {/* Dialog sửa thư mục */}
        <CreateFolderDialog
          open={editOpen}
          onOpenChange={setEditOpen}
          defaultValues={{
            id: folder.id,
            title: folder.title,
            description: folder.description,
          }}
          onSuccess={async (updated) => {
            // Reload folder detail với title/description mới
            await openFolder(updated.id);
          }}
        />
      </section>
    );
  }

  // --- List view ---
  return (
    <section className="folder-page">
      <div className="page-heading">
        <div>
          <p className="eyebrow">Thư viện</p>
          <h1>Thư mục</h1>
          <p>Sắp xếp học phần theo chủ đề mà không thay đổi dữ liệu gốc.</p>
        </div>
        <button
          className="primary-button"
          onClick={() => setCreateOpen(true)}
        >
          Tạo thư mục
        </button>
      </div>

      <button className="ghost-button" onClick={onBack}>
        ← Học phần
      </button>

      {error && (
        <p className="message message--error" role="alert">
          {error}{" "}
          <button className="ghost-button" onClick={() => void loadFolders()}>
            Thử lại
          </button>
        </p>
      )}

      <section className="set-grid folder-grid">
        {loading ? (
          <div className="loading-skeleton" aria-busy="true">
            {[1, 2, 3].map((id) => (
              <div className="skeleton-row" key={id} />
            ))}
          </div>
        ) : folders.length === 0 && !error ? (
          <div className="empty-panel">
            <h2>Chưa có thư mục</h2>
            <p>Tạo thư mục đầu tiên để nhóm các học phần liên quan.</p>
            <button className="primary-button" onClick={() => setCreateOpen(true)}>
              Tạo thư mục
            </button>
          </div>
        ) : (
          folders.map((item) => (
            <button
              className="set-card"
              key={item.id}
              onClick={() => void openFolder(item.id)}
            >
              <span>{item.studySetCount} học phần</span>
              <strong>{item.title}</strong>
              <small>{item.description || "Chưa có mô tả"}</small>
            </button>
          ))
        )}
      </section>

      {/* Dialog tạo thư mục mới */}
      <CreateFolderDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onSuccess={async (created) => {
          // Sau khi tạo xong, mở ngay folder vừa tạo (như hquizlet)
          await openFolder(created.id);
        }}
      />
    </section>
  );
}
