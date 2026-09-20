import React from "react";
import type { Flashcard } from "../../types";

type Props = {
  card: Flashcard;
  onToggleStar?: (card: Flashcard) => void;
};

export function FlashcardListCard({ card, onToggleStar }: Props) {
  const [expanded, setExpanded] = React.useState(false);
  const hasExtra = Boolean(
    card.imageUrl || card.exampleSentence || card.hintExplanation,
  );

  return (
    <div className="sd-fc-row-wrap">
      <article className="sd-fc-row">
        <div className="sd-fc-row-main">
          {card.imageUrl && (
            <div className="sd-fc-row-thumb">
              <img src={card.imageUrl} alt="" />
            </div>
          )}
          <div className="sd-fc-row-term">{card.term}</div>
          <div className="sd-fc-row-sep" aria-hidden />
          <div className="sd-fc-row-def">{card.definition}</div>
          <div className="sd-fc-row-actions">
            {onToggleStar && (
              <button
                type="button"
                className={`sd-star-btn ${card.starred ? "starred" : ""}`}
                onClick={() => onToggleStar(card)}
                title={card.starred ? "Bỏ đánh dấu" : "Đánh dấu"}
              >
                {card.starred ? "★" : "☆"}
              </button>
            )}
          </div>
        </div>

        {expanded && (
          <div className="sd-fc-row-expanded">
            {card.imageUrl && (
              <img
                src={card.imageUrl}
                alt={card.term}
                className="sd-fc-row-expanded-img"
              />
            )}
            {card.exampleSentence && (
              <div className="sd-fc-extra-block">
                <div className="sd-fc-extra-label">Example</div>
                <div className="sd-fc-extra-text">{card.exampleSentence}</div>
              </div>
            )}
            {card.hintExplanation && (
              <div className="sd-fc-extra-block">
                <div className="sd-fc-extra-label">Hint</div>
                <div className="sd-fc-extra-text">{card.hintExplanation}</div>
              </div>
            )}
          </div>
        )}
      </article>

      {hasExtra && (
        <button
          type="button"
          className="sd-fc-expand-btn"
          onClick={() => setExpanded((v) => !v)}
          aria-expanded={expanded}
          title={expanded ? "Thu gọn" : "Xem thêm"}
        >
          <svg
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.5"
            className={expanded ? "sd-fc-chevron--up" : ""}
          >
            <path d="M6 9l6 6 6-6" />
          </svg>
        </button>
      )}
    </div>
  );
}
