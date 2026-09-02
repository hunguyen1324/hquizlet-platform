// StudySetEditor — Dev 3 [P2-WEB-02]
// Tạo/sửa study set và flashcards qua studySetApi + transactional bulkSave.
// Phase 10: Thêm language selector, visibility toggle, import Excel.

import React, { useState } from "react";
import type { StudySet, DraftCard } from "../../types";
import { useAuth } from "../auth/AuthContext";
import { studySetApi, flashcardApi, importApi, ttsApi } from "../../lib/api";

const LANGUAGES = [
  { code: "en-US", name: "English (US)", flag: "🇺🇸" },
  { code: "vi-VN", name: "Tiếng Việt", flag: "🇻🇳" },
  { code: "ja-JP", name: "日本語", flag: "🇯🇵" },
  { code: "ko-KR", name: "한국어", flag: "🇰🇷" },
  { code: "zh-CN", name: "中文 (简体)", flag: "🇨🇳" },
  { code: "zh-TW", name: "中文 (繁體)", flag: "🇹🇼" },
  { code: "fr-FR", name: "Français", flag: "🇫🇷" },
  { code: "de-DE", name: "Deutsch", flag: "🇩🇪" },
  { code: "es-ES", name: "Español", flag: "🇪🇸" },
  { code: "th-TH", name: "ไทย", flag: "🇹🇭" },
  { code: "id-ID", name: "Bahasa Indonesia", flag: "🇮🇩" },
];

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

async function playTTS(token: string, text: string, lang: string) {
  try {
    const blob = await ttsApi.getAudio(token, text, lang);
    const url = URL.createObjectURL(blob);
    const audio = new Audio(url);
    audio.play();
    audio.onended = () => URL.revokeObjectURL(url);
  } catch {
    // Fallback: use browser TTS
    const u = new SpeechSynthesisUtterance(text);
    u.lang = lang;
    window.speechSynthesis.speak(u);
  }
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
  const [termLanguage, setTermLanguage] = useState(existingSet?.termLanguage ?? "en-US");
  const [definitionLanguage, setDefinitionLanguage] = useState(existingSet?.definitionLanguage ?? "en-US");
  const [visibility, setVisibility] = useState<string>(existingSet?.visibility ?? "public");
  const [showImport, setShowImport] = useState(false);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importLoading, setImportLoading] = useState(false);
  const [importResult, setImportResult] = useState<{ imported: number; errors: Array<{ row: number; field: string; reason: string }> } | null>(null);

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
            termLanguage,
            definitionLanguage,
            visibility,
          })
        : await studySetApi.create(token, {
            title: title.trim(),
            description: description.trim(),
            contentType: "flashcard",
            termLanguage,
            definitionLanguage,
            visibility,
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
        <div style={{ display: "flex", gap: 16 }}>
          <label style={{ flex: 1 }}>
            Ngôn ngữ thuật ngữ
            <select value={termLanguage} onChange={(e) => setTermLanguage(e.target.value)}>
              {LANGUAGES.map((l) => (
                <option key={l.code} value={l.code}>{l.flag} {l.name}</option>
              ))}
            </select>
          </label>
          <label style={{ flex: 1 }}>
            Ngôn ngữ định nghĩa
            <select value={definitionLanguage} onChange={(e) => setDefinitionLanguage(e.target.value)}>
              {LANGUAGES.map((l) => (
                <option key={l.code} value={l.code}>{l.flag} {l.name}</option>
              ))}
            </select>
          </label>
        </div>
        <label>
          Chế độ hiển thị
          <select value={visibility} onChange={(e) => setVisibility(e.target.value)}>
            <option value="public">Công khai</option>
            <option value="private">Riêng tư</option>
          </select>
        </label>
      </section>

      {error && <p className="message message--error">{error}</p>}

      <section style={{ padding: 16, border: "1px solid var(--border)", borderRadius: 8, marginBottom: 16 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <strong>📥 Nhập từ Excel (.xlsx)</strong>
            <button className="secondary-button" type="button" onClick={() => setShowImport(!showImport)}>
              {showImport ? "Đóng" : "Mở"}
            </button>
          </div>
          {showImport && (
            <div style={{ marginTop: 12 }}>
              <p style={{ fontSize: 14, color: "var(--muted-foreground)" }}>
                Chọn file .xlsx với cột: Term, Definition (bắt buộc), Example, Hint, Synonyms, Image URL (tùy chọn)
              </p>
              <input type="file" accept=".xlsx,.xls" onChange={(e) => setImportFile(e.target.files?.[0] ?? null)} />
              {importFile && (
                <button
                  className="primary-button"
                  style={{ marginTop: 8 }}
                  disabled={importLoading || !existingSet}
                  type="button"
                  onClick={async () => {
                    if (!importFile || !existingSet) return;
                    setImportLoading(true);
                    setImportResult(null);
                    try {
                      const result = await importApi.flashcards(token, existingSet.id, importFile);
                      setImportResult(result);
                      if (result.errors.length === 0) {
                        const updated = await studySetApi.get(token, existingSet.id);
                        onSave(updated);
                      }
                    } catch (err) {
                      setError(err instanceof Error ? err.message : "Import failed");
                    } finally {
                      setImportLoading(false);
                    }
                  }}
                >
                  {importLoading ? "Đang nhập..." : "Nhập dữ liệu"}
                </button>
              )}
              {!existingSet && (
                <p style={{ marginTop: 8, fontSize: 13, color: "var(--muted-foreground)" }}>
                  💡 Lưu bộ thẻ trước, sau đó quay lại để nhập dữ liệu từ Excel.
                </p>
              )}
              {importResult && (
                <div style={{ marginTop: 8 }}>
                  <p>Đã nhập: {importResult.imported} thẻ</p>
                  {importResult.errors.length > 0 && (
                    <div style={{ color: "var(--destructive)" }}>
                      <p>Lỗi:</p>
                      <ul>
                        {importResult.errors.map((e, i) => (
                          <li key={i}>Dòng {e.row}: [{e.field}] {e.reason}</li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </section>

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
            <div style={{ display: "flex", gap: 8, marginBottom: 8 }}>
              <button
                type="button"
                className="secondary-button"
                style={{ fontSize: 12 }}
                onClick={() => playTTS(token, card.term, termLanguage)}
              >
                🔊 Term
              </button>
              <button
                type="button"
                className="secondary-button"
                style={{ fontSize: 12 }}
                onClick={() => playTTS(token, card.definition, definitionLanguage)}
              >
                🔊 Definition
              </button>
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
