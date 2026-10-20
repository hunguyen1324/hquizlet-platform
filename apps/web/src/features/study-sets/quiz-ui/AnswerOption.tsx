/** Visual state of one answer option. Correctness is never colour-only: each
 * state also carries a glyph and a text label, so it survives grayscale,
 * colour-blindness and screen readers. */
export type OptionState = "idle" | "selected" | "pending" | "correct" | "wrong" | "passed";

const STATE_LABEL: Record<OptionState, string> = {
  idle: "",
  selected: "Đã chọn",
  pending: "Đang chấm…",
  correct: "Đúng",
  wrong: "Sai",
  passed: "Đáp án đúng",
};

export function AnswerOption({
  letter,
  text,
  state,
  disabled,
  onSelect,
}: {
  letter: string;
  text: string;
  state: OptionState;
  disabled: boolean;
  onSelect: () => void;
}) {
  const label = STATE_LABEL[state];
  return (
    <button
      type="button"
      className={`qz-answer qz-answer--${state}`}
      aria-pressed={state === "selected" || state === "pending" || state === "correct"}
      aria-busy={state === "pending" || undefined}
      disabled={disabled}
      onClick={onSelect}
    >
      <span className="qz-answer-letter" aria-hidden="true">
        {letter}
      </span>
      <span className="qz-answer-text">{text}</span>
      {label ? (
        <span className="qz-answer-state">
          <span aria-hidden="true">{state === "pending" ? "◌" : state === "wrong" ? "✗" : "✓"}</span>
          <span>{label}</span>
        </span>
      ) : null}
    </button>
  );
}
