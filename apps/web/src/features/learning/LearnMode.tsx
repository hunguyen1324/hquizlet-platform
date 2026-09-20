// LearnMode — full parity: written + multiple-choice + true/false + settings + undo
import React from "react";
import { useAuth } from "../auth/AuthContext";
import { quizApi, type QuizGeneratedItem } from "../../lib/api/client";
import type { Flashcard } from "./types";
import type { CardResult } from "./progressContract";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import { useQuizGeneration } from "./useQuizGeneration";
import { LearnSettingsDialog } from "./LearnSettingsDialog";
import type { LearnSettings } from "./LearnSettingsDialog";
import { TrueFalseQuestion } from "./TrueFalseQuestion";
import "./learning.css";

type Props = { cards: Flashcard[]; studySetId: number };
type LearnItem = Pick<QuizGeneratedItem, "flashcardId" | "term" | "definition" | "choices" | "starred"> & {
  _tfDisplayDef?: string;
  _tfIsTrue?: boolean;
};
type QuestionType = "written" | "multipleChoice" | "trueFalse";
type AnswerState = { submitted: string; correct: boolean; attempts: number; responseTimeMs: number };

/** Tính loại câu hỏi theo số thẻ và settings */
function getQuestionType(
  index: number,
  total: number,
  hasChoices: boolean,
  allowedTypes: QuestionType[],
): QuestionType {
  const available = allowedTypes.filter((t) => t !== "multipleChoice" || hasChoices);
  if (available.length === 0) return "written";
  if (available.length === 1) return available[0];
  // Phân loại theo plan: <4 written only, 4–7 30/30/40, ≥8 25/25/50
  if (total < 4) return "written";
  const ratios = total < 8
    ? ["multipleChoice", "trueFalse", "written", "written", "trueFalse", "multipleChoice", "written"]
    : ["written", "multipleChoice", "trueFalse", "written", "written", "trueFalse", "multipleChoice", "written"];
  const cycle = ratios[index % ratios.length] as QuestionType;
  return available.includes(cycle) ? cycle : available[0];
}

