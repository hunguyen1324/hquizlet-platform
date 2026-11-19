/** Keep status available to screen readers without taking space from the answer.
 * The result panel shows the full correctness message. */
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
  percentage,
  hideOption = false,
}: {
  letter: string;
  text: string;
  state: OptionState;
  disabled: boolean;
  onSelect: () => void;
  percentage?: number;
  hideOption?: boolean;
}) {
  const label = STATE_LABEL[state];
  return (
    <button
      type="button"
      className={`qz-answer qz-answer--${state}${hideOption ? " qz-slot-hidden" : ""}`}
      aria-pressed={state === "selected" || state === "pending" || state === "correct"}
      aria-busy={state === "pending" || undefined}
      disabled={disabled}
      onClick={onSelect}
    >
      <span className="qz-answer-letter" aria-hidden="true">
        {letter}
      </span>
      <span className="qz-answer-text">{text}</span>
      {percentage !== undefined && <span className="qz-answer-percent">{percentage}%</span>}
      {label ? (
        <span className="qz-answer-state qz-answer-state--hidden">
          <span aria-hidden="true">{state === "pending" ? "◌" : state === "wrong" ? "✗" : "✓"}</span>
          <span>{label}</span>
        </span>
      ) : null}
    </button>
  );
}
