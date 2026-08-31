// StudySetEditor — Dev 3 [P2-WEB-02]
// Fix P0-04: dùng POST /v1/study-sets/{id}/flashcards/bulk thay vì sequential requests.
// Bulk payload: id=0 → create, id>0 → update, delete=true → delete.

import React, { useState } from "react";
import type { StudySet, DraftCard } from "../../types";
import { useAuth } from "../auth/AuthContext";
import { studySetApi, flashcardApi } from "../../lib/api";

type Props = {
  existingSet?: StudySet;
  onSave: (set: StudySet) => void;
  onCancel: () => void;
};

function newDraftCard(): DraftCard {
  return { key: crypto.randomUUID(), term: "", definition: "" };
}

function emptyDraftCards(): DraftCard[] {
  return [newDraftCard(), newDraftCard(), newDraftCard()];
}

function toDraftCards(cards: StudySet["flashcards"]): DraftCard[] {
  if (!cards?.length) return emptyDraftCards();
  return cards.map((c) => ({
    key: String(c.id),
    id: c.id,
    term: c.term,
    definition: c.definition,
    starred: c.starred,
  }));
}

export function StudySetEditor({ existingSet, onSave, onCancel }: Props) {
  const { token } = useAuth();
  const isEditing = Boolean(existingSet);
  const [title, setTitle] = useState(existingSet?.title ?? "");
  const [description, setDescription] = useState(existingSet?.description ?? "");
  const [draftCards, setDraftCards] = useState<DraftCard[]>(() =>
    toDraftCards(existingSet?.flashcards)
  );
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  function updateCard(key: string, field: "term" | "definition", value: string) {
    setDraftCards((prev) =>
      prev.map((c) => (c.key === key ? { ...c, [field]: value } : c))
    );
  }

  function addCard() {
    setDraftCards((prev) => [...prev, newDraftCard()]);
  }

  function removeCard(key: string) {
    setDraftCards((prev) => (prev.length === 1 ? prev : prev.filter((c) => c.key !== key)));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    const cleanCards = draftCards
      .map((c) => ({ ...c, term: c.term.trim(), definition: c.definition.trim() }))
      .filter((c) => c.term || c.definition);

    if (!title.trim()) {
      setError("Cần nhập tiêu đề bộ thẻ.");
      return;
    }
    if (cleanCards.length === 0) {
      setError("Cần ít nhất một thẻ.");
      return;
    }
    if (cleanCards.some((c) => !c.term || !c.definition)) {
      setError("Mỗi thẻ cần có đủ thuật ngữ và định nghĩa.");
      return;
    }

    setLoading(true);
    try {
      // Step 1: create or update study set header
      const saved = existingSet
        ? await studySetApi.update(token, existingSet.id, {
            title: title.trim(),
            description: description.trim(),
          })
        : await studySetApi.create(token, {
            title: title.trim(),
            description: description.trim(),
          });

      // Step 2: P0-04 fix — single bulk request instead of sequential create/update/delete.
      // id=0 → create; id>0 → update; delete=true → delete removed cards.
      const existingCardIds = new Set((existingSet?.flashcards ?? []).map((c) => c.id));

      // Cards still present in the form (create or update)
      const keepItems = cleanCards.map((card, idx) => ({
        id: card.id ?? 0,
        term: card.term,
        definition: card.definition,
        position: idx,
        delete: false,
      }));

      // Cards that were in existingSet but no longer in the form → delete
      const keptIds = new Set(cleanCards.filter((c) => c.id).map((c) => c.id!));
      const deleteItems = [...existingCardIds]
        .filter((id) => !keptIds.has(id))
        .map((id) => ({
          id,
          term: "",
          definition: "",
          position: 0,
          delete: true,
        }));

      await flashcardApi.bulkSave(token, saved.id, {
        cards: [...keepItems, ...deleteItems],
      });

      // Step 3: fetch updated set with latest flashcards
      const updated = await studySetApi.get(token, saved.id);
      onSave(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Không lưu được học phần.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <form className="create-page" onSubmit={handleSubmit}>
      <section className="create-header">
        <div>
          <p className="eyebrow">{isEditing ? "Sửa học phần" : "Tạo học phần mới"}</p>
          <h1>{isEditing ? "Cập nhật thẻ học" : "Tạo thẻ học"}</h1>
        </div>
        <div className="header-actions">
          <button className="ghost-button" type="button" onClick={onCancel}>
            Hủy
          </button>
          <button className="primary-button" disabled={loading} type="submit">
            {loading ? "Đang lưu..." : isEditing ? "Lưu thay đổi" : "Tạo học phần"}
          </button>
        </div>
      </section>

      <section className="create-meta">
        <label>
          Tiêu đề
          <input
            autoFocus
            placeholder="Ví dụ: English Vocabulary Unit 1"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
        </label>
        <label>
          Mô tả
          <textarea
            placeholder="Mô tả ngắn về nội dung bộ thẻ"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
      </section>

      {error && <p className="message message--error">{error}</p>}

      <section className="cards-editor">
        <div className="cards-editor-heading">
          <div>
            <p className="eyebrow">Cards</p>
            <h2>Thuật ngữ và định nghĩa</h2>
          </div>
          <button className="secondary-button" type="button" onClick={addCard}>
            + Thêm thẻ
          </button>
        </div>

        {draftCards.map((card, index) => (
          <article className="draft-card" key={card.key}>
            <div className="draft-index">{index + 1}</div>
            <label>
              Thuật ngữ
              <input
                placeholder="apple"
                value={card.term}
                onChange={(e) => updateCard(card.key, "term", e.target.value)}
              />
            </label>
            <label>
              Định nghĩa
              <input
                placeholder="quả táo"
                value={card.definition}
                onChange={(e) => updateCard(card.key, "definition", e.target.value)}
              />
            </label>
            <button
              className="icon-button"
              type="button"
              onClick={() => removeCard(card.key)}
              aria-label="Xóa thẻ"
            >
              ×
            </button>
          </article>
        ))}

        <button className="add-row" type="button" onClick={addCard}>
          + Thêm một thẻ nữa
        </button>
      </section>
    </form>
  );
}
