import React from "react";
import type { QuizOption, QuizQuestion, QuizQuestionType } from "../../types";
import { CustomAudioPlayer } from "./exam/ExamQuestionLayout";
import { buildExamItems, PART_LABELS } from "./exam/toeic";
import { ExamTimer } from "./exam/ExamTimer";
import { contentProtection } from "./exam/contentProtection";
import { ExamGame } from "./exam/ExamGame";
import { ViewerWatermark } from "../../components/learning/ViewerWatermark";
import { quizQuestionToLike } from "./exam/types";

type QuizMode = "practice" | "exam";
type QuizUiLayout = "default" | "toeic";
type DetailTab = "explanation" | "vocabulary" | "grammar";
type PlayQuestion = { audioUrl?: string; srtUrl?: string; imageUrl?: string | null; key: string; questionText: string; questionType: Exclude<QuizQuestionType, "paragraph">; correctAnswer: string; answerExplanation?: string; paragraphText?: string; tags: string[]; options: QuizOption[] };
type RawSubQuestion = Partial<QuizQuestion> & { id?: number | string };
type SavedSession = {
  mode: QuizMode;
  uiLayout: QuizUiLayout;
  index: number;
  answers: Record<string, string>;
  activeKeys: string[];
  savedAt: string;
  deadline?: number | null;
  resultMode?: "instant" | "end";
};

function parseSubQuestions(value: unknown): RawSubQuestion[] {
  if (!value) return [];
  try { const parsed = typeof value === "string" ? JSON.parse(value) : value; return Array.isArray(parsed) ? parsed as RawSubQuestion[] : []; } catch { return []; }
}
function normalize(value = "") { return value.trim().toLocaleLowerCase(); }
export function flattenQuestions(questions: QuizQuestion[]): PlayQuestion[] {
  return questions.flatMap((question, questionIndex) => {
    if (question.questionType !== "paragraph") return [{ key: `q-${question.id ?? questionIndex}`, audioUrl: question.audioUrl, srtUrl: question.srtUrl, imageUrl: question.imageUrl, questionText: question.questionText, questionType: question.questionType, correctAnswer: question.correctAnswer ?? "", answerExplanation: question.answerExplanation, tags: question.tags ?? [], options: question.options ?? [] }];
    return parseSubQuestions(question.subQuestions).map((sub, subIndex) => ({
      key: `q-${question.id ?? questionIndex}-sub-${sub.id ?? subIndex}`, questionText: sub.questionText ?? `Câu ${subIndex + 1}`,
      questionType: sub.questionType === "true_false" || sub.questionType === "written" ? sub.questionType : "multiple_choice",
      correctAnswer: sub.correctAnswer ?? "", answerExplanation: sub.answerExplanation, paragraphText: question.paragraphText ?? question.questionText,
      audioUrl: sub.audioUrl || question.audioUrl, srtUrl: sub.srtUrl || question.srtUrl, imageUrl: sub.imageUrl || question.imageUrl,
      tags: sub.tags ?? question.tags ?? [], options: sub.options ?? [],
    }));
  });
}
function correctOptionText(question: PlayQuestion) {
  const marked = question.options.find((option) => option.isCorrect)?.text;
  if (marked) return marked;
  const index = /^[a-d]$/i.test(question.correctAnswer) ? question.correctAnswer.toUpperCase().charCodeAt(0) - 65 : -1;
  return index >= 0 ? question.options[index]?.text ?? question.correctAnswer : question.correctAnswer;
}
function isAnswerCorrect(question: PlayQuestion, submitted: string) {
  if (!submitted) return false;
  if (question.questionType === "multiple_choice") {
    if (question.options.find((option) => option.text === submitted)?.isCorrect) return true;
    return normalize(submitted) === normalize(correctOptionText(question));
  }
  return normalize(submitted) === normalize(question.correctAnswer);
}

