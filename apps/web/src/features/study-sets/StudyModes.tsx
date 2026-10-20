import React from "react";
import type { LearningMode, LearningProgress } from "../../lib/api/client";

export type StudyModeNav = LearningMode;

/** Minimal slice of a saved session used to show "how far you got" per mode. */
export type ModeProgress = Pick<LearningProgress, "score" | "total" | "completedAt">;

type ModeItem = {
  mode: StudyModeNav;
  label: string;
  hint: string;
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
  { mode: "flashcards", label: "Thẻ ghi nhớ", hint: "Lật thẻ và tự đánh giá", Icon: IconFlashcards },
  { mode: "learn", label: "Học", hint: "Ôn lại thẻ hay sai trước", Icon: IconLearn },
  { mode: "test", label: "Kiểm tra", hint: "Bài kiểm tra có chấm điểm", Icon: IconTest },
  { mode: "match", label: "Ghép thẻ", hint: "Ghép thuật ngữ với định nghĩa", Icon: IconMatch },
];

/** Percent of correct answers in the last saved session, or null when unknown. */
export function modePercent(progress?: ModeProgress): number | null {
  if (!progress || !progress.total || progress.total <= 0) return null;
  const percent = Math.round((progress.score / progress.total) * 100);
  return Math.min(100, Math.max(0, percent));
}

type Props = {
  activeMode: StudyModeNav;
  onSelectMode: (mode: StudyModeNav) => void;
  /** Last saved session per mode (from GET /progress/latest); missing modes are unseen. */
  progress?: Partial<Record<StudyModeNav, ModeProgress>>;
  /** Single primary action: jump back into the most recently used mode. */
  onContinue?: () => void;
};

export function StudyModes({ activeMode, onSelectMode, progress, onContinue }: Props) {
  const started = MODES.some(({ mode }) => modePercent(progress?.[mode]) !== null);

  return (
    <section className="sd-mode-hub" aria-label="Chế độ học">
      <div className="sd-mode-hub-head">
        <h2 className="sd-mode-hub-title">Học bộ này</h2>
        {onContinue && (
          <button type="button" className="primary-button sd-mode-hub-cta" onClick={onContinue}>
            {started ? "Tiếp tục học" : "Bắt đầu học"}
          </button>
        )}
      </div>

      <div className="sd-mode-grid" role="navigation" aria-label="Chọn chế độ học">
        {MODES.map(({ mode, label, hint, Icon }) => {
          const isActive = activeMode === mode;
          const percent = modePercent(progress?.[mode]);
          const detail = progress?.[mode];
          return (
            <button
              key={mode}
              type="button"
              className={`sd-mode-card${isActive ? " sd-mode-card--active" : ""}`}
              aria-current={isActive ? "page" : undefined}
              aria-label={percent !== null ? `${label}, lần trước đúng ${percent}%` : `${label}, chưa học lần nào`}
              onClick={() => onSelectMode(mode)}
            >
              <span className="sd-mode-card-icon"><Icon /></span>
              <span className="sd-mode-card-body">
                <span className="sd-mode-card-label">{label}</span>
                <span className="sd-mode-card-hint">{hint}</span>
                {percent !== null ? (
                  <>
                    <span className="sd-mode-card-meter" aria-hidden>
                      <span className="sd-mode-card-meter-fill" style={{ width: `${percent}%` }} />
                    </span>
                    <span className="sd-mode-card-meta">
                      Đúng {percent}% · {detail?.score}/{detail?.total} câu
                    </span>
                  </>
                ) : (
                  <span className="sd-mode-card-meta sd-mode-card-meta--muted">Chưa học</span>
                )}
              </span>
            </button>
          );
        })}
      </div>
    </section>
  );
}
