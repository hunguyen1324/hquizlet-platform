// StudySetEditor - Dev 3 (FE-CORE-06)
// Gọi API thật POST/PUT /v1/study-sets và flashcards qua gateway.

import React, { useState } from "react";
import type { StudySet, DraftCard, Flashcard } from "../../types";
import { useAuth, apiFetch } from "../auth/AuthContext";

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
  return cards.map((c) => ({ key: String(c.id), id: c.id, term: c.term, definition: c.definition, starred: c.starred }));
}

export function StudySetEditor({ existingSet, onSave, onCancel }: Props) {
  const { token } = useAuth();
  const isEditing = Boolean(existingSet);
  const [title, setTitle] = useState(existingSet?.title ?? "");
  const [description, setDescription] = useState(existingSet?.description ?? "");
  const [draftCards, setDraftCards] = useState<DraftCard[]>(() => toDraftCards(existingSet?.flashcards));
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  function updateCard(key: string, field: "term" | "definition", value: string) {
    setDraftCards((prev) => prev.map((c) => c.key === key ? { ...c, [field]: value } : c));
  }

  function addCard() { setDraftCards((prev) => [...prev, newDraftCard()]); }

  function removeCard(key: string) {
    setDraftCards((prev) => prev.length === 1 ? prev : prev.filter((c) => c.key !== key));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    const cleanCards = draftCards
      .map((c) => ({ ...c, term: c.term.trim(), definition: c.definition.trim() }))
      .filter((c) => c.term || c.definition);

    if (!title.trim()) { setError("Cần nhập tiêu đề bộ thẻ."); return; }
    if (cleanCards.length === 0) { setError("Cần ít nhất một thẻ."); return; }
    if (cleanCards.some((c) => !c.term || !c.definition)) {
      setError("Mỗi thẻ cần có đủ thuật ngữ và định nghĩa.");
      return;
    }

    setLoading(true);
    try {
      // Create or update study set
      const payload = JSON.stringify({ title: title.trim(), description: description.trim() });
      const saved = existingSet
        ? await apiFetch<StudySet>(`/v1/study-sets/${existingSet.id}`, token, { method: "PUT", body: payload })
        : await apiFetch<StudySet>("/v1/study-sets", token, { method: "POST", body: payload });

      // Sync flashcards: update existing, create new, delete removed
      const existingIds = new Set((existingSet?.flashcards ?? []).map((c) => c.id));
      for (const card of cleanCards) {
        if (card.id) {
          existingIds.delete(card.id);
          await apiFetch(`/v1/flashcards/${card.id}`, token, {
            method: "PUT",
            body: JSON.stringify({ term: card.term, definition: card.definition }),
          });
        } else {
          await apiFetch(`/v1/study-sets/${saved.id}/flashcards`, token, {
            method: "POST",
            body: JSON.stringify({ term: card.term, definition: card.definition }),
          });
        }
      }
      for (const id of existingIds) {
        await apiFetch(`/v1/flashcards/${id}`, token, { method: "DELETE" });
      }

      // Fetch updated set with flashcards
      const updated = await apiFetch<StudySet>(`/v1/study-sets/${saved.id}`, token);
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
          <button className="ghost-button" type="button" onClick={onCancel}>Hủy</button>
          <button className="primary-button" disabled={loading} type="submit">
            {loading ? "Đang lưu..." : isEditing ? "Lưu thay đổi" : "Tạo học phần"}
          </button>
        </div>
      </section>

      <section className="create-meta">
        <label>
          Tiêu đề
          <input autoFocus placeholder="Ví dụ: English Vocabulary Unit 1" value={title} onChange={(e) => setTitle(e.target.value)} />
        </label>
        <label>
          Mô tả
          <textarea placeholder="Mô tả ngắn về nội dung bộ thẻ" value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
      </section>

      {error && <p className="message message--error">{error}</p>}

      <section className="cards-editor">
        <div className="cards-editor-heading">
          <div><p className="eyebrow">Cards</p><h2>Thuật ngữ và định nghĩa</h2></div>
          <button className="secondary-button" type="button" onClick={addCard}>+ Thêm thẻ</button>
        </div>

        {draftCards.map((card, index) => (
          <article className="draft-card" key={card.key}>
            <div className="draft-index">{index + 1}</div>
            <label>
              Thuật ngữ
              <input placeholder="apple" value={card.term} onChange={(e) => updateCard(card.key, "term", e.target.value)} />
            </label>
            <label>
              Định nghĩa
              <input placeholder="quả táo" value={card.definition} onChange={(e) => updateCard(card.key, "definition", e.target.value)} />
            </label>
            <button className="icon-button" type="button" onClick={() => removeCard(card.key)} aria-label="Xóa thẻ">×</button>
          </article>
        ))}

        <button className="add-row" type="button" onClick={addCard}>+ Thêm một thẻ nữa</button>
      </section>
    </form>
  );
}