export function QuizPlayer({ studySetId, questions }: { studySetId: number; questions: QuizQuestion[] }) {
  const allQuestions = React.useMemo(() => flattenQuestions(questions), [questions]);
  const toeicQuestions = React.useMemo(() => questions.map(quizQuestionToLike), [questions]);
  const [selectedPart, setSelectedPart] = React.useState<number | "all">("all");
  const [resultMode, setResultMode] = React.useState<"instant" | "end">("instant");
  const [durationMinutes, setDurationMinutes] = React.useState(120);
  const [practiceTimed, setPracticeTimed] = React.useState(false);
  const [deadline, setDeadline] = React.useState<number | null>(null);
  const [timeLeft, setTimeLeft] = React.useState(0);
  const examItems = React.useMemo(() => buildExamItems(toeicQuestions), [toeicQuestions]);
  const partCounts = React.useMemo(() => examItems.reduce<Record<number, number>>((counts, item) => {
    counts[item.part] = (counts[item.part] ?? 0) + 1;
    return counts;
  }, {}), [examItems]);
  const storageKey = `hquizlet:quiz-session:${studySetId}`;
  const [mode, setMode] = React.useState<QuizMode | null>(null);
  const [uiLayout, setUiLayout] = React.useState<QuizUiLayout>("default");
  const [preview, setPreview] = React.useState(false);
  const [activeKeys, setActiveKeys] = React.useState<string[]>([]);
  const playable = React.useMemo(() => activeKeys.length ? activeKeys.map((key) => allQuestions.find((q) => q.key === key)).filter(Boolean) as PlayQuestion[] : allQuestions, [activeKeys, allQuestions]);
  const [index, setIndex] = React.useState(0);
  const [answer, setAnswer] = React.useState("");
  const [answers, setAnswers] = React.useState<Record<string, string>>({});
  const [revealed, setRevealed] = React.useState(false);
  const [finished, setFinished] = React.useState(false);
  const [sorting, setSorting] = React.useState<QuizOption[]>([]);
  const [savedSession, setSavedSession] = React.useState<SavedSession | null>(null);
  const [savedNotice, setSavedNotice] = React.useState("");
  const [detailTab, setDetailTab] = React.useState<DetailTab>("explanation");
  const current = playable[index];
  const instantFeedback = mode === "practice" && resultMode === "instant";

  React.useEffect(() => {
    if (!mode || uiLayout === "toeic" || finished || !deadline) return;
    const tick = () => {
      const remaining = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
      setTimeLeft(remaining);
      if (remaining === 0) {
        setFinished(true);
        localStorage.removeItem(storageKey);
        setSavedSession(null);
      }
    };
    tick();
    const timer = window.setInterval(tick, 250);
    return () => window.clearInterval(timer);
  }, [mode, uiLayout, finished, deadline, storageKey]);

  React.useEffect(() => {
    try { const raw = localStorage.getItem(storageKey); if (raw) setSavedSession(JSON.parse(raw) as SavedSession); } catch { localStorage.removeItem(storageKey); }
  }, [storageKey]);
  React.useEffect(() => {
    setAnswer(current ? answers[current.key] ?? "" : "");
    setSorting([...(current?.options ?? [])].sort((a, b) => a.position - b.position));
    setRevealed(false);
    setDetailTab("explanation");
  }, [current?.key]); // eslint-disable-line react-hooks/exhaustive-deps

  const score = playable.reduce((total, question) => total + (isAnswerCorrect(question, answers[question.key] ?? "") ? 1 : 0), 0);
  const wrongQuestions = playable.filter((question) => !isAnswerCorrect(question, answers[question.key] ?? ""));
  const saveSession = React.useCallback((showNotice = true) => {
    if (!mode || finished || uiLayout === "toeic" || (deadline && deadline <= Date.now())) return;
    const session: SavedSession = {
      mode,
      uiLayout,
      index,
      answers,
      activeKeys: playable.map((q) => q.key),
      deadline, resultMode,
      savedAt: new Date().toISOString(),
    };
    localStorage.setItem(storageKey, JSON.stringify(session));
    setSavedSession(session);
    if (showNotice) { setSavedNotice("Đã lưu tiến độ"); window.setTimeout(() => setSavedNotice(""), 1800); }
  }, [activeKeys, answers, finished, index, mode, playable, storageKey, uiLayout, deadline, resultMode]);
  React.useEffect(() => { if (mode && !finished) saveSession(false); }, [answers, index, mode, saveSession, finished]);

  function storeAnswer(submitted: string) { if (!current || !submitted.trim()) return false; setAnswer(submitted); setAnswers((old) => ({ ...old, [current.key]: submitted })); return true; }
  function submitPractice(submitted = answer) { if (revealed || !storeAnswer(submitted)) return; setRevealed(true); }
  function chooseAnswer(submitted: string) { if (instantFeedback) submitPractice(submitted); else storeAnswer(submitted); }
  function submitSorting() { const submitted = sorting.map((option) => option.text).join(" → "); if (instantFeedback) submitPractice(submitted); else storeAnswer(submitted); }
  function goTo(nextIndex: number) { setIndex(Math.max(0, Math.min(playable.length - 1, nextIndex))); }
  function finishQuiz() { setFinished(true); localStorage.removeItem(storageKey); setSavedSession(null); }
  function next() { if (index + 1 >= playable.length) finishQuiz(); else goTo(index + 1); }
  function start(nextMode: QuizMode, keys = allQuestions.map((q) => q.key)) {
    const seconds = Math.max(1, Math.min(600, durationMinutes || 120)) * 60;
    setTimeLeft(seconds);
    setDeadline(nextMode === "exam" || practiceTimed ? Date.now() + seconds * 1000 : null); setMode(nextMode); setPreview(false); setActiveKeys(keys); setIndex(0); setAnswer(""); setAnswers({}); setRevealed(false); setFinished(false); }
  function resume() {
    if (!savedSession) return;
    setDeadline(savedSession.deadline ?? null);
    setResultMode(savedSession.resultMode ?? "instant");
    setMode(savedSession.mode);
    setUiLayout(savedSession.uiLayout ?? "default");
    setActiveKeys(savedSession.activeKeys);
    setAnswers(savedSession.answers);
    setIndex(Math.min(savedSession.index, savedSession.activeKeys.length - 1));
    setFinished(false);
    setPreview(false);
  }
  function leave() { saveSession(true); setMode(null); }

  if (!allQuestions.length) return <div className="quiz-player-empty">Quiz chưa có câu hỏi có thể làm.</div>;
  if (preview) return <section {...contentProtection} className="quiz-preview"><ViewerWatermark /><header><div><p className="eyebrow">Xem trước</p><h2>Toàn bộ {allQuestions.length} câu hỏi</h2></div><button className="secondary-button" onClick={() => setPreview(false)}>Đóng xem trước</button></header>{allQuestions.map((question, i) => <article key={question.key}><b>Câu {i + 1}</b>{question.paragraphText && <p className="quiz-preview-passage">{question.paragraphText}</p>}<h3>{question.questionText}</h3>{question.options.length > 0 && <ol type="A">{question.options.map((o) => <li key={o.text}>{o.text}</li>)}</ol>}<details><summary>Xem đáp án</summary><strong>{correctOptionText(question)}</strong>{question.answerExplanation && <p>{question.answerExplanation}</p>}</details></article>)}</section>;

  if (mode && uiLayout === "toeic") {
    return (
      <ExamGame
        questions={toeicQuestions}
        timerEnabled={mode === "exam" || practiceTimed}
        durationSeconds={Math.max(1, Math.min(600, durationMinutes || 120)) * 60}
        selectedPart={selectedPart}
        instantFeedback={instantFeedback}
        lockAudio={mode === "exam"}
        onExit={() => {
          setMode(null);
          localStorage.removeItem(storageKey);
          setSavedSession(null);
        }}
      />
    );
  }

  if (!mode) return <section className="quiz-mode-picker">
    <div className="quiz-mode-heading"><p className="eyebrow">Quiz</p><h2>Chọn cách học</h2><p>{allQuestions.length} câu hỏi đã sẵn sàng.</p>
      <div className="quiz-layout-picker" role="group" aria-label="Giao diện làm bài">
        <span className="quiz-layout-picker-label">Giao diện</span>
        <button type="button" className={`quiz-layout-option${uiLayout === "default" ? " active" : ""}`} onClick={() => setUiLayout("default")}>
          <strong>Mặc định</strong>
          <small>Phiếu trả lời bên phải, lưới đáp án 2×2.</small>
        </button>
        <button type="button" className={`quiz-layout-option${uiLayout === "toeic" ? " active" : ""}`} onClick={() => setUiLayout("toeic")}>
          <strong>TOEIC (IIG)</strong>
          <small>Hai cột, hướng dẫn Part, thanh điều hướng kiểu thi thật.</small>
        </button>
      </div>
      <div className="quiz-settings">
        {uiLayout === "toeic" && <label>Chọn Part<select value={selectedPart} onChange={(e) => setSelectedPart(e.target.value === "all" ? "all" : Number(e.target.value))}>
          <option value="all">Tất cả các Part ({examItems.length} câu)</option>
          {Object.entries(partCounts).map(([part, count]) => <option key={part} value={part}>{PART_LABELS[Number(part)]} ({count} câu)</option>)}
        </select></label>}
        <label>Xem kết quả khi luyện tập<select value={resultMode} onChange={(e) => setResultMode(e.target.value as "instant" | "end")}>
          <option value="instant">Ngay sau mỗi câu</option><option value="end">Sau khi nộp bài</option>
        </select></label>
        <label>Thời gian làm bài (phút)<input type="number" min={1} max={600} value={durationMinutes} onChange={(e) => setDurationMinutes(Math.max(1, Math.min(600, Number(e.target.value) || 1)))} /></label>
        <label><input type="checkbox" checked={practiceTimed} onChange={(e) => setPracticeTimed(e.target.checked)} /> Áp dụng thời gian cho luyện tập</label>
        <small>Thi thử luôn có đếm ngược. Hết giờ sẽ tự nộp bài.</small>
      </div>
      {savedSession && savedSession.uiLayout !== "toeic" && <button type="button" className="quiz-resume" onClick={resume}>▶ Làm tiếp từ câu {savedSession.index + 1}<small>Đã lưu {new Date(savedSession.savedAt).toLocaleString("vi-VN")}</small></button>}</div>
    <button type="button" className="quiz-mode-card" onClick={() => start("practice")}><span className="quiz-mode-icon">✦</span><strong>Luyện tập</strong><small>Xem đúng sai, từ vựng và ngữ pháp sau mỗi câu.</small><b>Bắt đầu →</b></button>
    <button type="button" className="quiz-mode-card" onClick={() => start("exam")}><span className="quiz-mode-icon quiz-mode-icon--exam">✓</span><strong>Thi thử</strong><small>Làm toàn bộ đề rồi nộp bài để xem phân tích.</small><b>Vào phòng thi →</b></button>
    <button type="button" className="quiz-mode-card quiz-mode-card--preview" onClick={() => setPreview(true)}><span className="quiz-mode-icon">◉</span><strong>Xem trước</strong><small>Xem toàn bộ câu hỏi và đáp án trước khi làm.</small><b>Mở bản xem trước →</b></button>
  </section>;

  if (finished) {
    const percent = Math.round((score / playable.length) * 100);
    const typeStats = Object.entries(playable.reduce<Record<string, { total: number; correct: number }>>((stats, q) => { const row = stats[q.questionType] ?? { total: 0, correct: 0 }; row.total++; if (isAnswerCorrect(q, answers[q.key] ?? "")) row.correct++; stats[q.questionType] = row; return stats; }, {}));
    return <section {...contentProtection} className="quiz-player quiz-player-result"><ViewerWatermark /><p className="eyebrow">Phân tích kết quả</p><h2>{percent}%</h2><p>Đúng <strong>{score}/{playable.length}</strong> · Sai hoặc bỏ trống <strong>{playable.length - score}</strong></p>
      <div className="quiz-analytics">{typeStats.map(([type, stat]) => <div key={type}><span>{type.replace("_", " ")}</span><strong>{stat.correct}/{stat.total}</strong><progress max={stat.total} value={stat.correct} /></div>)}</div>
      <div className="quiz-result-actions"><button className="secondary-button" onClick={() => setMode(null)}>Đổi chế độ</button>{wrongQuestions.length > 0 && <button className="secondary-button" onClick={() => start("practice", wrongQuestions.map((q) => q.key))}>Luyện lại {wrongQuestions.length} câu sai</button>}<button className="primary-button" onClick={() => start(mode)}>Làm lại toàn bộ</button></div>
      <div className="quiz-result-review">{playable.map((question, i) => { const submitted = answers[question.key] ?? ""; const correct = isAnswerCorrect(question, submitted); return <div key={question.key} className={correct ? "correct" : "wrong"}><b>{i + 1}</b><span>{question.questionText}<small>Bạn trả lời: {submitted || "Bỏ trống"}</small></span><strong>{correct ? "Đúng" : `Đáp án: ${correctOptionText(question)}`}</strong></div>; })}</div>
    </section>;
  }
  if (!current) return null;
  const options = current.questionType === "true_false" ? [{ text: "true", label: "Đúng" }, { text: "false", label: "Sai" }] : current.options.map((option) => ({ text: option.text, label: option.text }));
  const currentCorrect = isAnswerCorrect(current, answer);
  return <section {...contentProtection} className="quiz-session">
    <ViewerWatermark />
    <header className="quiz-session-toolbar"><button className="quiz-exit" onClick={leave}>← Thoát</button><strong>{mode === "practice" ? "Làm bài · Luyện tập" : "Làm bài · Thi thử"}</strong><div>{deadline && <ExamTimer secondsLeft={timeLeft} />}<button className="secondary-button" onClick={() => saveSession(true)}>▣ Lưu</button><span>{savedNotice || `Đã trả lời ${Object.keys(answers).length}/${playable.length}`}</span></div></header>
    <div className="quiz-player-progress"><span style={{ width: `${((index + 1) / playable.length) * 100}%` }} /></div>
    <div className={`quiz-session-layout${instantFeedback && revealed ? " has-detail" : ""}`}><main className="quiz-player quiz-player--active"><div className="quiz-player-progress-row"><span>Câu {index + 1}/{playable.length}</span><span>{instantFeedback ? `${score} câu đúng` : "Chọn đáp án phù hợp"}</span></div>
      {current.audioUrl && <CustomAudioPlayer key={current.audioUrl} src={current.audioUrl} srtUrl={current.srtUrl} muted={false} />}
      {current.imageUrl && <img src={current.imageUrl} alt="Hình minh họa câu hỏi" style={{ maxWidth: "100%", maxHeight: 320 }} draggable={false} />}
      {current.paragraphText && <div className="quiz-player-passage"><strong>Đoạn văn</strong><p>{current.paragraphText}</p></div>}<h2 className="quiz-player-question">{current.questionText}</h2>
      {(current.questionType === "multiple_choice" || current.questionType === "true_false") && <div className="quiz-player-options">{options.map((option, oi) => { const selected = answer === option.text; const correct = revealed && isAnswerCorrect(current, option.text); return <button key={`${option.text}-${oi}`} className={`quiz-player-option${selected ? " selected" : ""}${correct ? " correct" : ""}${revealed && selected && !currentCorrect ? " wrong" : ""}`} disabled={revealed} onClick={() => chooseAnswer(option.text)}><span>{String.fromCharCode(65 + oi)}</span>{option.label}</button>; })}</div>}
      {current.questionType === "written" && <div className="quiz-player-written"><input value={answer} disabled={revealed} placeholder="Nhập câu trả lời" onChange={(e) => setAnswer(e.target.value)} />{instantFeedback ? !revealed && <button className="primary-button" onClick={() => submitPractice()}>Kiểm tra</button> : <button className="primary-button" onClick={() => storeAnswer(answer)}>Lưu đáp án</button>}</div>}
      {current.questionType === "sorting" && <div className="quiz-player-sorting">{sorting.map((option, oi) => <div key={`${option.text}-${oi}`} className="quiz-player-sort-row"><span>{oi + 1}</span><strong>{option.text}</strong><button disabled={revealed || oi === 0} onClick={() => setSorting((items) => { const next = [...items]; [next[oi - 1], next[oi]] = [next[oi]!, next[oi - 1]!]; return next; })}>↑</button><button disabled={revealed || oi === sorting.length - 1} onClick={() => setSorting((items) => { const next = [...items]; [next[oi], next[oi + 1]] = [next[oi + 1]!, next[oi]!]; return next; })}>↓</button></div>)}{!revealed && <button className="primary-button" onClick={submitSorting}>{instantFeedback ? "Kiểm tra thứ tự" : "Lưu thứ tự"}</button>}</div>}
      {instantFeedback && revealed && <div className={`quiz-player-feedback ${currentCorrect ? "correct" : "wrong"}`}><strong>{currentCorrect ? "Chính xác!" : `Đáp án đúng: ${correctOptionText(current)}`}</strong><button className="primary-button" onClick={next}>{index + 1 === playable.length ? "Xem kết quả" : "Câu tiếp theo"}</button></div>}
      {!instantFeedback && <div className="quiz-exam-navigation"><button className="secondary-button" disabled={index === 0} onClick={() => goTo(index - 1)}>← Câu trước</button>{index + 1 === playable.length ? <button className="primary-button" onClick={finishQuiz}>Nộp bài</button> : <button className="primary-button" onClick={() => goTo(index + 1)}>Câu tiếp theo →</button>}</div>}
    </main>
    {instantFeedback && revealed ? <aside className="quiz-detail-panel"><nav>{(["explanation", "vocabulary", "grammar"] as DetailTab[]).map((tab) => <button key={tab} className={detailTab === tab ? "active" : ""} onClick={() => setDetailTab(tab)}>{tab === "explanation" ? "Giải thích" : tab === "vocabulary" ? "Từ vựng" : "Ngữ pháp"}</button>)}</nav><div className="quiz-detail-content">{detailTab === "explanation" && <><h3>{currentCorrect ? "✓ Chính xác" : "✕ Chưa đúng"}</h3><p><b>Bạn chọn:</b> {answer || "Bỏ trống"}</p><p><b>Đáp án:</b> {correctOptionText(current)}</p><hr/><p>{current.answerExplanation || "Chưa có giải thích cho câu hỏi này."}</p></>}{detailTab === "vocabulary" && <><h3>Từ vựng liên quan</h3>{current.tags.length ? current.tags.map((tag) => <span className="quiz-tag" key={tag}>{tag}</span>) : <p>Các lựa chọn trong câu:</p>}<ul>{current.options.map((o) => <li key={o.text}>{o.text}</li>)}</ul></>}{detailTab === "grammar" && <><h3>Phân tích ngữ pháp</h3><p>{current.answerExplanation || "Chưa có ghi chú ngữ pháp riêng cho câu hỏi này."}</p></>}</div></aside>
      : <aside className="quiz-answer-sheet"><strong>Phiếu trả lời</strong><div>{playable.map((q, qi) => <button key={q.key} className={`${qi === index ? "active" : ""}${answers[q.key] ? " answered" : ""}`} onClick={() => goTo(qi)}>{qi + 1}</button>)}</div><small>Ô xanh là câu đã trả lời.</small></aside>}
    </div>
  </section>;
}
