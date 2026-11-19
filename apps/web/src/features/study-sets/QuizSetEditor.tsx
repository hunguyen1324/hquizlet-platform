import { readVisibilityDefault } from "../settings/visibilityDefaults";
import { normalizeSubtitleText } from "../../lib/subtitles";
// QuizSetEditor — tạo/sửa quiz set (multiple choice, true/false, written)
// Tham chiếu: hquizlet/apps/nextjs/src/components/study-set/quiz-set-form.tsx

import React, { useEffect, useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { quizApi, importApi, quizQuestionApi, studySetApi } from "../../lib/api";
import { RichTextContent } from "./exam/RichTextContent";
import { StudyActionIcon } from "./StudyActionIcon";
import "./QuizSetEditor.css";

const ListeningMediaEditor = React.lazy(() => import("./ListeningMediaEditor").then(module => ({ default: module.ListeningMediaEditor })));

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
];

type QuestionType = "multiple_choice" | "true_false" | "written" | "paragraph" | "sorting";

const QUESTION_LABELS = ["Từ vựng", "Ngữ pháp", "Đọc", "Nghe"];

interface QuizOption {
  text: string;
  position: number;
}

interface QuizQuestion {
  key: string;
  questionText: string;
  questionType: QuestionType;
  correctAnswer: string;
  answerExplanation: string;
  paragraphText: string;
  imageUrl: string;
  audioUrl: string;
  srtText: string;
  options: QuizOption[];
  position: number;
  subQuestions?: unknown;
  tags?: string[];
  timeInSeconds?: number;
}

type Props = {
  existingSetId?: number;
  questionId?: number;
  onSave: () => void;
  onCancel: () => void;
};

function newQuestion(position: number): QuizQuestion {
  return {
    key: crypto.randomUUID(),
    questionText: "",
    questionType: "multiple_choice",
    correctAnswer: "",
    answerExplanation: "",
    paragraphText: "",
    imageUrl: "",
    audioUrl: "",
    srtText: "",
    options: [
      { text: "", position: 0 },
      { text: "", position: 1 },
      { text: "", position: 2 },
      { text: "", position: 3 },
    ],
    position,
  };
}

