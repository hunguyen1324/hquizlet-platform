// TestMode — Quizlet-style: settings dialog (gear icon) + question list + result
import React from "react";
import { useAuth } from "../auth/AuthContext";
import { quizApi, type QuizAnswer, type QuizGeneratedItem } from "../../lib/api/client";
import type { Flashcard } from "./types";
import type { CardResult } from "./progressContract";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import { useQuizGeneration } from "./useQuizGeneration";
import type { TestSettings, QuestionTypeOption } from "./TestSettingsForm";
import { MultipleChoiceQuestion } from "./MultipleChoiceQuestion";
import { TrueFalseQuestion } from "./TrueFalseQuestion";
import "./learning.css";

type Props = { cards: Flashcard[]; studySetId: number };

type TestItem = {
  flashcardId: number;
  questionText: string;
  correctAnswer: string;
  choices: string[];
  questionType: QuestionTypeOption;
  imageUrl?: string | null;
  _tfDisplayDef?: string;
  _tfIsTrue?: boolean;
};

type AnswerState = {
  answer: string;
  selectedIndex?: number;
  tfAnswer?: boolean;
  attempts: number;
  responseTimeMs: number;
  _tfCorrect?: boolean;
};

const TYPE_LABELS: Record<QuestionTypeOption, string> = {
  multipleChoice: "Trắc nghiệm",
  written: "Tự luận",
  trueFalse: "Đúng / Sai",
};

function assignQuestionType(
  index: number,
  allowedTypes: QuestionTypeOption[],
  hasChoices: boolean,
): QuestionTypeOption {
  if (allowedTypes.length === 1) return allowedTypes[0];
  const available = allowedTypes.filter((t) => t !== "multipleChoice" || hasChoices);
  if (available.length === 0) return "written";
  return available[index % available.length];
}

/** Settings Dialog — popup gear icon style */
function TestSettingsDialog({
  settings,
  totalCards,
  onClose,
  onChange,
}: {
  settings: TestSettings;
  totalCards: number;
  onClose: () => void;
  onChange: (s: TestSettings) => void;
}) {
  const maxCards = Math.min(totalCards, 20);
  const [local, setLocal] = React.useState<TestSettings>(settings);

  function toggleType(t: QuestionTypeOption) {
    setLocal((prev) => {
      if (prev.questionTypes.includes(t)) {
        if (prev.questionTypes.length === 1) return prev;
        return { ...prev, questionTypes: prev.questionTypes.filter((x) => x !== t) };
      }
      return { ...prev, questionTypes: [...prev.questionTypes, t] };
    });
  }

  return (
    <div className="ql-settings-overlay" role="dialog" aria-modal="true" aria-label="Cài đặt kiểm tra">
      <div className="ql-settings-panel ql-settings-panel--test">
        <div className="ql-settings-header">
          <h3>Cài đặt bài kiểm tra</h3>
          <button type="button" className="ql-settings-close" onClick={onClose} aria-label="Đóng">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
              <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>

        {/* Số câu */}
        <div className="ql-settings-section">
          <p className="ql-settings-label">Số câu hỏi</p>
          <div className="ql-test-slider-wrap">
            <input
              type="range"
              min={2}
              max={maxCards}
              value={local.limit}
              onChange={(e) => setLocal((p) => ({ ...p, limit: Number(e.target.value) }))}
              className="ql-test-slider"
            />
            <span className="ql-test-slider-val">{local.limit}</span>
          </div>
          <p className="ql-settings-hint">{totalCards} thẻ trong bộ này · tối đa 20</p>
        </div>

        {/* Loại câu hỏi */}
        <div className="ql-settings-section">
          <p className="ql-settings-label">Loại câu hỏi</p>
          {(["multipleChoice", "written", "trueFalse"] as QuestionTypeOption[]).map((t) => (
            <label key={t} className="ql-settings-check">
              <input
                type="checkbox"
                checked={local.questionTypes.includes(t)}
                onChange={() => toggleType(t)}
              />
              <span>{TYPE_LABELS[t]}</span>
            </label>
          ))}
        </div>

        {/* Hỏi theo term hay definition */}
        <div className="ql-settings-section">
          <p className="ql-settings-label">Hỏi theo</p>
          <label className="ql-settings-check">
            <input
              type="radio"
              name="test-ask-side"
              checked={local.askTerm}
              onChange={() => setLocal((p) => ({ ...p, askTerm: true }))}
            />
            <span>Thuật ngữ → điền định nghĩa</span>
          </label>
          <label className="ql-settings-check">
            <input
              type="radio"
              name="test-ask-side"
              checked={!local.askTerm}
              onChange={() => setLocal((p) => ({ ...p, askTerm: false }))}
            />
            <span>Định nghĩa → điền thuật ngữ</span>
          </label>
        </div>

        <div className="ql-settings-footer">
          <button type="button" className="secondary-button" onClick={onClose}>Hủy</button>
          <button type="button" className="primary-button" onClick={() => { onChange(local); onClose(); }}>
            Áp dụng & Bắt đầu lại
          </button>
        </div>
      </div>
    </div>
  );
}

