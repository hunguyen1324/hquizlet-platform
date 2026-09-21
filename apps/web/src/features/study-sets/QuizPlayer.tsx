import React from "react";
import type { QuizOption, QuizQuestion, QuizQuestionType } from "../../types";

type PlayQuestion = {
  key: string;
  questionText: string;
  questionType: Exclude<QuizQuestionType, "paragraph">;
  correctAnswer: string;
  answerExplanation?: string;
  paragraphText?: string;
  options: QuizOption[];
};

type RawSubQuestion = Partial<QuizQuestion> & { id?: number | string };

function parseSubQuestions(value: unknown): RawSubQuestion[] {
  if (!value) return [];
  try {
    const parsed = typeof value === "string" ? JSON.parse(value) : value;
    return Array.isArray(parsed) ? (parsed as RawSubQuestion[]) : [];
  } catch {
    return [];
  }
}

function normalize(value: string) {
  return value.trim().toLocaleLowerCase();
}

function flattenQuestions(questions: QuizQuestion[]): PlayQuestion[] {
  return questions.flatMap((question, questionIndex) => {
    if (question.questionType !== "paragraph") {
      return [{
        key: `q-${question.id ?? questionIndex}`,
        questionText: question.questionText,
        questionType: question.questionType,
        correctAnswer: question.correctAnswer ?? "",
        answerExplanation: question.answerExplanation,
        options: question.options ?? [],
      }];
    }

    const subQuestions = parseSubQuestions(question.subQuestions);
    return subQuestions.map((subQuestion, subIndex) => ({
      key: `q-${question.id ?? questionIndex}-sub-${subQuestion.id ?? subIndex}`,
      questionText: subQuestion.questionText ?? `Câu ${subIndex + 1}`,
      questionType: subQuestion.questionType === "true_false" || subQuestion.questionType === "written"
        ? subQuestion.questionType
        : "multiple_choice",
      correctAnswer: subQuestion.correctAnswer ?? "",
      answerExplanation: subQuestion.answerExplanation,
      paragraphText: question.paragraphText ?? question.questionText,
      options: subQuestion.options ?? [],
    }));
  });
}

