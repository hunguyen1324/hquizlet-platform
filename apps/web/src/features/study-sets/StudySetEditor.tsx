// StudySetEditor — Dev 3 [P2-WEB-02]
// Tạo/sửa study set và flashcards qua studySetApi + transactional bulkSave.
// Phase 10: Thêm language selector, visibility toggle, import Excel.
// Phase UI: Quizlet-inspired redesign

import React, { useState } from "react";
import type { StudySet, DraftCard } from "../../types";
import { useAuth } from "../auth/AuthContext";
import { studySetApi, flashcardApi, importApi, ttsApi } from "../../lib/api";
import "./StudySetEditor.css";

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

function LangLabel({ code }: { code: string }) {
  const lang = LANGUAGES.find((l) => l.code === code);
  return <span>{lang ? `${lang.flag} ${lang.name}` : code}</span>;
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
  const [importResult, setImportResult] = useState<{
    imported: number;
    errors: Array<{ row: number; field: string; reason: string }>;
  } | null>(null);

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

    if (!title.trim()) { setError("Cần nhập tiêu đề bộ thẻ."); return; }
    if (cleanCards.length === 0) { setError("Cần ít nhất một thẻ."); return; }
    if (cleanCards.some((c) => !c.term || !c.definition)) {
      setError("Mỗi thẻ cần có đủ thuật ngữ và định nghĩa.");
      return;
    }

    setLoading(true);
    try {
      const saved = existingSet
        ? await studySetApi.update(token, existingSet.id, {
            title: title.trim(), description: description.trim(),
            termLanguage, definitionLanguage, visibility,
          })
        : await studySetApi.create(token, {
            title: title.trim(), description: description.trim(),
            contentType: "flashcard", termLanguage, definitionLanguage, visibility,
          });

      const keepItems = cleanCards.map((card, position) => ({
        ...(card.id ? { id: card.id } : {}),
        term: card.term, definition: card.definition,
        exampleSentence: card.exampleSentence, hintExplanation: card.hintExplanation,
        synonyms: card.synonyms, imageUrl: card.imageUrl, position,
      }));

      const keptIds = new Set(
        cleanCards.map((card) => card.id).filter((id): id is number => typeof id === "number")
      );
      const deleteItems = (existingSet?.flashcards ?? [])
        .filter((card) => !keptIds.has(card.id))
        .map((card) => ({ id: card.id, term: card.term, definition: card.definition, delete: true }));

      await flashcardApi.bulkSave(token, saved.id, [...keepItems, ...deleteItems]);
      const updated = await studySetApi.get(token, saved.id);
      onSave(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Không lưu được học phần.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <form className="qe-page" onSubmit={handleSubmit}>
      {/* ── Top bar ─────────────────────────────────────── */}
      <div className="qe-topbar">
        <span className="qe-topbar-title">
          {isEditing ? "Chỉnh sửa học phần" : "Tạo học phần mới"}
        </span>
        <div className="qe-topbar-actions">
          <button className="qe-btn qe-btn--ghost" type="button" onClick={onCancel}>
            Hủy
          </button>
          <button className="qe-btn qe-btn--primary" disabled={loading} type="submit">
            {loading ? "Đang lưu…" : isEditing ? "Lưu thay đổi" : "Tạo học phần"}
          </button>
        </div>
      </div>

      {/* ── Meta ─────────────────────────────────────────── */}
      <div className="qe-meta">
        {/* Title */}
        <div className="qe-meta-row">
          <div className="qe-field">
            <label className="qe-field-label" htmlFor="qe-title">Tiêu đề</label>
            <input
              id="qe-title"
              className="qe-title-input"
              autoFocus
              placeholder="Ví dụ: English Vocabulary Unit 1"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </div>
        </div>

        {/* Description */}
        <div className="qe-meta-row">
          <div className="qe-field">
            <label className="qe-field-label" htmlFor="qe-desc">Mô tả (tùy chọn)</label>
            <textarea
              id="qe-desc"
              placeholder="Mô tả ngắn về nội dung bộ thẻ…"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
        </div>

        {/* Languages */}
        <div className="qe-meta-row qe-meta-row--2col">
          <div className="qe-field">
            <label className="qe-field-label" htmlFor="qe-term-lang">Ngôn ngữ — Thuật ngữ</label>
            <select id="qe-term-lang" value={termLanguage} onChange={(e) => setTermLanguage(e.target.value)}>
              {LANGUAGES.map((l) => (
                <option key={l.code} value={l.code}>{l.flag} {l.name}</option>
              ))}
            </select>
          </div>
          <div className="qe-field">
            <label className="qe-field-label" htmlFor="qe-def-lang">Ngôn ngữ — Định nghĩa</label>
            <select id="qe-def-lang" value={definitionLanguage} onChange={(e) => setDefinitionLanguage(e.target.value)}>
              {LANGUAGES.map((l) => (
                <option key={l.code} value={l.code}>{l.flag} {l.name}</option>
              ))}
            </select>
          </div>
        </div>

        {/* Visibility */}
        <div className="qe-meta-row">
          <div className="qe-field">
            <span className="qe-field-label">Chế độ hiển thị</span>
            <div className="qe-vis-row">
              <button
                type="button"
                className={`qe-vis-btn${visibility === "public" ? " qe-vis-btn--active" : ""}`}
                onClick={() => setVisibility("public")}
              >
                🌐 Công khai
              </button>
              <button
                type="button"
                className={`qe-vis-btn${visibility === "private" ? " qe-vis-btn--active" : ""}`}
                onClick={() => setVisibility("private")}
              >
                🔒 Riêng tư
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* ── Import Excel accordion ───────────────────────── */}
      <div className="qe-import">
        <button
          type="button"
          className="qe-import-toggle"
          onClick={() => setShowImport(!showImport)}
          aria-expanded={showImport}
        >
          <span>📥 Nhập từ Excel (.xlsx)</span>
          <span className={`qe-import-toggle-icon${showImport ? " qe-import-toggle-icon--open" : ""}`}>▼</span>
        </button>
        {showImport && (
          <div className="qe-import-body">
            <p className="qe-import-hint">
              File .xlsx cần các cột: <strong>Term</strong>, <strong>Definition</strong> (bắt buộc);
              Example, Hint, Synonyms, Image URL (tùy chọn).
            </p>
            <input
              type="file"
              accept=".xlsx,.xls"
              onChange={(e) => setImportFile(e.target.files?.[0] ?? null)}
            />
            {importFile && (
              <button
                className="qe-btn qe-btn--primary"
                style={{ marginTop: 10 }}
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
                    setError(err instanceof Error ? err.message : "Import thất bại");
                  } finally {
                    setImportLoading(false);
                  }
                }}
              >
                {importLoading ? "Đang nhập…" : "Nhập dữ liệu"}
              </button>
            )}
            {!existingSet && (
              <p style={{ marginTop: 10, fontSize: "0.82rem", color: "#586380" }}>
                💡 Lưu bộ thẻ trước, sau đó quay lại để nhập từ Excel.
              </p>
            )}
            {importResult && (
              <div style={{ marginTop: 10 }}>
                <p style={{ color: "#22c55e", fontWeight: 700 }}>✓ Đã nhập {importResult.imported} thẻ</p>
                {importResult.errors.length > 0 && (
                  <div style={{ color: "#ef4444", fontSize: "0.85rem", marginTop: 6 }}>
                    {importResult.errors.map((e, i) => (
                      <p key={i} style={{ margin: "2px 0" }}>Dòng {e.row}: [{e.field}] {e.reason}</p>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {/* ── Error ────────────────────────────────────────── */}
      {error && <div className="qe-error">⚠ {error}</div>}

      {/* ── Cards section ────────────────────────────────── */}
      <div>
        <div className="qe-cards-header">
          <div>
            <span className="qe-cards-title">Thẻ học</span>
            <span className="qe-cards-count">{draftCards.length} thẻ</span>
          </div>
          <button className="qe-btn qe-btn--outline qe-btn--sm" type="button" onClick={addCard}>
            + Thêm thẻ
          </button>
        </div>

        {/* Column labels */}
        <div className="qe-lang-bar">
          <div className="qe-lang-bar-label">
            📝 Thuật ngữ · <LangLabel code={termLanguage} />
          </div>
          <div className="qe-lang-bar-label">
            💡 Định nghĩa · <LangLabel code={definitionLanguage} />
          </div>
        </div>

        {/* Card list */}
        {draftCards.map((card, index) => (
          <div className="qe-card" key={card.key}>
            {/* Card header */}
            <div className="qe-card-header">
              <span className="qe-card-num">{index + 1}</span>
              <div className="qe-card-header-right">
                <button
                  type="button"
                  className="qe-card-tts"
                  title="Phát âm thuật ngữ"
                  onClick={() => playTTS(token, card.term, termLanguage)}
                >
                  🔊 Term
                </button>
                <button
                  type="button"
                  className="qe-card-tts"
                  title="Phát âm định nghĩa"
                  onClick={() => playTTS(token, card.definition, definitionLanguage)}
                >
                  🔊 Def
                </button>
                <button
                  className="qe-btn qe-btn--danger"
                  type="button"
                  onClick={() => removeCard(card.key)}
                  aria-label="Xóa thẻ"
                >
                  ✕
                </button>
              </div>
            </div>

            {/* Main term / definition */}
            <div className="qe-card-body">
              <div className="qe-card-col">
                <span className="qe-card-col-label">Thuật ngữ</span>
                <input
                  className="qe-card-input"
                  placeholder="Nhập thuật ngữ…"
                  value={card.term}
                  onChange={(e) => updateCard(card.key, "term", e.target.value)}
                />
              </div>
              <div className="qe-card-col">
                <span className="qe-card-col-label">Định nghĩa</span>
                <input
                  className="qe-card-input"
                  placeholder="Nhập định nghĩa…"
                  value={card.definition}
                  onChange={(e) => updateCard(card.key, "definition", e.target.value)}
                />
              </div>
            </div>

            {/* Extra fields */}
            <div className="qe-card-extras">
              <div className="qe-card-extra-col">
                <span className="qe-card-extra-label">Ví dụ</span>
                <textarea
                  className="qe-card-extra-input"
                  rows={2}
                  placeholder="I ____ to school every day."
                  value={card.exampleSentence ?? ""}
                  onChange={(e) => updateCard(card.key, "exampleSentence", e.target.value)}
                />
              </div>
              <div className="qe-card-extra-col">
                <span className="qe-card-extra-label">Gợi ý / Giải thích</span>
                <textarea
                  className="qe-card-extra-input"
                  rows={2}
                  placeholder="Giải thích ngắn…"
                  value={card.hintExplanation ?? ""}
                  onChange={(e) => updateCard(card.key, "hintExplanation", e.target.value)}
                />
              </div>
              <div className="qe-card-extra-col">
                <span className="qe-card-extra-label">Từ đồng nghĩa</span>
                <textarea
                  className="qe-card-extra-input"
                  rows={2}
                  placeholder="happy → joyful, cheerful…"
                  value={card.synonyms ?? ""}
                  onChange={(e) => updateCard(card.key, "synonyms", e.target.value)}
                />
              </div>
            </div>
          </div>
        ))}

        {/* Add card button */}
        <button className="qe-add-card" type="button" onClick={addCard}>
          <span style={{ fontSize: "1.2rem", lineHeight: 1 }}>+</span>
          Thêm một thẻ nữa
        </button>
      </div>
    </form>
  );
}
