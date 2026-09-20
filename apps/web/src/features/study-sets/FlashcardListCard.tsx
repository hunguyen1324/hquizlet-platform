import React from "react";
import type { Flashcard } from "../../types";

type Props = {
  card: Flashcard;
  onToggleStar?: (card: Flashcard) => void;
  onEdit?: (card: Flashcard) => void;
  onDelete?: (card: Flashcard) => void;
};

export function FlashcardListCard({ card, onToggleStar, onEdit, onDelete }: Props) {
  const [expanded, setExpanded] = React.useState(false);
  const hasExtra = Boolean(
    card.imageUrl || card.exampleSentence || card.hintExplanation,
  );

  return (
    <div className="sd-fc-row-wrap">
      <article className="sd-fc-row ql-card-row">
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
            {onEdit && (
              <button
                type="button"
                className="sd-edit-btn"
                onClick={() => onEdit(card)}
                title="Sửa thẻ"
                aria-label={`Sửa thẻ "${card.term}"`}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                </svg>
              </button>
            )}
            {onDelete && (
              <button
                type="button"
                className="sd-delete-btn"
                onClick={() => onDelete(card)}
                title="Xóa thẻ"
                aria-label={`Xóa thẻ "${card.term}"`}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="3 6 5 6 21 6"/>
                  <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>
                  <path d="M10 11v6M14 11v6"/>
                  <path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/>
                </svg>
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
