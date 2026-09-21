import React from "react";
import type { QuizOption, QuizQuestion, QuizQuestionType } from "../../types";

type QuizMode = "practice" | "exam";
type PlayQuestion = { key: string; questionText: string; questionType: Exclude<QuizQuestionType, "paragraph">; correctAnswer: string; answerExplanation?: string; paragraphText?: string; options: QuizOption[] };
type RawSubQuestion = Partial<QuizQuestion> & { id?: number | string };

function parseSubQuestions(value: unknown): RawSubQuestion[] {
  if (!value) return [];
  try { const parsed = typeof value === "string" ? JSON.parse(value) : value; return Array.isArray(parsed) ? parsed as RawSubQuestion[] : []; } catch { return []; }
}
function normalize(value = "") { return value.trim().toLocaleLowerCase(); }
function flattenQuestions(questions: QuizQuestion[]): PlayQuestion[] {
  return questions.flatMap((question, questionIndex) => {
    if (question.questionType !== "paragraph") return [{ key: `q-${question.id ?? questionIndex}`, questionText: question.questionText, questionType: question.questionType, correctAnswer: question.correctAnswer ?? "", answerExplanation: question.answerExplanation, options: question.options ?? [] }];
    return parseSubQuestions(question.subQuestions).map((subQuestion, subIndex) => ({
      key: `q-${question.id ?? questionIndex}-sub-${subQuestion.id ?? subIndex}`,
      questionText: subQuestion.questionText ?? `Câu ${subIndex + 1}`,
      questionType: subQuestion.questionType === "true_false" || subQuestion.questionType === "written" ? subQuestion.questionType : "multiple_choice",
      correctAnswer: subQuestion.correctAnswer ?? "", answerExplanation: subQuestion.answerExplanation,
      paragraphText: question.paragraphText ?? question.questionText, options: subQuestion.options ?? [],
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
  if (question.questionType === "multiple_choice") {
    if (question.options.find((option) => option.text === submitted)?.isCorrect) return true;
    return normalize(submitted) === normalize(correctOptionText(question));
  }
  return normalize(submitted) === normalize(question.correctAnswer);
}

export function QuizPlayer({ questions }: { questions: QuizQuestion[] }) {
  const playable = React.useMemo(() => flattenQuestions(questions), [questions]);
  const [mode, setMode] = React.useState<QuizMode | null>(null);
  const [index, setIndex] = React.useState(0);
  const [answer, setAnswer] = React.useState("");
  const [answers, setAnswers] = React.useState<Record<string, string>>({});
  const [revealed, setRevealed] = React.useState(false);
  const [finished, setFinished] = React.useState(false);
  const [sorting, setSorting] = React.useState<QuizOption[]>([]);
  const current = playable[index];

  React.useEffect(() => {
    setAnswer(current ? answers[current.key] ?? "" : "");
    setSorting([...(current?.options ?? [])].sort((a, b) => a.position - b.position));
    setRevealed(false);
  }, [current?.key]); // eslint-disable-line react-hooks/exhaustive-deps

  const score = playable.reduce((total, question) => total + (isAnswerCorrect(question, answers[question.key] ?? "") ? 1 : 0), 0);
  function storeAnswer(submitted: string) { if (!current || !submitted.trim()) return false; setAnswer(submitted); setAnswers((old) => ({ ...old, [current.key]: submitted })); return true; }
  function submitPractice(submitted = answer) { if (revealed || !storeAnswer(submitted)) return; setRevealed(true); }
  function chooseAnswer(submitted: string) { if (mode === "practice") submitPractice(submitted); else storeAnswer(submitted); }
  function submitSorting() { const submitted = sorting.map((option) => option.text).join(" → "); if (mode === "practice") submitPractice(submitted); else storeAnswer(submitted); }
  function goTo(nextIndex: number) { setIndex(Math.max(0, Math.min(playable.length - 1, nextIndex))); }
  function next() { if (index + 1 >= playable.length) setFinished(true); else goTo(index + 1); }
  function start(nextMode: QuizMode) { setMode(nextMode); setIndex(0); setAnswer(""); setAnswers({}); setRevealed(false); setFinished(false); }

  if (playable.length === 0) return <div className="quiz-player-empty">Quiz chưa có câu hỏi có thể làm.</div>;
  if (!mode) return <section className="quiz-mode-picker">
    <div className="quiz-mode-heading"><p className="eyebrow">Sẵn sàng bắt đầu</p><h2>Chọn cách làm quiz</h2><p>{playable.length} câu hỏi đã sẵn sàng.</p></div>
    <button type="button" className="quiz-mode-card" onClick={() => start("practice")}><span className="quiz-mode-icon">✦</span><strong>Luyện tập</strong><small>Xem đúng sai và giải thích ngay sau mỗi câu.</small><b>Bắt đầu →</b></button>
    <button type="button" className="quiz-mode-card" onClick={() => start("exam")}><span className="quiz-mode-icon quiz-mode-icon--exam">✓</span><strong>Thi thử</strong><small>Làm toàn bộ đề rồi nộp bài để xem điểm.</small><b>Vào phòng thi →</b></button>
  </section>;

  if (finished) {
    const percent = Math.round((score / playable.length) * 100);
    return <section className="quiz-player quiz-player-result"><p className="eyebrow">Kết quả {mode === "exam" ? "thi thử" : "luyện tập"}</p><h2>{percent}%</h2><p>Trả lời đúng <strong>{score} / {playable.length}</strong> câu</p>
      <div className="quiz-result-actions"><button type="button" className="secondary-button" onClick={() => setMode(null)}>Đổi chế độ</button><button type="button" className="primary-button" onClick={() => start(mode)}>Làm lại</button></div>
      <div className="quiz-result-review">{playable.map((question, questionIndex) => { const submitted = answers[question.key] ?? ""; const correct = isAnswerCorrect(question, submitted); return <div key={question.key} className={correct ? "correct" : "wrong"}><b>{questionIndex + 1}</b><span>{question.questionText}</span><strong>{correct ? "Đúng" : `Đáp án: ${correctOptionText(question)}`}</strong></div>; })}</div>
    </section>;
  }
  if (!current) return null;
  const options = current.questionType === "true_false" ? [{ text: "true", label: "Đúng" }, { text: "false", label: "Sai" }] : current.options.map((option) => ({ text: option.text, label: option.text }));
  const currentCorrect = isAnswerCorrect(current, answer);
  return <section className="quiz-session">
    <header className="quiz-session-toolbar"><button type="button" className="quiz-exit" onClick={() => setMode(null)}>← Thoát</button><strong>{mode === "practice" ? "Luyện tập" : "Thi thử"}</strong><span>Đã trả lời {Object.keys(answers).length}/{playable.length}</span></header>
    <div className="quiz-player-progress"><span style={{ width: `${((index + 1) / playable.length) * 100}%` }} /></div>
    <div className="quiz-session-layout"><main className="quiz-player quiz-player--active"><div className="quiz-player-progress-row"><span>Câu {index + 1} / {playable.length}</span><span>{mode === "practice" ? `${score} câu đúng` : "Chọn đáp án phù hợp"}</span></div>
      {current.paragraphText && <div className="quiz-player-passage"><strong>Đoạn văn</strong><p>{current.paragraphText}</p></div>}
      <h2 className="quiz-player-question">{current.questionText}</h2>
      {(current.questionType === "multiple_choice" || current.questionType === "true_false") && <div className="quiz-player-options">{options.map((option, optionIndex) => { const selected = answer === option.text; const correct = revealed && isAnswerCorrect(current, option.text); return <button type="button" key={`${option.text}-${optionIndex}`} className={`quiz-player-option${selected ? " selected" : ""}${correct ? " correct" : ""}${revealed && selected && !currentCorrect ? " wrong" : ""}`} disabled={revealed} onClick={() => chooseAnswer(option.text)}><span>{String.fromCharCode(65 + optionIndex)}</span>{option.label}</button>; })}</div>}
      {current.questionType === "written" && <div className="quiz-player-written"><input value={answer} disabled={revealed} placeholder="Nhập câu trả lời" onChange={(event) => setAnswer(event.target.value)} onKeyDown={(event) => { if (event.key === "Enter") mode === "practice" ? submitPractice() : storeAnswer(answer); }} />{mode === "practice" ? !revealed && <button type="button" className="primary-button" onClick={() => submitPractice()}>Kiểm tra</button> : <button type="button" className="primary-button" onClick={() => storeAnswer(answer)}>Lưu đáp án</button>}</div>}
      {current.questionType === "sorting" && <div className="quiz-player-sorting">{sorting.map((option, optionIndex) => <div key={`${option.text}-${optionIndex}`} className="quiz-player-sort-row"><span>{optionIndex + 1}</span><strong>{option.text}</strong><button type="button" disabled={revealed || optionIndex === 0} onClick={() => setSorting((items) => { const nextItems = [...items]; [nextItems[optionIndex - 1], nextItems[optionIndex]] = [nextItems[optionIndex]!, nextItems[optionIndex - 1]!]; return nextItems; })}>↑</button><button type="button" disabled={revealed || optionIndex === sorting.length - 1} onClick={() => setSorting((items) => { const nextItems = [...items]; [nextItems[optionIndex], nextItems[optionIndex + 1]] = [nextItems[optionIndex + 1]!, nextItems[optionIndex]!]; return nextItems; })}>↓</button></div>)}{!revealed && <button type="button" className="primary-button" onClick={submitSorting}>{mode === "practice" ? "Kiểm tra thứ tự" : "Lưu thứ tự"}</button>}</div>}
      {mode === "practice" && revealed && <div className={`quiz-player-feedback ${currentCorrect ? "correct" : "wrong"}`}><strong>{currentCorrect ? "Chính xác!" : `Đáp án đúng: ${correctOptionText(current)}`}</strong>{current.answerExplanation && <p>{current.answerExplanation}</p>}<button type="button" className="primary-button" onClick={next}>{index + 1 === playable.length ? "Xem kết quả" : "Câu tiếp theo"}</button></div>}
      {mode === "exam" && <div className="quiz-exam-navigation"><button type="button" className="secondary-button" disabled={index === 0} onClick={() => goTo(index - 1)}>← Câu trước</button>{index + 1 === playable.length ? <button type="button" className="primary-button" onClick={() => setFinished(true)}>Nộp bài</button> : <button type="button" className="primary-button" onClick={() => goTo(index + 1)}>Câu tiếp theo →</button>}</div>}
    </main>{mode === "exam" && <aside className="quiz-answer-sheet"><strong>Phiếu trả lời</strong><div>{playable.map((question, questionIndex) => <button type="button" key={question.key} className={`${questionIndex === index ? "active" : ""}${answers[question.key] ? " answered" : ""}`} onClick={() => goTo(questionIndex)}>{questionIndex + 1}</button>)}</div><small>Ô màu xanh là câu đã trả lời.</small></aside>}</div>
  </section>;
}
