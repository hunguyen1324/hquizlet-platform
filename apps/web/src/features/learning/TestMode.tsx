// TestMode — full parity: settings → mixed question types → result breakdown
import React from "react";
import { useAuth } from "../auth/AuthContext";
import { quizApi, type QuizAnswer, type QuizGeneratedItem } from "../../lib/api/client";
import type { Flashcard } from "./types";
import type { CardResult } from "./progressContract";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import { useQuizGeneration } from "./useQuizGeneration";
import { TestSettingsForm } from "./TestSettingsForm";
import type { TestSettings, QuestionTypeOption } from "./TestSettingsForm";
import { MultipleChoiceQuestion } from "./MultipleChoiceQuestion";
import { TrueFalseQuestion } from "./TrueFalseQuestion";
import "./learning.css";

type Props = { cards: Flashcard[]; studySetId: number };

type TestItem = {
  flashcardId: number;
  questionText: string; // term hoặc definition (tuỳ settings)
  correctAnswer: string; // definition hoặc term
  choices: string[];     // cho MC
  questionType: QuestionTypeOption;
};

type AnswerState = {
  answer: string;
  selectedIndex?: number;
  tfAnswer?: boolean;
  attempts: number;
  responseTimeMs: number;
};

/** Phân loại câu hỏi dựa trên index và settings (client-side) */
function assignQuestionType(
  index: number,
  total: number,
  allowedTypes: QuestionTypeOption[],
  hasChoices: boolean,
): QuestionTypeOption {
  // Nếu chỉ 1 loại được chọn
  if (allowedTypes.length === 1) return allowedTypes[0];
  // MC chỉ khả dụng khi có choices từ backend
  const available = allowedTypes.filter((t) => t !== "multipleChoice" || hasChoices);
  if (available.length === 0) return "written";
  // Phân bổ theo tỷ lệ: lặp vòng theo available types
  return available[index % available.length];
}

