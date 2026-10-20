import type { QuizQuestion } from "../../../types";

const norm = (v = "") => v.trim().toLocaleLowerCase();

/** Text of the correct option once the server has revealed it. */
export function correctText(q: QuizQuestion): string {
  const marked = q.options?.find((o) => o.isCorrect)?.text;
  if (marked) return marked;
  const raw = q.correctAnswer ?? "";
  const letter = /^[a-d]$/i.test(raw.trim()) ? raw.trim().toUpperCase().charCodeAt(0) - 65 : -1;
  return letter >= 0 ? (q.options?.[letter]?.text ?? raw) : raw;
}

export function isCorrectOption(q: QuizQuestion, text: string): boolean {
  const marked = q.options?.find((o) => o.isCorrect);
  if (marked) return marked.text === text;
  return norm(correctText(q)) === norm(text);
}

export function answerLabel(q: QuizQuestion, value: string): string {
  if (q.questionType === "true_false") return value === "true" ? "Đúng" : value === "false" ? "Sai" : value;
  return value;
}

export function formatClock(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), r = s % 60;
  const mm = String(m).padStart(2, "0"), ss = String(r).padStart(2, "0");
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}

export function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60), s = seconds % 60;
  return m > 0 ? `${m} phút ${s} giây` : `${s} giây`;
}

export function timeAgo(iso: string, now = Date.now()): string {
  const diff = Math.max(0, now - Date.parse(iso));
  const min = Math.floor(diff / 60000);
  if (min < 1) return "vừa xong";
  if (min < 60) return `${min} phút trước`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h} giờ trước`;
  return `${Math.floor(h / 24)} ngày trước`;
}

/** True when the key event came from a text field, where shortcuts must not fire. */
export function isTypingTarget(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el || !el.tagName) return false;
  return el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT" || el.isContentEditable;
}
