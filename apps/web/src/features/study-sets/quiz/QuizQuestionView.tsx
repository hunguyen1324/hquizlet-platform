import React from "react";
import type { ProtectedQuizView } from "../../../lib/api/protectedQuiz";
import { RichTextContent } from "../exam/RichTextContent";
import { answerLabel, correctText, isCorrectOption, isTypingTarget } from "./quizHelpers";

type Item = ProtectedQuizView["page"][number];

export function QuizQuestionView({ item, number, total, answer, pendingValue, result, instant, submitted, audio, onAnswer }: {
  item: Item;
  number: number;
  total: number;
  answer?: string;
  pendingValue?: string;
  result?: boolean;
  instant: boolean;
  submitted: boolean;
  audio?: React.ReactNode;
  onAnswer: (value: string) => void;
}) {
  const q = item.question;
  const chosen = answer ?? pendingValue;
  const locked = submitted || (instant && chosen !== undefined);
  const revealed = item.revealed;
  const choices = q.questionType === "true_false"
    ? [{ value: "true", label: "Đúng" }, { value: "false", label: "Sai" }]
    : q.questionType === "multiple_choice" ? (q.options ?? []).map((o) => ({ value: o.text, label: o.text })) : [];

  // 1–9 or A–D picks an answer; ignored while typing.
  React.useEffect(() => {
    if (!choices.length || locked) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.ctrlKey || e.metaKey || e.altKey || isTypingTarget(e.target)) return;
      const k = e.key.toLowerCase();
      const idx = /^[1-9]$/.test(k) ? Number(k) - 1 : /^[a-d]$/.test(k) ? k.charCodeAt(0) - 97 : -1;
      const picked = choices[idx];
      if (picked) { e.preventDefault(); onAnswer(picked.value); }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [choices, locked, onAnswer]);

  const [draft, setDraft] = React.useState(answer ?? "");
  const [order, setOrder] = React.useState(() => (q.options ?? []).map((o) => o.text));
  const move = (i: number, d: -1 | 1) => setOrder((old) => { const next = [...old]; [next[i], next[i + d]] = [next[i + d]!, next[i]!]; return next; });

  return <article className="qz-column" aria-labelledby={`qz-q-${item.index}`}>
    {audio}
    {q.imageUrl && <img className="qz-image" src={q.imageUrl} alt="Hình minh họa câu hỏi" decoding="async" draggable={false} />}
    {q.paragraphText && <RichTextContent value={q.paragraphText} className="qz-passage" />}
    <div id={`qz-q-${item.index}`} className="qz-question"><span className="qz-muted">Câu {number}/{total}</span><RichTextContent value={q.questionText} /></div>

    {choices.length > 0 && <fieldset className="qz-choices">
      <legend className="qz-hint">{locked ? "" : "Chọn một đáp án (phím 1–4 hoặc A–D)"}</legend>
      {choices.map((c, i) => {
        const selected = chosen === c.value;
        const good = revealed && isCorrectOption(q, c.value);
        const bad = revealed && selected && !good;
        return <button key={c.value} type="button" disabled={locked} aria-pressed={selected}
          className={`qz-choice${selected ? " selected" : ""}${good ? " correct" : ""}${bad ? " wrong" : ""}${pendingValue === c.value && answer === undefined ? " pending" : ""}`}
          onClick={() => onAnswer(c.value)}>
          <span className="qz-letter">{String.fromCharCode(65 + i)}</span>
          <span className="qz-choice-text">{c.label}</span>
          {good && <span className="qz-choice-mark" aria-label="Đáp án đúng">✓</span>}
          {bad && <span className="qz-choice-mark" aria-label="Bạn chọn sai">✗</span>}
        </button>;
      })}
    </fieldset>}

    {q.questionType === "written" && <form className="qz-written" onSubmit={(e) => { e.preventDefault(); if (draft.trim() && !locked) onAnswer(draft.trim()); }}>
      <input aria-label="Câu trả lời" placeholder="Nhập câu trả lời" value={chosen !== undefined && locked ? chosen : draft} disabled={locked} onChange={(e) => setDraft(e.target.value)} />
      {!locked && <button className="qz-btn primary" disabled={!draft.trim()}>{instant ? "Kiểm tra" : "Lưu đáp án"}</button>}
    </form>}

    {q.questionType === "sorting" && <div className="qz-sort">
      <p className="qz-hint">Dùng mũi tên để sắp xếp đúng thứ tự.</p>
      {order.map((text, i) => <div className="qz-sort-row" key={text}>
        <span className="qz-letter">{i + 1}</span><strong>{text}</strong>
        <button disabled={locked || i === 0} aria-label="Đưa lên" onClick={() => move(i, -1)}>↑</button>
        <button disabled={locked || i === order.length - 1} aria-label="Đưa xuống" onClick={() => move(i, 1)}>↓</button>
      </div>)}
      {!locked && <button className="qz-btn primary" onClick={() => onAnswer(order.join(" → "))}>{instant ? "Kiểm tra thứ tự" : "Lưu thứ tự"}</button>}
    </div>}

    {revealed && <section className={`qz-feedback ${result ? "correct" : "wrong"}`} role="status">
      <h3>{result ? "Chính xác!" : "Chưa đúng"}</h3>
      {!result && chosen !== undefined && <div>Bạn chọn: <strong>{answerLabel(q, chosen)}</strong></div>}
      {!result && <div>Đáp án đúng: <strong>{answerLabel(q, correctText(q))}</strong></div>}
      {q.answerExplanation && <RichTextContent value={q.answerExplanation} />}
    </section>}
    {submitted && !revealed && <section className="qz-feedback skipped"><h3>Bạn chưa trả lời câu này</h3><div>Hãy làm lại bài để xem đáp án và giải thích.</div></section>}
  </article>;
}
