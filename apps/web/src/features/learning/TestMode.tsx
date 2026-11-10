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
import { ViewerWatermark } from "../../components/learning/ViewerWatermark";
import "./learning.css";

type Props = { cards: Flashcard[]; studySetId: number; title?: string };

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

/** The same setup dialog is used before starting and when changing settings. */
function TestSettingsDialog({ settings, totalCards, title, onClose, onChange }: {
  settings: TestSettings; totalCards: number; title: string;
  onClose: () => void; onChange: (settings: TestSettings) => void;
}) {
  const maxCards = Math.min(totalCards, 20);
  const [local, setLocal] = React.useState<TestSettings>(settings);
  const panelRef = React.useRef<HTMLDivElement>(null);
  React.useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    const panel = panelRef.current;
    panel?.querySelector<HTMLElement>("button, input, select")?.focus();
    const keydown = (event: KeyboardEvent) => {
      if (event.key === "Escape") { event.preventDefault(); onClose(); }
      if (event.key === "Tab" && panel) {
        const elements = Array.from(panel.querySelectorAll<HTMLElement>("button:not(:disabled), input:not(:disabled), select:not(:disabled)"));
        const first = elements[0], last = elements[elements.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    };
    document.addEventListener("keydown", keydown);
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => { document.removeEventListener("keydown", keydown); document.body.style.overflow = overflow; previous?.focus(); };
  }, []);
  function toggleType(type: QuestionTypeOption) {
    setLocal(previous => previous.questionTypes.includes(type)
      ? previous.questionTypes.length > 1 ? { ...previous, questionTypes: previous.questionTypes.filter(item => item !== type) } : previous
      : { ...previous, questionTypes: [...previous.questionTypes, type] });
  }
  return <div className="ql-settings-overlay test-setup-overlay" onMouseDown={event => { if (event.target === event.currentTarget) onClose(); }}>
    <div className="test-setup-dialog" role="dialog" aria-modal="true" aria-labelledby="test-setup-title" ref={panelRef}>
      <button type="button" className="ql-settings-close test-setup-close" onClick={onClose} aria-label="Đóng thiết lập">×</button>
      <header className="test-setup-heading"><div><p>{title}</p><h2 id="test-setup-title">Thiết lập bài kiểm tra</h2></div><span className="study-mode-art" aria-hidden="true"><i /><i /></span></header>
      <label className="test-setup-row"><span>Câu hỏi <small>(tối đa {maxCards})</small></span>
        <input type="number" min={2} max={maxCards} value={local.limit} aria-label="Số câu hỏi"
          onChange={event => setLocal(previous => ({ ...previous, limit: Math.max(2, Math.min(maxCards, Number(event.target.value) || 2)) }))} />
      </label>
      <label className="test-setup-row"><span>Trả lời bằng</span><select value={local.askTerm ? "definition" : "term"}
        onChange={event => setLocal(previous => ({ ...previous, askTerm: event.target.value === "definition" }))}>
        <option value="definition">Định nghĩa</option><option value="term">Thuật ngữ</option>
      </select></label>
      <div className="test-setup-types">{(["trueFalse", "multipleChoice", "written"] as QuestionTypeOption[]).map(type => <label key={type} className="test-setup-row">
        <span>{TYPE_LABELS[type]}</span><input className="study-switch" type="checkbox" role="switch" checked={local.questionTypes.includes(type)} onChange={() => toggleType(type)} />
      </label>)}</div>
      <footer><button type="button" className="primary-button study-pill-button" onClick={() => onChange(local)}>Bắt đầu làm kiểm tra</button></footer>
    </div>
  </div>;
}

export function TestMode({ cards, studySetId, title = "Học phần" }: Props) {
  const { token } = useAuth();
  const generation = useQuizGeneration(studySetId, "test", Math.min(cards.length, 100));
  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({ studySetId, mode: "test" });

  const defaultSettings: TestSettings = {
    limit: Math.min(cards.length, 20),
    questionTypes: ["multipleChoice"],
    askTerm: true,
  };

  const [settings, setSettings] = React.useState<TestSettings>(defaultSettings);
  const [showSettings, setShowSettings] = React.useState(true);
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
    return (
      <div className="learn-loading" role="status">
        <ViewerWatermark />
        Đang tạo bài kiểm tra…
      </div>
    );
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

  if (!started) {
    return <div className="test-setup-landing">
      <span className="study-mode-art" aria-hidden="true"><i /><i /></span>
      <p>{title}</p><h2>Kiểm tra kiến thức của bạn</h2>
      <p>Chọn số câu hỏi và cách trả lời để bắt đầu.</p>
      <button type="button" className="primary-button study-pill-button" onClick={() => setShowSettings(true)}>Thiết lập bài kiểm tra</button>
      {showSettings && <TestSettingsDialog settings={settings} totalCards={cards.length} title={title}
        onClose={() => setShowSettings(false)}
        onChange={next => { setSettings(next); setStartedAt(new Date()); setStarted(true); setShowSettings(false); }} />}
    </div>;
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
        <ViewerWatermark />
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
      <ViewerWatermark />
      {showSettings && (
        <TestSettingsDialog
          settings={settings}
          title={title}
          totalCards={cards.length}
          onClose={() => setShowSettings(false)}
          onChange={(s) => {
            setShowSettings(false);
            setStartedAt(new Date());
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
                  <span>{settings.askTerm ? "Thuật ngữ" : "Định nghĩa"}</span>
                  <span className="test-question-position">{qIndex + 1}/{questions.length}</span>
                </div>
                <p className="test-term">{q.questionText}</p>

                <p className="test-answer-instruction">{q.questionType === "multipleChoice" ? "Chọn đáp án đúng" : TYPE_LABELS[q.questionType]}</p>
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
