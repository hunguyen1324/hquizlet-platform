import React from "react";
import type { LearningMode } from "../learning/types";

export type StudyModeNav = LearningMode;

type ModeItem = {
  mode: StudyModeNav;
  label: string;
  Icon: React.FC<{ className?: string }>;
};

/** Lucide-style icons (same set as quizlet-clone study-modes) */
function IconFlashcards({ className }: { className?: string }) {
  return (
    <svg className={className} width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <rect width="14" height="14" x="8" y="8" rx="2" ry="2" />
      <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
    </svg>
  );
}

function IconLearn({ className }: { className?: string }) {
  return (
    <svg className={className} width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M21.42 10.922a1 1 0 0 0-.019-1.838L12.83 5.18a2 2 0 0 0-1.66 0L2.6 9.08a1 1 0 0 0 0 1.832l8.57 3.908a2 2 0 0 0 1.66 0z" />
      <path d="M22 10v6" />
      <path d="M6 12.5V16a6 3 0 0 0 12 0v-3.5" />
    </svg>
  );
}

function IconTest({ className }: { className?: string }) {
  return (
    <svg className={className} width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M15.5 3H5a2 2 0 0 0-2 2v14c0 1.1.9 2 2 2h14a2 2 0 0 0 2-2V8.5L15.5 3Z" />
      <path d="M15 3v6h6" />
      <path d="m10 16 2 2 4-4" />
    </svg>
  );
}

function IconMatch({ className }: { className?: string }) {
  return (
    <svg className={className} width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M15.39 4.39a1 1 0 0 0 1.68-.474 2.85 2.85 0 1 1 3.013 3.013 1 1 0 0 0-.474 1.68l1.683 1.682a2.414 2.414 0 0 1 0 3.414L19.61 15.39a1 1 0 0 1-1.68-.474 2.85 2.85 0 1 0-3.013 3.013 1 1 0 0 1 .474 1.68l-1.683 1.682a2.414 2.414 0 0 1-3.414 0L8.61 19.61a1 1 0 0 0-1.68.474 2.85 2.85 0 1 1-3.013-3.013 1 1 0 0 0 .474-1.68l-1.683-1.682a2.414 2.414 0 0 1 0-3.414L4.39 8.61a1 1 0 0 1 1.68.474 2.85 2.85 0 1 0 3.013-3.013 1 1 0 0 1-.474-1.68l1.683-1.682a2.414 2.414 0 0 1 3.414 0z" />
    </svg>
  );
}

const MODES: ModeItem[] = [
  { mode: "flashcards", label: "Thẻ ghi nhớ", Icon: IconFlashcards },
  { mode: "learn", label: "Học", Icon: IconLearn },
  { mode: "test", label: "Kiểm tra", Icon: IconTest },
  { mode: "match", label: "Ghép thẻ", Icon: IconMatch },
];

type Props = {
  activeMode: StudyModeNav;
  onSelectMode: (mode: StudyModeNav) => void;
};

export function StudyModes({ activeMode, onSelectMode }: Props) {
  return (
    <div className="sd-study-modes">
      <div className="sd-study-modes-grid" role="navigation" aria-label="Chế độ học">
        {MODES.map(({ mode, label, Icon }) => {
          const isActive = activeMode === mode;
          return (
            <button
              key={mode}
              type="button"
              className={`sd-study-mode-card${isActive ? " sd-study-mode-card--active" : ""}`}
              aria-current={isActive ? "page" : undefined}
              onClick={() => onSelectMode(mode)}
            >
              <Icon className="sd-study-mode-icon" />
              <span className="sd-study-mode-label">{label}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