export function LearnMode({ cards, studySetId }: Props) {
  const { token } = useAuth();
  const generation = useQuizGeneration(studySetId, "learn", Math.min(cards.length, 100));
  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({ studySetId, mode: "learn" });

  const [settings, setSettings] = React.useState<LearnSettings>({
    questionTypes: ["written", "multipleChoice", "trueFalse"],
    starredOnly: false,
    swapSides: false,
  });
  const [showSettings, setShowSettings] = React.useState(false);
  const [restartKey, setRestartKey] = React.useState(0);
  const [queue, setQueue] = React.useState<number[]>([]);
  const [prevStack, setPrevStack] = React.useState<{ queueSnapshot: number[]; answersSnapshot: Record<number, AnswerState> }[]>([]);
  const [answers, setAnswers] = React.useState<Record<number, AnswerState>>({});
  const [input, setInput] = React.useState("");
  const [feedback, setFeedback] = React.useState<boolean | null>(null);
  const [tfAnswer, setTfAnswer] = React.useState<boolean | null>(null);
  const [submitting, setSubmitting] = React.useState(false);
  const [done, setDone] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [startedAt, setStartedAt] = React.useState(() => new Date());
  const [questionStartedAt, setQuestionStartedAt] = React.useState(() => Date.now());
  const inputRef = React.useRef<HTMLInputElement | null>(null);

  const data = generation.state.state === "ready" ? generation.state.data : null;
  const rawItems = React.useMemo(
    () => (data?.items ?? []).filter((x) => x.term !== undefined) as LearnItem[],
    [data],
  );

  // Filter starred if needed, then add TF metadata
  const items = React.useMemo((): LearnItem[] => {
    const base = settings.starredOnly ? rawItems.filter((x) => x.starred) : rawItems;
    return base.map((item, idx) => {
      const correctAnswer = settings.swapSides ? (item.term ?? "") : (item.definition ?? "");
      const total = base.length;
      const hasChoices = (item.choices?.length ?? 0) >= 2;
      const qType = getQuestionType(idx, total, hasChoices, settings.questionTypes);
      let tfDef = correctAnswer;
      if (qType === "trueFalse") {
        const isFake = idx % 2 === 1;
        if (isFake) {
          const other = base.find((_, i2) => i2 !== idx);
          const otherDef = settings.swapSides ? (other?.term ?? "") : (other?.definition ?? "");
          if (otherDef) tfDef = otherDef;
        }
      }
      return { ...item, _tfDisplayDef: tfDef, _tfIsTrue: tfDef === correctAnswer };
    });
  }, [rawItems, settings]);

  React.useEffect(() => {
    if (items.length && !done) setQueue(items.map((_, i) => i));
  // restartKey: incrementing this triggers re-init even when items hasn't changed
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [items, restartKey]);

  const currentIndex = queue[0];
  const current = currentIndex === undefined ? undefined : items[currentIndex];
  const progressPct = items.length > 0 ? ((items.length - queue.length) / items.length) * 100 : 0;

  const questionType: QuestionType = React.useMemo(() => {
    if (!current) return "written";
    const idx = items.indexOf(current);
    const hasChoices = (current.choices?.length ?? 0) >= 2;
    return getQuestionType(idx, items.length, hasChoices, settings.questionTypes);
  }, [current, items, settings.questionTypes]);

  const promptText = settings.swapSides ? current?.definition : current?.term;
  const correctAnswer = settings.swapSides ? (current?.term ?? "") : (current?.definition ?? "");

  async function submitWritten() {
    if (!current || !input.trim() || submitting || feedback !== null || !data) return;
    setSubmitting(true);
    setError(null);
    const previous = answers[current.flashcardId];
    const attempts = (previous?.attempts ?? 0) + 1;
    try {
      const result = await quizApi.evaluate(token, studySetId, {
        mode: "learn",
        seed: data.seed,
        limit: items.length,
        answers: [{
          flashcardId: current.flashcardId,
          submitted: input,
          attempts,
          responseTimeMs: Date.now() - questionStartedAt,
        }],
      });
      const cardResult = result.cardResults[0];
      if (!cardResult) throw new Error("Backend không trả kết quả cho thẻ này");
      setAnswers((old) => ({
        ...old,
        [current.flashcardId]: { submitted: input, correct: cardResult.correct, attempts, responseTimeMs: cardResult.responseTimeMs ?? 0 },
      }));
      setFeedback(cardResult.correct);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Không thể chấm câu trả lời");
    } finally {
      setSubmitting(false);
    }
  }

  function chooseMcq(choice: string) {
    if (!current || feedback !== null || submitting) return;
    setInput(choice);
    void (async () => {
      setSubmitting(true);
      setError(null);
      const attempts = (answers[current.flashcardId]?.attempts ?? 0) + 1;
      try {
        const result = await quizApi.evaluate(token, studySetId, {
          mode: "learn",
          seed: data!.seed,
          limit: items.length,
          answers: [{ flashcardId: current.flashcardId, submitted: choice, attempts, responseTimeMs: Date.now() - questionStartedAt }],
        });
        const cardResult = result.cardResults[0];
        if (!cardResult) throw new Error("Backend không trả kết quả");
        setAnswers((old) => ({
          ...old,
          [current.flashcardId]: { submitted: choice, correct: cardResult.correct, attempts, responseTimeMs: cardResult.responseTimeMs ?? 0 },
        }));
        setFeedback(cardResult.correct);
      } catch (e: unknown) {
        setError(e instanceof Error ? e.message : "Không thể chấm câu trả lời");
      } finally {
        setSubmitting(false);
      }
    })();
  }

  function chooseTF(isTrue: boolean) {
    if (!current || feedback !== null || submitting) return;
    setTfAnswer(isTrue);
    const isCorrect = current._tfIsTrue === isTrue;
    setAnswers((old) => ({
      ...old,
      [current.flashcardId]: {
        submitted: isTrue ? "Đúng" : "Sai",
        correct: isCorrect,
        attempts: (old[current.flashcardId]?.attempts ?? 0) + 1,
        responseTimeMs: Date.now() - questionStartedAt,
      },
    }));
    setFeedback(isCorrect);
  }

  function next() {
    if (!current || feedback === null) return;
    // Save to undo stack
    setPrevStack((s) => [...s.slice(-4), { queueSnapshot: [...queue], answersSnapshot: { ...answers } }]);
    const nextQueue = feedback ? queue.slice(1) : [...queue.slice(1), currentIndex];
    setQueue(nextQueue);
    setInput("");
    setFeedback(null);
    setTfAnswer(null);
    setQuestionStartedAt(Date.now());
    if (nextQueue.length === 0) finish();
    else window.setTimeout(() => inputRef.current?.focus(), 0);
  }

  function undo() {
    if (!prevStack.length) return;
    const prev = prevStack[prevStack.length - 1];
    setPrevStack((s) => s.slice(0, -1));
    setQueue(prev.queueSnapshot);
    setAnswers(prev.answersSnapshot);
    setInput("");
    setFeedback(null);
    setTfAnswer(null);
    setQuestionStartedAt(Date.now());
  }

  function finish() {
    const cardResults: CardResult[] = items.map((item) => {
      const answer = answers[item.flashcardId];
      return {
        flashcardId: item.flashcardId,
        correct: answer?.correct ?? true,
        attempts: answer?.attempts ?? 1,
        responseTimeMs: answer?.responseTimeMs,
      };
    });
    setDone(true);
    onSessionComplete({
      score: cardResults.filter((x) => x.correct).length,
      total: cardResults.length,
      cardResults,
      startedAt,
    });
  }

  function restart(keepGeneration = false) {
    resetSave();
    setDone(false);
    setAnswers({});
    setPrevStack([]);
    setInput("");
    setFeedback(null);
    setTfAnswer(null);
    setError(null);
    setStartedAt(new Date());
    setQuestionStartedAt(Date.now());
<<<<<<< HEAD
    // Increment restartKey BEFORE clearing queue so useEffect fires with fresh items
    setRestartKey((k) => k + 1);
    generation.regenerate();
=======
    if (!keepGeneration) generation.regenerate();
>>>>>>> 46c209a9dbaf0ce39db298890229e6fe78482ab0
  }

  /* ── Guard states ── */
  if (cards.length < 2) {
    return <LearningEmptyState message="Cần ít nhất 2 thẻ để học." hint="Thêm thẻ trong phần 'Sửa thẻ'." />;
  }
  if (generation.state.state === "loading") {
    return <div className="learn-loading" role="status">Đang tạo bài học…</div>;
  }
  if (generation.state.state === "error") {
    return (
      <div className="learn-error" role="alert">
        Không thể tạo Learn: {generation.state.error.message}
        <button type="button" className="secondary-button" onClick={generation.regenerate}>Thử lại</button>
      </div>
    );
  }
  if (!items.length) {
    return <LearningEmptyState message="Backend không trả về câu hỏi hợp lệ." />;
  }

  /* ── Done screen ── */
  if (done) {
    const cardResults: CardResult[] = items.map((item) => ({
      flashcardId: item.flashcardId,
      correct: answers[item.flashcardId]?.correct ?? true,
      attempts: answers[item.flashcardId]?.attempts ?? 1,
      responseTimeMs: answers[item.flashcardId]?.responseTimeMs,
    }));
    const score = cardResults.filter((x) => x.correct).length;
    const pct = items.length ? Math.round((score / items.length) * 100) : 0;
    return (
      <div className="learn-done">
        <div className="test-result-header">
          <div className={`test-result-score-ring ${pct >= 80 ? "ring--green" : pct >= 50 ? "ring--yellow" : "ring--red"}`}>
            <span className="test-result-pct">{pct}%</span>
          </div>
          <div>
            <h2>Hoàn thành vòng học!</h2>
            <p className="learn-score">
              Đúng <strong>{score}</strong> / {items.length} câu
            </p>
          </div>
        </div>
        <ProgressSaveStatus status={saveStatus} onRetry={() => onSessionComplete({ score, total: items.length, cardResults, startedAt })} />
        <div className="test-result-actions">
          <button type="button" className="secondary-button" onClick={() => setShowSettings(true)}>
            Cài đặt lại
          </button>
          <button type="button" className="primary-button" onClick={restart}>Học lại</button>
        </div>
        {showSettings && (
          <LearnSettingsDialog
            settings={settings}
            onClose={() => setShowSettings(false)}
            onChange={(s) => { setSettings(s); setShowSettings(false); restart(true); }}
            hasStarred={cards.some((c) => c.starred)}
          />
        )}
      </div>
    );
  }

  if (!current) return <div className="learn-loading" role="status">Đang chuẩn bị câu hỏi…</div>;

  const useMcq = questionType === "multipleChoice" && (current.choices?.length ?? 0) >= 2;
  const useTF = questionType === "trueFalse";

  return (
    <div className="learn-mode learn-mode--quizlet">
      {showSettings && (
        <LearnSettingsDialog
          settings={settings}
          onClose={() => setShowSettings(false)}
          onChange={(s) => { setSettings(s); setShowSettings(false); restart(true); }}
          hasStarred={cards.some((c) => c.starred)}
        />
      )}

      {/* Progress row */}
      <div className="mode-progress-row">
        <div className="progress-bar-track" role="progressbar" aria-valuenow={Math.round(progressPct)} aria-valuemin={0} aria-valuemax={100}>
          <div className="progress-bar-fill" style={{ width: `${progressPct}%` }} />
        </div>
        <div className="learn-progress-actions">
          <span className="mode-progress-label">Thẻ {items.length - queue.length + 1} / {items.length}</span>
          {/* Undo button */}
          {prevStack.length > 0 && (
            <button
              type="button"
              className="ql-icon-btn learn-undo-btn"
              onClick={undo}
              title="Quay lại thẻ trước (Undo)"
              aria-label="Quay lại thẻ trước"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="9 14 4 9 9 4"/><path d="M20 20v-7a4 4 0 00-4-4H4"/>
              </svg>
            </button>
          )}
          {/* Settings button */}
          <button
            type="button"
            className="ql-icon-btn"
            onClick={() => setShowSettings(true)}
            title="Cài đặt"
            aria-label="Cài đặt học"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="3"/>
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
            </svg>
          </button>
        </div>
      </div>

      {/* Question card */}
      <div className="learn-session-card">
        <p className="learn-prompt-label">
          {useMcq && "Chọn đáp án đúng"}
          {useTF && "Đúng hay Sai?"}
          {!useMcq && !useTF && (settings.swapSides ? "Định nghĩa" : "Thuật ngữ")}
        </p>
        <p className="learn-prompt">{promptText}</p>
      </div>

      {/* Question input area */}
      {useMcq && (
        <div className="test-choices learn-mcq">
          {current.choices!.map((choice, i) => {
            const selected = input === choice;
            const showResult = feedback !== null && selected;
            return (
              <button
                key={`${choice}-${i}`}
                type="button"
                className={`test-choice${selected ? " selected" : ""}${showResult ? (feedback ? " choice-correct" : " choice-wrong") : ""}`}
                onClick={() => chooseMcq(choice)}
                disabled={feedback !== null || submitting}
              >
                <span className="choice-letter">{String.fromCharCode(65 + i)}</span>
                {choice}
              </button>
            );
          })}
        </div>
      )}

      {useTF && (
        <TrueFalseQuestion
          term={promptText ?? ""}
          displayDefinition={current._tfDisplayDef ?? correctAnswer}
          correctDefinition={correctAnswer}
          answer={tfAnswer}
          revealed={feedback !== null}
          onAnswer={chooseTF}
        />
      )}

      {!useMcq && !useTF && (
        <div className="learn-input-area">
          <label className="learn-input-label" htmlFor="learn-answer">Nhập định nghĩa</label>
          <input
            id="learn-answer"
            ref={inputRef}
            className={`learn-input${feedback === null ? "" : feedback ? " input-correct" : " input-wrong"}`}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") feedback === null ? void submitWritten() : next();
            }}
            disabled={submitting || feedback !== null}
            autoFocus
            autoComplete="off"
            placeholder="Gõ câu trả lời của bạn…"
          />
        </div>
      )}

      {/* Feedback for written/mcq (TF shows its own) */}
      {!useTF && feedback !== null && (
        <div className={`learn-feedback ${feedback ? "feedback-correct" : "feedback-wrong"}`}>
          {feedback ? "Chính xác!" : <>Chưa đúng. Đáp án: <strong>{correctAnswer}</strong></>}
        </div>
      )}
      {error && <div className="learn-error" role="alert">{error}</div>}

      {/* Actions */}
      <div className="learn-actions">
        {!useMcq && !useTF && feedback === null && (
          <button type="button" className="primary-button" onClick={() => void submitWritten()} disabled={!input.trim() || submitting}>
            {submitting ? "Đang chấm…" : "Kiểm tra"}
          </button>
        )}
        {feedback !== null && (
          <button type="button" className="primary-button" onClick={next}>Tiếp theo →</button>
        )}
        {useTF && feedback !== null && (
          <button type="button" className="primary-button" onClick={next}>Tiếp theo →</button>
        )}
      </div>
    </div>
  );
}