export function TestMode({ cards, studySetId }: Props) {
  const { token } = useAuth();
  const generation = useQuizGeneration(studySetId, "test", Math.min(cards.length, 100));
  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({ studySetId, mode: "test" });

  const defaultSettings: TestSettings = {
    limit: Math.min(cards.length, 20),
    questionTypes: ["multipleChoice", "written", "trueFalse"],
    askTerm: true,
  };

  const [settings, setSettings] = React.useState<TestSettings>(defaultSettings);
  const [showSettings, setShowSettings] = React.useState(false);
  const [questions, setQuestions] = React.useState<TestItem[]>([]);
  const [answers, setAnswers] = React.useState<Record<number, AnswerState>>({});
  const [writtenInputs, setWrittenInputs] = React.useState<Record<number, string>>({});
  const [writtenFeedback, setWrittenFeedback] = React.useState<Record<number, boolean | null>>({});
  const [writtenSubmitting, setWrittenSubmitting] = React.useState<Record<number, boolean>>({});
  const [submitted, setSubmitted] = React.useState(false);
  const [result, setResult] = React.useState<{ score: number; total: number; cardResults: CardResult[] } | null>(null);
  const [startedAt, setStartedAt] = React.useState(() => new Date());
  const [questionStartedAt] = React.useState(() => Date.now());
  const [error, setError] = React.useState<string | null>(null);
  const [started, setStarted] = React.useState(false);

  const data = generation.state.state === "ready" ? generation.state.data : null;
  const rawItems = React.useMemo(() => data?.items ?? [], [data]);

  /* ── Build questions khi settings thay đổi ── */
  React.useEffect(() => {
    if (!started || !rawItems.length) return;
    const mcItems = rawItems.filter((i) => i.kind === "question" || i.choices);
    const chosen = mcItems.slice(0, settings.limit);

    const built: TestItem[] = chosen.map((item, idx) => {
      const term = item.term ?? item.text ?? "";
      const definition = item.definition ?? "";
      const questionText = settings.askTerm ? term : definition;
      const correctAnswer = settings.askTerm ? definition : term;
      const hasChoices = (item.choices?.length ?? 0) >= 2;
      const qType = assignQuestionType(idx, settings.questionTypes, hasChoices);
      let tfDef = correctAnswer;
      if (qType === "trueFalse") {
        const isFake = idx % 2 === 1;
        if (isFake) {
          const otherItem = mcItems.find((x, i2) => i2 !== idx);
          const otherAnswer = settings.askTerm ? (otherItem?.definition ?? "") : (otherItem?.term ?? "");
          if (otherAnswer) tfDef = otherAnswer;
        }
      }
      return {
        flashcardId: item.flashcardId,
        questionText,
        correctAnswer,
        choices: hasChoices ? (item.choices ?? []) : [],
        questionType: qType,
        imageUrl: null,
        _tfDisplayDef: tfDef,
        _tfIsTrue: tfDef === correctAnswer,
      };
    });
    setQuestions(built);
    setAnswers({});
    setWrittenInputs({});
    setWrittenFeedback({});
    setWrittenSubmitting({});
    setSubmitted(false);
    setResult(null);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [settings, rawItems, started]);

  /* ── Guard states ── */
  if (cards.length < 2) {
    return <LearningEmptyState message="Cần ít nhất 2 thẻ để làm bài kiểm tra." hint="Thêm thẻ trong phần 'Sửa thẻ'." />;
  }
  if (generation.state.state === "loading") {
    return <div className="learn-loading" role="status">Đang tạo bài kiểm tra…</div>;
  }
  if (generation.state.state === "error") {
    return (
      <div className="learn-error" role="alert">
        Không thể tạo Test: {generation.state.error.message}
        <button type="button" className="secondary-button" onClick={generation.regenerate}>Thử lại</button>
      </div>
    );
  }
  if (!rawItems.length) {
    return <LearningEmptyState message="Backend không trả về câu hỏi hợp lệ." hint="Thử lại để tạo một bài kiểm tra mới." />;
  }

  /* ── Pre-start screen ── */
  if (!started) {
    const maxCards = Math.min(cards.length, 20);
    return (
      <div className="test-prestart-screen">
        <div className="test-prestart-icon" aria-hidden="true">
          <svg width="52" height="52" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/>
          </svg>
        </div>
        <h2 className="test-prestart-title">Kiểm tra</h2>
        <p className="test-prestart-desc">
          Tự đánh giá kiến thức với {Math.min(cards.length, settings.limit)} câu hỏi gồm trắc nghiệm, tự luận và đúng/sai.
        </p>

        {/* Preview settings */}
        <div className="test-prestart-settings">
          <div className="test-prestart-setting-row">
            <span className="test-prestart-setting-label">Số câu</span>
            <span className="test-prestart-setting-val">{Math.min(cards.length, settings.limit)} / {cards.length}</span>
          </div>
          <div className="test-prestart-setting-row">
            <span className="test-prestart-setting-label">Loại câu</span>
            <span className="test-prestart-setting-val">
              {settings.questionTypes.map((t) => TYPE_LABELS[t]).join(", ")}
            </span>
          </div>
          <div className="test-prestart-setting-row">
            <span className="test-prestart-setting-label">Hỏi theo</span>
            <span className="test-prestart-setting-val">{settings.askTerm ? "Thuật ngữ" : "Định nghĩa"}</span>
          </div>
        </div>

        <div className="test-prestart-actions">
          <button
            type="button"
            className="secondary-button test-prestart-settings-btn"
            onClick={() => setShowSettings(true)}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="3"/>
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
            </svg>
            Tùy chỉnh
          </button>
          <button
            type="button"
            className="primary-button test-prestart-start-btn"
            onClick={() => { setStarted(true); setStartedAt(new Date()); }}
          >
            Bắt đầu kiểm tra
          </button>
        </div>

        {showSettings && (
          <TestSettingsDialog
            settings={settings}
            totalCards={cards.length}
            onClose={() => setShowSettings(false)}
            onChange={(s) => { setSettings(s); }}
          />
        )}
      </div>
    );
  }

  /* ── Answer handlers ── */
  function chooseMC(flashcardId: number, choice: string, selectedIndex: number) {
    if (answers[flashcardId] || submitted) return;
    setAnswers((prev) => ({
      ...prev,
      [flashcardId]: { answer: choice, selectedIndex, attempts: 1, responseTimeMs: Date.now() - questionStartedAt },
    }));
  }

  function chooseTF(q: TestItem, isTrue: boolean) {
    if (answers[q.flashcardId] || submitted) return;
    const correct = q._tfIsTrue === isTrue;
    setAnswers((prev) => ({
      ...prev,
      [q.flashcardId]: {
        answer: isTrue ? "Đúng" : "Sai",
        tfAnswer: isTrue,
        attempts: 1,
        responseTimeMs: Date.now() - questionStartedAt,
        _tfCorrect: correct,
      },
    }));
  }

  async function submitWritten(q: TestItem) {
    const input = writtenInputs[q.flashcardId] ?? "";
    if (!input.trim() || writtenSubmitting[q.flashcardId] || writtenFeedback[q.flashcardId] !== undefined) return;
    if (!data) return;
    setWrittenSubmitting((p) => ({ ...p, [q.flashcardId]: true }));
    try {
      const res = await quizApi.evaluate(token, studySetId, {
        mode: "test",
        seed: data.seed,
        limit: questions.length,
        answers: [{ flashcardId: q.flashcardId, submitted: input, attempts: 1, responseTimeMs: Date.now() - questionStartedAt }],
      });
      const cr = res.cardResults[0];
      const correct = cr?.correct ?? false;
      setWrittenFeedback((p) => ({ ...p, [q.flashcardId]: correct }));
      setAnswers((prev) => ({
        ...prev,
        [q.flashcardId]: { answer: input, attempts: 1, responseTimeMs: cr?.responseTimeMs ?? 0 },
      }));
    } catch {
      setWrittenFeedback((p) => ({ ...p, [q.flashcardId]: false }));
      setAnswers((prev) => ({
        ...prev,
        [q.flashcardId]: { answer: input, attempts: 1, responseTimeMs: 0 },
      }));
    } finally {
      setWrittenSubmitting((p) => ({ ...p, [q.flashcardId]: false }));
    }
  }

  const seed = data!.seed;
  const answeredCount = questions.filter((q) => answers[q.flashcardId]).length;
  const allAnswered = answeredCount >= questions.length;

  async function submitTest() {
    if (submitted || !allAnswered) return;
    setSubmitted(true);
    setError(null);
    const payload: QuizAnswer[] = questions.map((q) => {
      const ans = answers[q.flashcardId];
      return {
        flashcardId: q.flashcardId,
        submitted: ans?.answer,
        selectedIndex: ans?.selectedIndex,
        attempts: ans?.attempts ?? 1,
        responseTimeMs: ans?.responseTimeMs ?? 0,
      };
    });
    try {
      const evaluated = await quizApi.evaluate(token, studySetId, {
        mode: "test", seed, limit: questions.length, answers: payload,
      });
      const cardResults = evaluated.cardResults as CardResult[];
      const merged = cardResults.map((cr) => {
        const q = questions.find((x) => x.flashcardId === cr.flashcardId);
        const ans = answers[cr.flashcardId] as AnswerState;
        if (q?.questionType === "trueFalse" && ans?._tfCorrect !== undefined) {
          return { ...cr, correct: ans._tfCorrect };
        }
        if (q?.questionType === "written" && writtenFeedback[cr.flashcardId] !== undefined) {
          return { ...cr, correct: writtenFeedback[cr.flashcardId] ?? false };
        }
        return cr;
      });
      const score = merged.filter((x) => x.correct).length;
      setResult({ score, total: merged.length, cardResults: merged });
      onSessionComplete({ score, total: merged.length, cardResults: merged, startedAt });
    } catch (e: unknown) {
      setSubmitted(false);
      setError(e instanceof Error ? e.message : "Không thể chấm bài");
    }
  }

  function restart() {
    resetSave();
    setSettings(defaultSettings);
    setQuestions([]);
    setAnswers({});
    setWrittenInputs({});
    setWrittenFeedback({});
    setWrittenSubmitting({});
    setSubmitted(false);
    setResult(null);
    setStartedAt(new Date());
    setError(null);
    setStarted(false);
    generation.regenerate();
  }

  /* ── Result screen ── */
  if (result) {
    const pct = result.total ? Math.round((result.score / result.total) * 100) : 0;
    const ringClass = pct >= 80 ? "ring--green" : pct >= 50 ? "ring--yellow" : "ring--red";
    return (
      <div className="learn-done">
        <div className="test-result-hero">
          <div className={`test-result-score-ring ${ringClass}`}>
            <svg viewBox="0 0 100 100" className="score-ring-svg" style={{ "--pct": pct } as React.CSSProperties}>
              <circle className="score-ring-bg" cx="50" cy="50" r="45"></circle>
              <circle className="score-ring-progress" cx="50" cy="50" r="45"></circle>
            </svg>
            <span className="test-result-pct">{pct}%</span>
          </div>
          <div>
            <h2 className="test-result-heading">Kết quả bài kiểm tra</h2>
            <p className="test-result-sub">
              Đúng <strong>{result.score}</strong> / {result.total} câu
            </p>
          </div>
        </div>

        {/* Breakdown bars */}
        <div className="test-result-breakdown">
          <div className="test-breakdown-item test-breakdown-item--correct">
            <span className="test-breakdown-count">{result.score}</span>
            <span className="test-breakdown-label">Đúng</span>
          </div>
          <div className="test-breakdown-sep" />
          <div className="test-breakdown-item test-breakdown-item--wrong">
            <span className="test-breakdown-count">{result.total - result.score}</span>
            <span className="test-breakdown-label">Sai</span>
          </div>
        </div>

        {error && <p className="learn-error">{error}</p>}
        <ProgressSaveStatus status={saveStatus} onRetry={() => onSessionComplete({ score: result.score, total: result.total, cardResults: result.cardResults, startedAt })} />

        <div className="learn-review">
          <h3 className="test-review-title">Chi tiết từng câu</h3>
          {questions.map((q, i) => {
            const r = result.cardResults.find((x) => x.flashcardId === q.flashcardId);
            const ans = answers[q.flashcardId];
            const isCorrect = r?.correct ?? false;
            return (
              <div key={q.flashcardId} className={`review-row ${isCorrect ? "correct" : "wrong"}`}>
                <span className="review-num">Câu {i + 1}</span>
                <div className="review-content">
                  <span className="review-type-chip review-type-chip--{q.questionType}">{TYPE_LABELS[q.questionType]}</span>
                  <span className="review-term">{q.questionText}</span>
                  <span className="review-answer">
                    Bạn: <em>{ans?.answer || "—"}</em>
                  </span>
                  {!isCorrect && (
                    <span className="review-correct-answer">
                      Đúng: <strong>{q.correctAnswer}</strong>
                    </span>
                  )}
                </div>
                <span className={`review-badge ${isCorrect ? "badge--correct" : "badge--wrong"}`}>
                  {isCorrect ? "✓" : "✗"}
                </span>
              </div>
            );
          })}
        </div>
        <div className="test-result-actions">
          <button type="button" className="secondary-button" onClick={() => { setStarted(false); setResult(null); setSubmitted(false); }}>
            Cài đặt lại
          </button>
          <button type="button" className="primary-button" onClick={restart}>
            Làm bài mới
          </button>
        </div>
      </div>
    );
  }

  /* ── Question list (in-progress) ── */
  return (
    <>
      {showSettings && (
        <TestSettingsDialog
          settings={settings}
          totalCards={cards.length}
          onClose={() => setShowSettings(false)}
          onChange={(s) => {
            setSettings(s);
            setAnswers({});
            setWrittenInputs({});
            setWrittenFeedback({});
            setWrittenSubmitting({});
            setSubmitted(false);
            setResult(null);
          }}
        />
      )}

      <form
        className="test-mode test-mode--form"
        onSubmit={(e) => { e.preventDefault(); void submitTest(); }}
      >
        {/* Toolbar */}
        <div className="test-mode-toolbar">
          <div className="mode-progress-row" style={{ flex: 1 }}>
            <div className="progress-bar-track" role="progressbar" aria-valuenow={answeredCount} aria-valuemax={questions.length}>
              <div className="progress-bar-fill" style={{ width: `${(answeredCount / questions.length) * 100}%` }} />
            </div>
            <span className="mode-progress-label">{answeredCount} / {questions.length}</span>
          </div>
          <button
            type="button"
            className="ql-icon-btn"
            onClick={() => setShowSettings(true)}
            title="Cài đặt bài kiểm tra"
            aria-label="Cài đặt"
          >
            <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="3"/>
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
            </svg>
          </button>
        </div>

        <div className="test-question-list">
          {questions.map((q, qIndex) => {
            const ans = answers[q.flashcardId] as AnswerState | undefined;
            const writtenFb = writtenFeedback[q.flashcardId];

            return (
              <section key={q.flashcardId} className="test-question-block">
                <div className="test-question-header">
                  <span className="test-question-num">{qIndex + 1}</span>
                  <span className="test-question-type-badge test-type-{q.questionType}">
                    {TYPE_LABELS[q.questionType]}
                  </span>
                  {ans && (
                    <span className={`test-question-answered-badge ${
                      q.questionType === "written"
                        ? writtenFb === true ? "badge--correct" : writtenFb === false ? "badge--wrong" : "badge--pending"
                        : q.questionType === "trueFalse"
                          ? ans._tfCorrect ? "badge--correct" : "badge--wrong"
                          : ans.answer === q.correctAnswer ? "badge--correct" : "badge--pending"
                    }`}>
                      {q.questionType === "written"
                        ? writtenFb === true ? "✓" : writtenFb === false ? "✗" : "✎"
                        : "✓"}
                    </span>
                  )}
                </div>
                <p className="test-term">{q.questionText}</p>

                {q.questionType === "multipleChoice" && (
                  <MultipleChoiceQuestion
                    choices={q.choices}
                    selected={ans?.answer ?? null}
                    revealed={Boolean(ans)}
                    correctAnswer={q.correctAnswer}
                    onChoose={(choice, idx) => chooseMC(q.flashcardId, choice, idx)}
                  />
                )}

                {q.questionType === "trueFalse" && (
                  <TrueFalseQuestion
                    term={q.questionText}
                    displayDefinition={q._tfDisplayDef ?? q.correctAnswer}
                    correctDefinition={q.correctAnswer}
                    answer={ans?.tfAnswer !== undefined ? ans.tfAnswer : null}
                    revealed={Boolean(ans)}
                    onAnswer={(isTrue) => chooseTF(q, isTrue)}
                  />
                )}

                {q.questionType === "written" && (
                  <div className="ql-written-wrap">
                    <label className="ql-written-label" htmlFor={`written-${q.flashcardId}`}>
                      Nhập {settings.askTerm ? "định nghĩa" : "thuật ngữ"}
                    </label>
                    <input
                      id={`written-${q.flashcardId}`}
                      type="text"
                      className={[
                        "ql-written-input",
                        writtenFb === true ? "ql-written-input--correct" : "",
                        writtenFb === false ? "ql-written-input--wrong" : "",
                      ].filter(Boolean).join(" ")}
                      value={writtenInputs[q.flashcardId] ?? ""}
                      onChange={(e) => setWrittenInputs((p) => ({ ...p, [q.flashcardId]: e.target.value }))}
                      onKeyDown={(e) => { if (e.key === "Enter") void submitWritten(q); }}
                      disabled={writtenFb !== undefined || submitted}
                      autoComplete="off"
                      placeholder="Gõ câu trả lời…"
                    />
                    {writtenFb !== undefined && (
                      <div className={`ql-written-feedback ${writtenFb ? "ql-written-feedback--correct" : "ql-written-feedback--wrong"}`}>
                        {writtenFb ? "✓ Chính xác!" : <>✗ Chưa đúng. Đáp án: <strong>{q.correctAnswer}</strong></>}
                      </div>
                    )}
                    {writtenFb === undefined && (
                      <button
                        type="button"
                        className="secondary-button"
                        style={{ marginTop: "0.5rem" }}
                        onClick={() => void submitWritten(q)}
                        disabled={!(writtenInputs[q.flashcardId]?.trim()) || writtenSubmitting[q.flashcardId]}
                      >
                        {writtenSubmitting[q.flashcardId] ? "Đang chấm…" : "Kiểm tra"}
                      </button>
                    )}
                  </div>
                )}
              </section>
            );
          })}
        </div>

        {error && <div className="learn-error" role="alert">{error}</div>}

        <div className="test-submit-row">
          <button
            type="button"
            className="secondary-button"
            onClick={() => { setStarted(false); setResult(null); setSubmitted(false); }}
          >
            ← Cài đặt lại
          </button>
          <button type="submit" className="primary-button" disabled={!allAnswered || submitted}>
            {submitted ? "Đang chấm…" : `Nộp bài (${answeredCount}/${questions.length})`}
          </button>
        </div>
      </form>
    </>
  );
}
