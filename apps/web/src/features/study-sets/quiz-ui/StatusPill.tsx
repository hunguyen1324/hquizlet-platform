/** Save state of the answer currently being written to the server. */
export type SaveState = "idle" | "saving" | "saved" | "error";

const LABELS: Record<Exclude<SaveState, "idle">, string> = {
  saving: "Đang lưu…",
  saved: "Đã lưu",
  error: "Chưa lưu được",
};

/** Small, always-visible indicator so a tap never feels unrecorded. */
export function StatusPill({ state, onRetry }: { state: SaveState; onRetry?: () => void }) {
  if (state === "idle") return null;
  return (
    <span className={`qz-pill qz-pill--${state}`} role="status" aria-live="polite">
      <span aria-hidden="true">{state === "saving" ? "◌" : state === "saved" ? "✓" : "!"}</span>
      <span>{LABELS[state]}</span>
      {state === "error" && onRetry ? (
        <button type="button" className="qz-pill-retry" onClick={onRetry}>
          Thử lại
        </button>
      ) : null}
    </span>
  );
}