export function TestMode({ cards, studySetId }: Props) {
  const { token } = useAuth();
  const generation = useQuizGeneration(studySetId, "test", Math.min(cards.length, 100));
  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({ studySetId, mode: "test" });

  const [settings, setSettings] = React.useState<TestSettings | null>(null);
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

  const data = generation.state.state === "ready" ? generation.state.data : null;
  const rawItems = React.useMemo(() => data?.items ?? [], [data]);

  /* ── Build questions khi settings thay đổi ── */
  React.useEffect(() => {
    if (!settings || !rawItems.length) return;
    const mcItems = rawItems.filter((i) => i.kind === "question" || i.choices);
    const chosen = mcItems.slice(0, settings.limit);

    const built: TestItem[] = chosen.map((item, idx) => {
      const term = item.term ?? item.text ?? "";
      const definition = item.definition ?? "";
      const questionText = settings.askTerm ? term : definition;
      const correctAnswer = settings.askTerm ? definition : term;
      const hasChoices = (item.choices?.length ?? 0) >= 2;
      const qType = assignQuestionType(idx, chosen.length, settings.questionTypes, hasChoices);

      // Tạo TF: đôi lúc hiện đúng def, đôi lúc hiện def ngẫu nhiên từ item khác
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
        _tfDisplayDef: tfDef,
        _tfIsTrue: tfDef === correctAnswer,
      } as TestItem & { _tfDisplayDef: string; _tfIsTrue: boolean };
    });
    setQuestions(built);
    setAnswers({});
    setWrittenInputs({});
    setWrittenFeedback({});
    setWrittenSubmitting({});
    setSubmitted(false);
    setResult(null);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [settings, rawItems]);

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

  /* ── Settings screen ── */
  if (!settings) {
    return (
      <TestSettingsForm
        totalCards={cards.length}
        onStart={(s) => setSettings(s)}
      />
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

  function chooseTF(q: TestItem & { _tfIsTrue?: boolean }, isTrue: boolean) {
    if (answers[q.flashcardId] || submitted) return;
    const item = q as TestItem & { _tfIsTrue: boolean };
    const correct = item._tfIsTrue === isTrue;
    setAnswers((prev) => ({
      ...prev,
      [q.flashcardId]: {
        answer: isTrue ? "Đúng" : "Sai",
        tfAnswer: isTrue,
        attempts: 1,
        responseTimeMs: Date.now() - questionStartedAt,
        _tfCorrect: correct,
      } as AnswerState,
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
    } catch (e) {
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

    // Build payload — cho TF và written, dùng submitted string; MC dùng selectedIndex
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

      // Merge TF local correctness for items not evaluated server-side
      const merged = cardResults.map((cr) => {
        const q = questions.find((x) => x.flashcardId === cr.flashcardId);
        const ans = answers[cr.flashcardId] as AnswerState & { _tfCorrect?: boolean };
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
    setSettings(null);
    setQuestions([]);
    setAnswers({});
    setWrittenInputs({});
    setWrittenFeedback({});
    setWrittenSubmitting({});
    setSubmitted(false);
    setResult(null);
    setStartedAt(new Date());
    setError(null);
    generation.regenerate();
  }

  /* ── Result screen ── */
  if (result) {
    const pct = result.total ? Math.round((result.score / result.total) * 100) : 0;
    return (
      <div className="learn-done">
        <div className="test-result-header">
          <div className={`test-result-score-ring ${pct >= 80 ? "ring--green" : pct >= 50 ? "ring--yellow" : "ring--red"}`}>
            <span className="test-result-pct">{pct}%</span>
          </div>
          <div>
            <h2>Kết quả bài kiểm tra</h2>
            <p className="learn-score">
              Đúng <strong>{result.score}</strong> / {result.total} câu
            </p>
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
                  <span className="review-term">{q.questionText}</span>
                  <span className="review-answer">
                    Bạn trả lời: <em>{ans?.answer || "—"}</em>
                  </span>
                  {!isCorrect && (
                    <span className="review-correct-answer">
                      Đáp án đúng: <strong>{q.correctAnswer}</strong>
                    </span>
                  )}
                </div>
                <span className={`review-badge ${isCorrect ? "badge--correct" : "badge--wrong"}`}>
                  {isCorrect ? "Đúng" : "Sai"}
                </span>
              </div>
            );
          })}
        </div>
        <div className="test-result-actions">
          <button type="button" className="secondary-button" onClick={() => setSettings(null)}>
            Cài đặt lại
          </button>
          <button type="button" className="primary-button" onClick={restart}>
            Làm lại
          </button>
        </div>
      </div>
    );
  }

  /* ── Question list ── */
  return (
    <form
      className="test-mode test-mode--form"
      onSubmit={(e) => { e.preventDefault(); void submitTest(); }}
    >
      <div className="mode-progress-row">
        <div className="progress-bar-track" role="progressbar" aria-valuenow={answeredCount} aria-valuemax={questions.length}>
          <div className="progress-bar-fill" style={{ width: `${(answeredCount / questions.length) * 100}%` }} />
        </div>
        <span className="mode-progress-label">{answeredCount} / {questions.length} câu đã trả lời</span>
      </div>

      <div className="test-question-list">
        {questions.map((q, qIndex) => {
          const extQ = q as TestItem & { _tfDisplayDef?: string; _tfIsTrue?: boolean };
          const ans = answers[q.flashcardId] as (AnswerState & { _tfCorrect?: boolean }) | undefined;
          const writtenFb = writtenFeedback[q.flashcardId];

          return (
            <section key={q.flashcardId} className="test-question-block">
              <div className="test-question">
                <p className="test-question-label">
                  Câu {qIndex + 1} ·{" "}
                  {q.questionType === "multipleChoice" && "Chọn đáp án đúng"}
                  {q.questionType === "written" && "Tự điền đáp án"}
                  {q.questionType === "trueFalse" && "Đúng hay Sai?"}
                </p>
                <p className="test-term">{q.questionText}</p>
              </div>

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
                  displayDefinition={extQ._tfDisplayDef ?? q.correctAnswer}
                  correctDefinition={q.correctAnswer}
                  answer={ans?.tfAnswer !== undefined ? ans.tfAnswer : null}
                  revealed={Boolean(ans)}
                  onAnswer={(isTrue) => chooseTF(extQ as TestItem & { _tfIsTrue: boolean }, isTrue)}
                />
              )}

              {q.questionType === "written" && (
                <div className="ql-written-wrap">
                  <label className="ql-written-label" htmlFor={`written-${q.flashcardId}`}>
                    Nhập định nghĩa
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
                      {writtenFb ? "Chính xác!" : <>Chưa đúng. Đáp án: <strong>{q.correctAnswer}</strong></>}
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
          onClick={() => setSettings(null)}
        >
          ← Cài đặt lại
        </button>
        <button type="submit" className="primary-button" disabled={!allAnswered || submitted}>
          {submitted ? "Đang chấm…" : "Nộp bài"}
        </button>
      </div>
    </form>
  );
}