export function QuizPlayer({ questions }: { questions: QuizQuestion[] }) {
  const playable = React.useMemo(() => flattenQuestions(questions), [questions]);
  const [started, setStarted] = React.useState(false);
  const [index, setIndex] = React.useState(0);
  const [answer, setAnswer] = React.useState("");
  const [revealed, setRevealed] = React.useState(false);
  const [score, setScore] = React.useState(0);
  const [finished, setFinished] = React.useState(false);
  const [sorting, setSorting] = React.useState<QuizOption[]>([]);

  const current = playable[index];

  React.useEffect(() => {
    setSorting([...(current?.options ?? [])].sort((a, b) => a.position - b.position));
  }, [current]);

  function isCorrect(submitted: string) {
    if (!current) return false;
    if (current.questionType === "multiple_choice") {
      const selectedIndex = current.options.findIndex((option) => option.text === submitted);
      const selectedOption = current.options[selectedIndex];
      if (selectedOption?.isCorrect) return true;
      if (selectedIndex >= 0 && normalize(current.correctAnswer) === String.fromCharCode(97 + selectedIndex)) return true;
    }
    return normalize(submitted) === normalize(current.correctAnswer);
  }

  function submit(submitted = answer) {
    if (!current || revealed || !submitted.trim()) return;
    setAnswer(submitted);
    setRevealed(true);
    if (isCorrect(submitted)) setScore((value) => value + 1);
  }

  function submitSorting() {
    const submitted = sorting.map((option) => option.text).join(" → ");
    submit(submitted);
  }

  function next() {
    if (index + 1 >= playable.length) {
      setFinished(true);
      return;
    }
    setIndex((value) => value + 1);
    setAnswer("");
    setRevealed(false);
  }

  function restart() {
    setIndex(0);
    setAnswer("");
    setRevealed(false);
    setScore(0);
    setFinished(false);
    setStarted(true);
  }

  if (playable.length === 0) {
    return <div className="quiz-player-empty">Quiz chưa có câu hỏi có thể làm.</div>;
  }

  if (!started) {
    return (
      <section className="quiz-player quiz-player-start">
        <div className="quiz-player-icon" aria-hidden>✓</div>
        <div>
          <p className="eyebrow">Quiz</p>
          <h2>Luyện tập với {playable.length} câu hỏi</h2>
          <p>Làm lần lượt các câu hỏi và xem đáp án ngay sau mỗi câu.</p>
        </div>
        <button type="button" className="primary-button" onClick={() => setStarted(true)}>
          Bắt đầu làm quiz
        </button>
      </section>
    );
  }

  if (finished) {
    const percent = Math.round((score / playable.length) * 100);
    return (
      <section className="quiz-player quiz-player-result">
        <p className="eyebrow">Kết quả</p>
        <h2>{percent}%</h2>
        <p>Đúng {score} / {playable.length} câu</p>
        <button type="button" className="primary-button" onClick={restart}>Làm lại</button>
      </section>
    );
  }

  if (!current) return null;

  const options = current.questionType === "true_false"
    ? [
        { text: "true", label: "Đúng" },
        { text: "false", label: "Sai" },
      ]
    : current.options.map((option) => ({ text: option.text, label: option.text }));

  return (
    <section className="quiz-player">
      <div className="quiz-player-progress-row">
        <span>Câu {index + 1} / {playable.length}</span>
        <span>{score} câu đúng</span>
      </div>
      <div className="quiz-player-progress"><span style={{ width: `${((index + 1) / playable.length) * 100}%` }} /></div>

      {current.paragraphText && (
        <div className="quiz-player-passage">
          <strong>Đoạn văn</strong>
          <p>{current.paragraphText}</p>
        </div>
      )}

      <h2 className="quiz-player-question">{current.questionText}</h2>

      {(current.questionType === "multiple_choice" || current.questionType === "true_false") && (
        <div className="quiz-player-options">
          {options.map((option, optionIndex) => {
            const selected = answer === option.text;
            const correct = revealed && isCorrect(option.text);
            return (
              <button
                type="button"
                key={`${option.text}-${optionIndex}`}
                className={`quiz-player-option${selected ? " selected" : ""}${correct ? " correct" : ""}`}
                disabled={revealed}
                onClick={() => submit(option.text)}
              >
                <span>{String.fromCharCode(65 + optionIndex)}</span>{option.label}
              </button>
            );
          })}
        </div>
      )}

      {current.questionType === "written" && (
        <div className="quiz-player-written">
          <input
            value={answer}
            disabled={revealed}
            placeholder="Nhập câu trả lời"
            onChange={(event) => setAnswer(event.target.value)}
            onKeyDown={(event) => { if (event.key === "Enter") submit(); }}
          />
          {!revealed && <button type="button" className="primary-button" onClick={() => submit()}>Kiểm tra</button>}
        </div>
      )}

      {current.questionType === "sorting" && (
        <div className="quiz-player-sorting">
          {sorting.map((option, optionIndex) => (
            <div key={`${option.text}-${optionIndex}`} className="quiz-player-sort-row">
              <span>{optionIndex + 1}</span><strong>{option.text}</strong>
              <button type="button" disabled={revealed || optionIndex === 0} onClick={() => setSorting((items) => {
                const nextItems = [...items];
                [nextItems[optionIndex - 1], nextItems[optionIndex]] = [nextItems[optionIndex]!, nextItems[optionIndex - 1]!];
                return nextItems;
              })}>↑</button>
              <button type="button" disabled={revealed || optionIndex === sorting.length - 1} onClick={() => setSorting((items) => {
                const nextItems = [...items];
                [nextItems[optionIndex], nextItems[optionIndex + 1]] = [nextItems[optionIndex + 1]!, nextItems[optionIndex]!];
                return nextItems;
              })}>↓</button>
            </div>
          ))}
          {!revealed && <button type="button" className="primary-button" onClick={submitSorting}>Kiểm tra thứ tự</button>}
        </div>
      )}

      {revealed && (
        <div className={`quiz-player-feedback ${isCorrect(answer) ? "correct" : "wrong"}`}>
          <strong>{isCorrect(answer) ? "Chính xác!" : `Đáp án đúng: ${current.correctAnswer}`}</strong>
          {current.answerExplanation && <p>{current.answerExplanation}</p>}
          <button type="button" className="primary-button" onClick={next}>
            {index + 1 === playable.length ? "Xem kết quả" : "Câu tiếp theo"}
          </button>
        </div>
      )}
    </section>
  );
}
