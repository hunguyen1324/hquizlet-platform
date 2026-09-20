// MultipleChoiceQuestion — 4 lựa chọn, highlight đúng/sai sau khi chọn
import React from "react";
import "./learning.css";

type Props = {
  choices: string[];
  selected: string | null;
  revealed: boolean;
  correctAnswer: string;
  onChoose: (choice: string, index: number) => void;
};

export function MultipleChoiceQuestion({
  choices,
  selected,
  revealed,
  correctAnswer,
  onChoose,
}: Props) {
  return (
    <div className="ql-mc-options">
      {choices.map((choice, i) => {
        const isSelected = selected === choice;
        const isCorrect = choice === correctAnswer;
        let cls = "ql-mc-option";
        if (isSelected) cls += " ql-mc-option--selected";
        if (revealed) {
          if (isCorrect) cls += " ql-mc-option--correct";
          else if (isSelected) cls += " ql-mc-option--wrong";
        }
        return (
          <button
            key={`${choice}-${i}`}
            type="button"
            id={`mc-option-${i}`}
            className={cls}
            onClick={() => onChoose(choice, i)}
            disabled={revealed}
            aria-pressed={isSelected}
          >
            <span className="ql-mc-option-letter">{String.fromCharCode(65 + i)}</span>
            <span className="ql-mc-option-text">{choice}</span>
            {revealed && isCorrect && (
              <svg className="ql-mc-check" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" aria-hidden="true">
                <polyline points="20 6 9 17 4 12" />
              </svg>
            )}
            {revealed && isSelected && !isCorrect && (
              <svg className="ql-mc-cross" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" aria-hidden="true">
                <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            )}
          </button>
        );
      })}
    </div>
  );
}