function QuestionCard({
  question,
  index,
  onChange,
  onRemove,
  onDuplicate,
  disabled,
  studySetId,
  ensureStudySet,
  onBusyChange,
  focused = false,
}: {
  studySetId?: number;
  focused?: boolean;
  ensureStudySet: () => Promise<number>;
  onBusyChange: (key: string, busy: boolean) => void;
  question: QuizQuestion;
  index: number;
  onChange: (q: QuizQuestion) => void;
  onRemove?: () => void;
  onDuplicate?: () => void;
  disabled: boolean;
}) {
  const latestQuestion = React.useRef(question);
  latestQuestion.current = question;
  const mediaBusy = React.useCallback((busy: boolean) => onBusyChange(question.key, busy), [onBusyChange, question.key]);
  const [editing, setEditing] = React.useState(focused || !question.questionText.trim());
  const [editorMounted, setEditorMounted] = React.useState(editing);
  const [explanationOpen, setExplanationOpen] = React.useState(false);
  const [mediaMounted, setMediaMounted] = React.useState(false);
  const [customLabel, setCustomLabel] = React.useState("");
  const tags = question.tags ?? [];
  function addCustomLabel() {
    const label = customLabel.trim().replace(/\s+/g, " ");
    if (!label) return;
    if (!tags.some(tag => tag.toLocaleLowerCase() === label.toLocaleLowerCase())) {
      setField("tags", [...tags, label]);
    }
    setCustomLabel("");
  }
  const typeLabel = { multiple_choice: "Nhiều lựa chọn", true_false: "Đúng / Sai", written: "Tự luận", paragraph: "Đọc hiểu", sorting: "Sắp xếp" }[question.questionType];
  const explicitAnswerIndex = question.options.findIndex(option => option.text === question.correctAnswer);
  const correctOption = (_text: string, index: number) => Boolean(question.correctAnswer) &&
    (explicitAnswerIndex >= 0 ? index === explicitAnswerIndex : question.correctAnswer.toUpperCase() === String.fromCharCode(65 + index));
  function setField<K extends keyof QuizQuestion>(key: K, value: QuizQuestion[K]) {
    onChange({ ...question, [key]: value });
  }

  function setOptionText(optIdx: number, text: string) {
    const newOptions = question.options.map((o, i) =>
      i === optIdx ? { ...o, text } : o
    );
    // Nếu đang chọn đáp án đúng là option này, cập nhật theo
    const newQ = { ...question, options: newOptions };
    if (question.correctAnswer === question.options[optIdx]?.text) {
      newQ.correctAnswer = text;
    }
    onChange(newQ);
  }

  function addOption() {
    onChange({
      ...question,
      options: [...question.options, { text: "", position: question.options.length }],
    });
  }

  function removeOption(optIdx: number) {
    const removed = question.options[optIdx];
    const newOptions = question.options
      .filter((_, i) => i !== optIdx)
      .map((o, i) => ({ ...o, position: i }));
    const newQ = { ...question, options: newOptions };
    const correctIndex = question.options.findIndex((option, index) => correctOption(option.text, index));
    if (correctIndex === optIdx || removed?.text === question.correctAnswer) newQ.correctAnswer = "";
    else if (correctIndex >= 0) newQ.correctAnswer = question.options[correctIndex].text;
    onChange(newQ);
  }

  return (
    <article id={`question-${question.key}`} className={`quiz-question-card draft-card${editing ? " qedit-card--editing" : ""}`}>
      <header className="qedit-card-head">
        <div className="qedit-question-meta"><strong>{String(index + 1).padStart(2, "0")}</strong><span>{typeLabel}</span>{question.timeInSeconds ? <span>· {question.timeInSeconds} giây</span> : null}</div>
        <div className="qedit-question-actions">
          {!focused && <><button type="button" className="qedit-icon-button" onClick={onDuplicate} disabled={disabled} title="Nhân bản câu hỏi" aria-label={`Nhân bản câu ${index + 1}`}><StudyActionIcon name="copy" /></button>
          <button type="button" className="qedit-icon-button qedit-danger" onClick={onRemove} disabled={disabled} title="Xóa câu hỏi" aria-label={`Xóa câu ${index + 1}`}><StudyActionIcon name="trash" /></button></>}
          <button type="button" className="qedit-edit-button" disabled={disabled} onClick={() => { setEditorMounted(true); setEditing(value => !value); }} aria-expanded={editing}><StudyActionIcon name={editing ? "check" : "edit"} />{editing ? (focused ? "Xem trước" : "Xong") : "Chỉnh sửa"}</button>
        </div>
      </header>
      {tags.length > 0 && <div className="qedit-labels" aria-label={`Nhãn câu ${index + 1}`}>{tags.map(tag => <span className="qedit-label" key={tag}>{tag}</span>)}</div>}
      {!editing && <div className="qedit-question-preview">
        {question.paragraphText && <RichTextContent className="qedit-passage" value={question.paragraphText} />}
        <RichTextContent className="qedit-prompt" value={question.questionText} />
        {question.imageUrl && <img className="qedit-question-image" src={question.imageUrl} alt={`Hình minh họa câu ${index + 1}`} loading="lazy" />}
        {question.questionType === "multiple_choice" && <div className="qedit-preview-options">{question.options.map((option, optionIndex) => <div key={optionIndex} className={`qedit-preview-option${correctOption(option.text, optionIndex) ? " qedit-preview-option--correct" : ""}`}><span className="qedit-option-indicator">{correctOption(option.text, optionIndex) ? "✓" : ""}</span><RichTextContent value={option.text} /></div>)}</div>}
        {question.questionType === "true_false" && <div className="qedit-preview-options">{["true", "false"].map(value => <div key={value} className={`qedit-preview-option${question.correctAnswer === value ? " qedit-preview-option--correct" : ""}`}><span className="qedit-option-indicator">{question.correctAnswer === value ? "✓" : ""}</span>{value === "true" ? "Đúng" : "Sai"}</div>)}</div>}
        {question.questionType !== "multiple_choice" && question.questionType !== "true_false" && question.correctAnswer && <div className="qedit-answer"><span>Đáp án</span><RichTextContent value={question.correctAnswer} /></div>}
        {question.answerExplanation && <div className="qedit-explanation">
          <button type="button" className="qedit-explanation-toggle" aria-expanded={explanationOpen} aria-controls={`explanation-${question.key}`} onClick={() => setExplanationOpen(value => !value)}>{explanationOpen ? "Ẩn giải thích" : "Xem giải thích câu trả lời"}</button>
          {explanationOpen && <div id={`explanation-${question.key}`}><RichTextContent className="qedit-explanation-content" value={question.answerExplanation} /></div>}
        </div>}
        {question.audioUrl && <span className="qedit-media-badge">Có âm thanh</span>}
      </div>}
      {editorMounted && <div className="quiz-question-body" hidden={!editing}>
        <div className="qedit-content-column">
        {/* Question text */}
        <label>
          <span>Nội dung câu hỏi <span className="required">*</span></span>
          <textarea
            rows={2}
            placeholder="Nhập câu hỏi..."
            value={question.questionText}
            onChange={(e) => setField("questionText", e.target.value)}
            disabled={disabled}
          />
        </label>

        {/* Question type */}
        <div className="quiz-type-row">
          <label>
            Loại câu hỏi
            <select
              value={question.questionType}
              onChange={(e) => {
                const t = e.target.value as QuestionType;
                const updated: QuizQuestion = { ...question, questionType: t, correctAnswer: "" };
                if (t === "multiple_choice" && updated.options.length < 2) {
                  updated.options = [
                    { text: "", position: 0 },
                    { text: "", position: 1 },
                  ];
                }
                onChange(updated);
              }}
              disabled={disabled}
            >
              <option value="multiple_choice">Trắc nghiệm</option>
              <option value="true_false">Đúng / Sai</option>
              <option value="written">Tự luận</option>
              <option value="paragraph">Đoạn văn</option>
              <option value="sorting">Sắp xếp</option>
            </select>
          </label>
        </div>

        {focused && <label>Thời gian (giây)
          <input type="number" min={1} value={question.timeInSeconds ?? ""} placeholder="Mặc định"
            onChange={event => setField("timeInSeconds", event.target.value ? Number(event.target.value) : undefined)} disabled={disabled} />
        </label>}
        <fieldset className="qedit-label-editor" disabled={disabled}>
          <legend>Nhãn câu hỏi</legend>
          <div className="qedit-labels">{QUESTION_LABELS.map(label => <button
            type="button" key={label} className="qedit-label-choice" aria-pressed={tags.includes(label)}
            onClick={() => setField("tags", tags.includes(label) ? tags.filter(tag => tag !== label) : [...tags, label])}
          >{label}</button>)}</div>
          <label htmlFor={`custom-label-${question.key}`}>Nhãn tùy chỉnh</label>
          <div className="qedit-custom-label">
            <input id={`custom-label-${question.key}`} value={customLabel} placeholder="Nhập nhãn rồi nhấn Enter…"
              onChange={event => setCustomLabel(event.target.value)}
              onKeyDown={event => { if (event.key === "Enter" && !event.nativeEvent.isComposing) { event.preventDefault(); addCustomLabel(); } }} />
            <button type="button" className="secondary-button" disabled={disabled || !customLabel.trim()} onClick={addCustomLabel}>Thêm</button>
          </div>
          {tags.length > 0 && <div className="qedit-labels">{tags.map(tag => <button type="button" key={tag} className="qedit-label-choice" aria-label={`Xóa nhãn ${tag}`} onClick={() => setField("tags", tags.filter(value => value !== tag))}>{tag} ×</button>)}</div>}
        </fieldset>
        </div>
        <div className="qedit-answer-column">
        {/* Options for multiple choice */}
        {question.questionType === "multiple_choice" && (
          <div className="quiz-options">
            <p className="quiz-options-label">Đáp án (chọn đáp án đúng)</p>
            <div className="qedit-option-grid">
            {question.options.map((opt, optIdx) => (
              <div key={optIdx} className="quiz-option-row">
                <input
                  type="radio"
                  name={`correct-${question.key}`}
                  aria-label={`Chọn đáp án ${optIdx + 1} là đáp án đúng`}
                  checked={correctOption(opt.text, optIdx) && opt.text !== ""}
                  onChange={() => setField("correctAnswer", opt.text)}
                  disabled={disabled || !opt.text}
                />
                <input
                  type="text"
                  placeholder={`Đáp án ${optIdx + 1}`}
                  aria-label={`Nội dung đáp án ${optIdx + 1}`}
                  value={opt.text}
                  onChange={(e) => setOptionText(optIdx, e.target.value)}
                  disabled={disabled}
                  className="quiz-option-input"
                />
                <button
                  type="button"
                  className="icon-button"
                  onClick={() => removeOption(optIdx)}
                  disabled={disabled || question.options.length <= 2}
                  aria-label="Xóa đáp án"
                >
                  ×
                </button>
              </div>
            ))}
            </div>
            <button type="button" className="secondary-button" onClick={addOption} disabled={disabled}>
              + Thêm đáp án
            </button>
          </div>
        )}

        {/* True/False */}
        {question.questionType === "true_false" && (
          <label>
            Đáp án đúng
            <select
              value={question.correctAnswer}
              onChange={(e) => setField("correctAnswer", e.target.value)}
              disabled={disabled}
            >
              <option value="">-- Chọn đáp án --</option>
              <option value="true">Đúng</option>
              <option value="false">Sai</option>
            </select>
          </label>
        )}

        {/* Written */}
        {question.questionType === "written" && (
          <label>
            Đáp án đúng <span className="required">*</span>
            <input
              type="text"
              placeholder="Nhập đáp án..."
              value={question.correctAnswer}
              onChange={(e) => setField("correctAnswer", e.target.value)}
              disabled={disabled}
            />
          </label>
        )}

        {/* Paragraph */}
        {question.questionType === "paragraph" && (
          <>
            <label>
              Đoạn văn
              <textarea
                rows={4}
                placeholder="Nhập đoạn văn..."
                value={question.paragraphText ?? ""}
                onChange={(e) => setField("paragraphText", e.target.value)}
                disabled={disabled}
              />
            </label>
            <label>
              Đáp án đúng <span className="required">*</span>
              <input
                type="text"
                placeholder="Nhập đáp án cho câu hỏi con..."
                value={question.correctAnswer}
                onChange={(e) => setField("correctAnswer", e.target.value)}
                disabled={disabled}
              />
            </label>
          </>
        )}

        {/* Sorting */}
        {question.questionType === "sorting" && (
          <>
            <p className="quiz-options-label">Các phần tử (nhập theo thứ tự đúng)</p>
            {question.options.map((opt, optIdx) => (
              <div key={optIdx} className="quiz-option-row">
                <span style={{ width: 24, textAlign: "center", color: "var(--muted-foreground)" }}>{optIdx + 1}.</span>
                <input
                  type="text"
                  placeholder={`Phần tử ${optIdx + 1}`}
                  value={opt.text}
                  onChange={(e) => setOptionText(optIdx, e.target.value)}
                  disabled={disabled}
                  className="quiz-option-input"
                />
                <button
                  type="button"
                  className="icon-button"
                  onClick={() => removeOption(optIdx)}
                  disabled={disabled || question.options.length <= 2}
                  aria-label="Xóa phần tử"
                >
                  ×
                </button>
              </div>
            ))}
            <button type="button" className="secondary-button" onClick={addOption} disabled={disabled}>
              + Thêm phần tử
            </button>
            <label style={{ marginTop: 8 }}>
              Thứ tự đúng (cách nhau bằng dấu phẩy)
              <input
                type="text"
                placeholder="VD: B,A,D,C"
                value={question.correctAnswer}
                onChange={(e) => setField("correctAnswer", e.target.value)}
                disabled={disabled}
              />
            </label>
          </>
        )}

        {/* Explanation */}
        <div className="qedit-explanation">
        <button type="button" className="qedit-explanation-toggle" aria-expanded={explanationOpen} aria-controls={`edit-explanation-${question.key}`} onClick={() => setExplanationOpen(value => !value)}>{explanationOpen ? "Ẩn giải thích" : "Chỉnh sửa giải thích (tùy chọn)"}</button>
        {explanationOpen && <label id={`edit-explanation-${question.key}`}>
          Giải thích (tùy chọn)
          <textarea
            rows={2}
            placeholder="Giải thích tại sao đây là đáp án đúng..."
            value={question.answerExplanation}
            onChange={(e) => setField("answerExplanation", e.target.value)}
            disabled={disabled}
          />
        </label>}
        </div>
        </div>
        <details className="qedit-media-settings" onToggle={event => { if (event.currentTarget.open) setMediaMounted(true); }}>
          <summary>Ảnh, âm thanh & phụ đề{question.imageUrl || question.audioUrl || question.srtText ? " · Đã thêm nội dung" : " · Tùy chọn"}</summary>
          <div className="qedit-media-content">
        {mediaMounted && <React.Suspense fallback={<p role="status">Đang tải công cụ âm thanh…</p>}><ListeningMediaEditor
          audioUrl={question.audioUrl} srtText={question.srtText} studySetId={studySetId}
          ensureStudySet={ensureStudySet} onBusyChange={mediaBusy} disabled={disabled}
          onChange={patch => onChange({ ...latestQuestion.current, ...patch })}
        /></React.Suspense>}
        <label>Link / đường dẫn ảnh<input value={question.imageUrl} onChange={(e) => setField("imageUrl", e.target.value)} placeholder="https://... hoặc /images/question.png" disabled={disabled} /></label>

          </div>
        </details>
      </div>}
    </article>
  );
}

