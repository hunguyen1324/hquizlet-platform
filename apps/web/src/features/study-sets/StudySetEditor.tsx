// StudySetEditor — Dev 3 [P2-WEB-02]
// Tạo/sửa study set và flashcards qua studySetApi + transactional bulkSave.

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
  return {
    key: crypto.randomUUID(),
    term: "",
    definition: "",
    exampleSentence: "",
    hintExplanation: "",
    synonyms: "",
    imageUrl: "",
  };
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
    exampleSentence: c.exampleSentence ?? "",
    hintExplanation: c.hintExplanation ?? "",
    synonyms: c.synonyms ?? "",
    imageUrl: c.imageUrl ?? "",
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

  function updateCard(
    key: string,
    field: "term" | "definition" | "exampleSentence" | "hintExplanation" | "synonyms" | "imageUrl",
    value: string
  ) {
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
      .map((c) => ({
        ...c,
        term: c.term.trim(),
        definition: c.definition.trim(),
        exampleSentence: c.exampleSentence?.trim() ?? "",
        hintExplanation: c.hintExplanation?.trim() ?? "",
        synonyms: c.synonyms?.trim() ?? "",
        imageUrl: c.imageUrl?.trim() || null,
      }))
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
      const saved = existingSet
        ? await studySetApi.update(token, existingSet.id, {
            title: title.trim(),
            description: description.trim(),
          })
        : await studySetApi.create(token, {
            title: title.trim(),
            description: description.trim(),
          });

      const keepItems = cleanCards.map((card, position) => ({
        ...(card.id ? { id: card.id } : {}),
        term: card.term,
        definition: card.definition,
        exampleSentence: card.exampleSentence,
        hintExplanation: card.hintExplanation,
        synonyms: card.synonyms,
        imageUrl: card.imageUrl,
        position,
      }));

      const keptIds = new Set(
        cleanCards
          .map((card) => card.id)
          .filter((id): id is number => typeof id === "number")
      );

      const deleteItems = (existingSet?.flashcards ?? [])
        .filter((card) => !keptIds.has(card.id))
        .map((card) => ({
          id: card.id,
          term: card.term,
          definition: card.definition,
          delete: true,
        }));

      await flashcardApi.bulkSave(
        token,
        saved.id,
        [...keepItems, ...deleteItems]
      );

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
            <div className="draft-card-header">
              <div className="draft-title">
                <span className="draft-number">{index + 1}</span>
                <strong>Flashcard</strong>
              </div>
              <button
                className="icon-button"
                type="button"
                onClick={() => removeCard(card.key)}
                aria-label="Xóa thẻ"
              >
                ×
              </button>
            </div>
            <div className="draft-main-fields">
              <label>
                Term
                <input
                  placeholder="2+2"
                  value={card.term}
                  onChange={(e) => updateCard(card.key, "term", e.target.value)}
                />
              </label>
              <label>
                Definition
                <input
                  placeholder="4"
                  value={card.definition}
                  onChange={(e) => updateCard(card.key, "definition", e.target.value)}
                />
              </label>
            </div>
            <div className="draft-extra-fields">
              <label>
                Example sentence
                <textarea
                  placeholder="I ____ to school every day."
                  value={card.exampleSentence ?? ""}
                  onChange={(e) => updateCard(card.key, "exampleSentence", e.target.value)}
                />
                <span>Use a blank in the sentence for Learn mode.</span>
              </label>
              <label>
                Hint / explanation
                <textarea
                  placeholder="Giải thích ngắn bằng tiếng Việt..."
                  value={card.hintExplanation ?? ""}
                  onChange={(e) => updateCard(card.key, "hintExplanation", e.target.value)}
                />
                <span>Optional explanation for the card.</span>
              </label>
              <label>
                Từ đồng nghĩa (Synonyms)
                <textarea
                  placeholder="VD: happy -> joyful, cheerful, glad"
                  value={card.synonyms ?? ""}
                  onChange={(e) => updateCard(card.key, "synonyms", e.target.value)}
                />
                <span>Từ đồng nghĩa hoặc cách diễn đạt tương đương.</span>
              </label>
            </div>
            <label className="draft-image-field">
              Image (Optional)
              <input
                placeholder="/api/storage/flashcards/... or external URL"
                value={card.imageUrl ?? ""}
                onChange={(e) => updateCard(card.key, "imageUrl", e.target.value)}
              />
            </label>
          </article>
        ))}

        <button className="add-row" type="button" onClick={addCard}>
          + Thêm một thẻ nữa
        </button>
      </section>
    </form>
  );
}