export function QuizSetEditor({ existingSetId, questionId, onSave, onCancel }: Props) {
  const { token, user } = useAuth();
  const [draftSetId, setDraftSetId] = useState(existingSetId);
  const draftRequest = React.useRef<Promise<number> | null>(null);
  const [busyQuestions, setBusyQuestions] = useState<Record<string, boolean>>({});
  const mediaBusy = Object.values(busyQuestions).some(Boolean);
  const changeMediaBusy = React.useCallback((key: string, busy: boolean) => setBusyQuestions(previous => previous[key] === busy ? previous : { ...previous, [key]: busy }), []);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [questions, setQuestions] = useState<QuizQuestion[]>([newQuestion(0)]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [termLanguage, setTermLanguage] = useState("en-US");
  const [definitionLanguage, setDefinitionLanguage] = useState("en-US");
  const [visibility, setVisibility] = useState<string>(() => readVisibilityDefault(user?.id, "study"));
  const [showImport, setShowImport] = useState(false);
  const [questionSearch, setQuestionSearch] = useState("");
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importLoading, setImportLoading] = useState(false);
  const [importResult, setImportResult] = useState<{ imported: number; errors: Array<{ row: number; field: string; reason: string }> } | null>(null);

  useEffect(() => { setDraftSetId(existingSetId); draftRequest.current = null; }, [existingSetId]);
  async function ensureStudySet() {
    if (draftSetId) return draftSetId;
    if (draftRequest.current) return draftRequest.current;
    draftRequest.current = studySetApi.create(token, { title: title.trim() || "Bài nghe chưa đặt tên", description, contentType: "quiz", termLanguage, definitionLanguage, visibility: "private" }).then(set => { setDraftSetId(set.id); return set.id; });
    try { return await draftRequest.current; } finally { draftRequest.current = null; }
  }
  async function cancelEditor() {
    if (mediaBusy || loading) return;
    if (draftSetId && !existingSetId) {
      setLoading(true);
      try { await studySetApi.delete(token, draftSetId); } catch (value) { setError(value instanceof Error ? value.message : "Không xóa được bản nháp."); setLoading(false); return; }
    }
    onCancel();
  }
  useEffect(() => {
    if (!existingSetId) return;
    let cancelled = false;
    setLoading(true);
    Promise.all([studySetApi.get(token, existingSetId), quizQuestionApi.list(token, existingSetId)])
      .then(([set, loadedQuestions]) => {
        if (cancelled) return;
        setTitle(set.title); setDescription(set.description ?? ""); setTermLanguage(set.termLanguage); setDefinitionLanguage(set.definitionLanguage); setVisibility(set.visibility);
        setQuestions(loadedQuestions.map((q, index) => ({
          key: String(q.id ?? crypto.randomUUID()), questionText: q.questionText, questionType: q.questionType,
          correctAnswer: q.options?.find(option => option.isCorrect)?.text || q.correctAnswer || "", answerExplanation: q.answerExplanation ?? "", paragraphText: q.paragraphText ?? "",
          imageUrl: q.imageUrl ?? "", audioUrl: q.audioUrl ?? "", srtText: q.srtText ?? "", options: (q.options ?? []).map((o, optionIndex) => ({ text: o.text, position: optionIndex })),
          position: q.position ?? index, subQuestions: q.subQuestions, tags: q.tags, timeInSeconds: q.timeInSeconds,
        })));
      })
      .catch((value) => setError(value instanceof Error ? value.message : "Không tải được quiz để sửa."))
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [existingSetId, token]);

  function updateQuestion(key: string, updated: QuizQuestion) {
    setQuestions((prev) => prev.map((q) => (q.key === key ? updated : q)));
  }

  function removeQuestion(key: string) {
    setQuestions((prev) => {
      if (prev.length === 1) return prev;
      return prev.filter((q) => q.key !== key).map((q, i) => ({ ...q, position: i }));
    });
  }

  function addQuestion() {
    setQuestions((prev) => [...prev, newQuestion(prev.length)]);
  }

  async function importQuizIntoSet(studySetId: number, file: File) {
    const result = await importApi.quiz(token, studySetId, file);
    const safeResult = { imported: result.imported, errors: result.errors ?? [] };
    setImportResult(safeResult);
    return safeResult;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (mediaBusy) { setError("Chờ tải dữ liệu file nghe xong trước khi lưu."); return; }
    try { questions.forEach(q => normalizeSubtitleText(q.srtText)); }
    catch (value) { setError(value instanceof Error ? value.message : "Kiểm tra định dạng SRT."); return; }

    if (!title.trim()) {
      setError("Vui lòng nhập tiêu đề quiz.");
      return;
    }
    if (!importFile && questions.some((q) => !q.questionText.trim())) {
      setError("Tất cả câu hỏi cần có nội dung.");
      return;
    }
    if (!importFile && questions.some((q) => q.questionType !== "paragraph" && !q.correctAnswer)) {
      setError("Tất cả câu hỏi cần có đáp án đúng.");
      return;
    }

    setLoading(true);
    try {
      const payload = {
        title: title.trim(),
        description: description.trim(),
        contentType: "quiz" as const,
        termLanguage,
        definitionLanguage,
        visibility,
        questions: importFile
          ? []
          : questions.map((q, i) => ({
              questionText: q.questionText.trim(),
              questionType: q.questionType,
              imageUrl: q.imageUrl.trim() || undefined,
              audioUrl: q.audioUrl || undefined,
              srtText: normalizeSubtitleText(q.srtText),
              subQuestions: q.subQuestions, tags: q.tags, timeInSeconds: q.timeInSeconds,
              correctAnswer: q.correctAnswer,
              answerExplanation: q.answerExplanation.trim() || undefined,
              paragraphText: q.paragraphText?.trim() || undefined,
              position: i,
              options:
                q.questionType === "multiple_choice" || q.questionType === "sorting"
                  ? q.options
                      .filter((o) => o.text.trim())
                      .map((o, idx) => ({ text: o.text.trim(), position: idx }))
                  : [],
            })),
      };
      if (draftSetId) {
        await studySetApi.update(token, draftSetId, { title: payload.title, description: payload.description, termLanguage, definitionLanguage, visibility });
        if (!importFile) await quizQuestionApi.bulkSave(token, draftSetId, questions.map((q, i) => ({
          questionText: q.questionText.trim(), questionType: q.questionType, correctAnswer: q.correctAnswer || undefined,
          answerExplanation: q.answerExplanation.trim() || undefined, paragraphText: q.paragraphText?.trim() || undefined,
          imageUrl: q.imageUrl.trim() || undefined, audioUrl: q.audioUrl || undefined, srtText: normalizeSubtitleText(q.srtText), timeInSeconds: q.timeInSeconds, subQuestions: q.subQuestions, tags: q.tags, position: i,
          options: q.options.filter((o) => o.text.trim()).map((o, idx) => ({ text: o.text.trim(), position: idx, isCorrect: q.correctAnswer.toUpperCase() === String.fromCharCode(65 + idx) || q.correctAnswer === o.text })),
        })));
        if (importFile) { const result = await importQuizIntoSet(draftSetId, importFile); if (result.errors.length > 0) return; }
        onSave(); return;
      }
      const saved = await quizApi.create(token, payload);
      if (importFile) {
        const result = await importQuizIntoSet(saved.id, importFile);
        if (result.errors.length > 0) return;
      }
      onSave();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Không thể tạo quiz. Vui lòng thử lại.");
    } finally {
      setLoading(false);
    }
  }

  if (existingSetId && questionId) {
    const index = questions.findIndex(q => q.key === String(questionId));
    const question = questions[index];
    async function saveQuestion(event: React.FormEvent) {
      event.preventDefault();
      if (!question || !existingSetId || !questionId || loading || mediaBusy) return;
      setError("");
      setLoading(true);
      try {
        const { questionType, correctAnswer, timeInSeconds, subQuestions, tags } = question;
        const questionText = question.questionText.trim();
        const correctIndex = question.options.findIndex(option => option.text === correctAnswer);
        const letterIndex = /^[A-Z]$/i.test(correctAnswer) ? correctAnswer.toUpperCase().charCodeAt(0) - 65 : -1;
        const effectiveIndex = correctIndex >= 0 ? correctIndex : letterIndex;
        const options = question.options.flatMap((option, position) => option.text.trim()
          ? [{ text: option.text.trim(), position, isCorrect: questionType === "multiple_choice" && position === effectiveIndex }]
          : []);
        if (!questionText) throw new Error("Nhập nội dung câu hỏi.");
        if (questionType !== "paragraph" && !correctAnswer.trim()) throw new Error("Chọn hoặc nhập đáp án đúng.");
        if (questionType === "multiple_choice" && (options.length < 2 || !options.some(option => option.isCorrect))) {
          throw new Error("Nhập ít nhất hai lựa chọn và chọn đáp án đúng.");
        }
        await quizQuestionApi.update(token, existingSetId, questionId, {
          questionText, questionType, timeInSeconds, subQuestions, tags,
          correctAnswer: questionType === "multiple_choice" ? options.find(option => option.isCorrect)?.text : correctAnswer,
          options: options.map((option, position) => ({ ...option, position })),
          imageUrl: question.imageUrl.trim() || undefined, audioUrl: question.audioUrl || undefined,
          srtText: normalizeSubtitleText(question.srtText), answerExplanation: question.answerExplanation.trim() || undefined,
          paragraphText: question.paragraphText.trim() || undefined, position: index,
        });
        onSave();
      } catch (reason) {
        setError(reason instanceof Error ? reason.message : "Không lưu được câu hỏi.");
      } finally {
        setLoading(false);
      }
    }
    return <form className="create-page qedit-page qedit-page--focused" onSubmit={saveQuestion}>
      <header className="qedit-header">
        <button type="button" className="qedit-back" onClick={onCancel} disabled={loading || mediaBusy} aria-label="Về chi tiết quiz"><StudyActionIcon name="back" /></button>
        <div className="qedit-title"><strong>{index >= 0 ? `Chỉnh sửa câu ${index + 1}` : "Chỉnh sửa câu hỏi"}</strong><span>{title}</span></div>
        <div className="qedit-header-actions">
          <button type="button" className="ghost-button" disabled={loading || mediaBusy} onClick={onCancel}>Hủy</button>
          <button type="submit" className="primary-button" disabled={loading || mediaBusy || !question}>Lưu câu hỏi</button>
        </div>
      </header>
      {error && <p className="message message--error" role="alert">{error}</p>}
      {!question && !error && <p role="status">{loading ? "Đang tải câu hỏi…" : "Không tìm thấy câu hỏi."}</p>}
      {question && <QuestionCard key={question.key} focused question={question} index={index}
        studySetId={existingSetId} ensureStudySet={ensureStudySet} onBusyChange={changeMediaBusy}
        onChange={updated => updateQuestion(question.key, updated)}
        disabled={loading || mediaBusy} />}
    </form>;
  }

  return (
    <form className="create-page qedit-page" onSubmit={handleSubmit}>
      <header className="qedit-header">
        <button type="button" className="qedit-back" onClick={() => void cancelEditor()} disabled={loading || mediaBusy} aria-label="Trở về học phần"><StudyActionIcon name="back" /></button>
        <label className="qedit-title">Tiêu đề<input placeholder="Nhập tiêu đề quiz" value={title} onChange={event => setTitle(event.target.value)} disabled={loading} /></label>
        <div className="qedit-header-actions"><button className="ghost-button" type="button" onClick={() => void cancelEditor()} disabled={loading || mediaBusy}>Hủy</button><button className="primary-button" type="submit" disabled={loading || mediaBusy}>{loading ? "Đang lưu…" : existingSetId ? "Lưu thay đổi" : "Tạo Quiz"}</button></div>
      </header>
      <details className="qedit-settings"><summary>Cài đặt quiz</summary><section className="create-meta">
        <label>
          Mô tả (tùy chọn)
          <textarea
            placeholder="Mô tả ngắn về bộ quiz..."
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={loading}
            rows={2}
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
            Ngôn ngữ đáp án
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

      </details>
      {error && <p className="message message--error" role="alert">{error}</p>}

      <section className="qedit-import">
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <strong>📥 Nhập quiz từ Excel (.xlsx)</strong>
            <button className="secondary-button" type="button" onClick={() => setShowImport(!showImport)}>
              {showImport ? "Đóng" : "Mở"}
            </button>
          </div>
          {showImport && (
            <div style={{ marginTop: 12 }}>
              <div style={{ display: "flex", gap: 12, justifyContent: "space-between", alignItems: "center" }}>
                <p style={{ fontSize: 14, color: "var(--muted-foreground)", margin: 0 }}>
                  Chọn file .xlsx với cột: Question, Type, Option A-D, Correct Answer, Time (s), Audio URL, Image URL, Answer Explanation
                </p>
                <a className="secondary-button" href={importApi.templateUrl("quiz_template.xlsx")} download style={{ textDecoration: "none", whiteSpace: "nowrap" }}>
                  Tải file mẫu
                </a>
              </div>
              <input type="file" accept=".xlsx,.xls" onChange={(e) => setImportFile(e.target.files?.[0] ?? null)} />
              {importFile && (
                <button
                  className="primary-button"
                  style={{ marginTop: 8 }}
                  disabled={importLoading || loading}
                  type={existingSetId ? "button" : "submit"}
                  onClick={async () => {
                    if (!importFile || !existingSetId) return;
                    setImportLoading(true);
                    setImportResult(null);
                    try {
                      const result = await importQuizIntoSet(existingSetId, importFile);
                      if (result.errors.length === 0) onSave();
                    } catch (err) {
                      setError(err instanceof Error ? err.message : "Import failed");
                    } finally {
                      setImportLoading(false);
                    }
                  }}
                >
                  {importLoading || loading ? "Đang nhập..." : existingSetId ? "Nhập dữ liệu" : "Tạo từ Excel"}
                </button>
              )}
              {!existingSetId && (
                <p style={{ marginTop: 8, fontSize: 13, color: "var(--muted-foreground)" }}>
                  Chọn file rồi bấm <strong>Tạo từ Excel</strong> để tạo quiz và nhập câu hỏi cùng lúc.
                </p>
              )}
              {importResult && (
                <div style={{ marginTop: 8 }}>
                  <p>Đã nhập: {importResult.imported} câu hỏi</p>
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

      <div className="qedit-workspace">
      <section className="cards-editor">
        <div className="cards-editor-heading">
          <div>
            <h2>{questions.length} câu hỏi <span>· {questions.filter(question => question.correctAnswer).length} có đáp án</span></h2>
          </div>
          <button className="secondary-button" type="button" onClick={addQuestion} disabled={loading}>
            + Thêm câu hỏi
          </button>
        </div>

        {questions.map((q, idx) => (
          <QuestionCard
            studySetId={draftSetId}
            ensureStudySet={ensureStudySet}
            onBusyChange={changeMediaBusy}
            key={q.key}
            question={q}
            index={idx}
            onChange={(updated) => updateQuestion(q.key, updated)}
            onRemove={() => removeQuestion(q.key)}
            onDuplicate={() => setQuestions(previous => {
              const next = [...previous];
              next.splice(idx + 1, 0, { ...structuredClone(q), key: crypto.randomUUID() });
              return next.map((question, position) => ({ ...question, position }));
            })}
            disabled={loading || mediaBusy}
          />
        ))}

        <button className="add-row" type="button" onClick={addQuestion} disabled={loading}>
          + Thêm một câu hỏi nữa
        </button>
      </section>
      <aside className="qedit-question-finder"><h2>Tìm câu hỏi trong quiz</h2><label className="qedit-search"><StudyActionIcon name="search" /><input type="search" aria-label="Tìm câu hỏi" placeholder="Nhập nội dung câu hỏi…" value={questionSearch} onChange={event => setQuestionSearch(event.target.value)} /></label>
        <p>{questions.filter(question => question.questionText.toLocaleLowerCase().includes(questionSearch.trim().toLocaleLowerCase())).length} câu hỏi</p>
        <nav aria-label="Đi đến câu hỏi">{questions.map((question, index) => question.questionText.toLocaleLowerCase().includes(questionSearch.trim().toLocaleLowerCase()) && <button type="button" key={question.key} onClick={() => document.getElementById(`question-${question.key}`)?.scrollIntoView({ behavior: "smooth", block: "start" })}><span>{index + 1}</span><RichTextContent value={question.questionText || "Câu hỏi mới"} /></button>)}</nav>
      </aside>
      </div>
    </form>
  );
}
