// apps/web/src/features/live/JoinLiveSession.tsx
// Player join flow: mã phòng → tên → vào phòng.
// One column, big targets, works on a phone held in one hand.

import React, { useState } from "react";

const DISPLAY_NAME_KEY = "hq.live.displayName";
const JOIN_CODE_LENGTH = 6;

interface JoinLiveSessionProps {
  initialCode?: string;
  onJoin: (code: string, displayName: string) => void;
  error?: string | null;
  loading: boolean;
}

export function sanitizeJoinCode(raw: string): string {
  return raw.toUpperCase().replace(/[^A-Z0-9]/g, "").slice(0, JOIN_CODE_LENGTH);
}

function rememberDisplayName(name: string) {
  try {
    window.localStorage.setItem(DISPLAY_NAME_KEY, name);
  } catch {
    // Private mode / storage disabled: remembering the name is a nicety only.
  }
}

function recalledDisplayName(): string {
  try {
    return window.localStorage.getItem(DISPLAY_NAME_KEY) ?? "";
  } catch {
    return "";
  }
}

/**
 * Turns a raw join failure into something a student can act on.
 * The gateway/quiz error message is technical, so the copy says what to try next.
 */
export function describeJoinError(input: unknown): string {
  const status = (input as { status?: number } | null)?.status;
  const code = String((input as { code?: string } | null)?.code ?? "").toUpperCase();
  const raw = input instanceof Error ? input.message : typeof input === "string" ? input : "";
  const haystack = `${code} ${raw}`.toUpperCase();

  if (status === 429 || haystack.includes("RATE_LIMITED")) {
    return "Bạn thử vào phòng quá nhanh. Chờ vài giây rồi bấm “Vào phòng” lại nhé.";
  }
  if (haystack.includes("LIVE_SESSION_NOT_FOUND") || status === 404) {
    return "Không tìm thấy phòng với mã này. Kiểm tra lại 6 ký tự trên bảng của giáo viên.";
  }
  if (haystack.includes("LIVE_SESSION_ENDED") || status === 410) {
    return "Phòng này đã kết thúc. Xin giáo viên mã của buổi mới.";
  }
  if (haystack.includes("LIVE_INVALID_STATE") || status === 409) {
    return "Buổi học đã bắt đầu nên không vào được nữa. Đề nghị giáo viên mở lại phòng chờ.";
  }
  if (haystack.includes("DISPLAY_NAME_TAKEN")) {
    return "Tên này đã có người dùng trong phòng. Chọn thêm một chữ để phân biệt nhé.";
  }
  if (haystack.includes("VALIDATION_ERROR") || status === 422) {
    return "Mã phòng hoặc tên chưa hợp lệ. Mã gồm 6 ký tự, tên từ 1 đến 40 ký tự.";
  }
  if (status !== undefined && status >= 500) {
    return "Máy chủ đang bận. Thử lại sau ít giây.";
  }
  if (raw) {
    return raw;
  }
  return "Không vào được phòng. Kiểm tra kết nối mạng rồi thử lại.";
}

export function JoinLiveSession({ initialCode, onJoin, error, loading }: JoinLiveSessionProps) {
  const [code, setCode] = useState(sanitizeJoinCode(initialCode ?? ""));
  const [displayName, setDisplayName] = useState(recalledDisplayName);

  const codeComplete = code.length === JOIN_CODE_LENGTH;
  const nameValid = displayName.trim().length > 0;
  const canSubmit = codeComplete && nameValid && !loading;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    const name = displayName.trim();
    rememberDisplayName(name);
    onJoin(code, name);
  }

  return (
    <section className="live-join" aria-labelledby="live-join-title">
      <header className="live-join-head">
        <p className="live-join-eyebrow">Lớp học trực tiếp</p>
        <h1 id="live-join-title" className="live-join-title">Vào phòng thi</h1>
        <p className="live-join-sub">Nhập mã 6 ký tự giáo viên đưa cho bạn, rồi chọn tên hiện trên bảng điểm.</p>
      </header>

      {error && (
        <div className="live-join-error" role="alert">
          <span aria-hidden>⚠️</span>
          <p>{error}</p>
        </div>
      )}

      <form className="live-join-form" onSubmit={handleSubmit} noValidate>
        <div className="live-join-field">
          <label htmlFor="live-join-code" className="live-join-label">Mã phòng</label>
          <input
            id="live-join-code"
            type="text"
            inputMode="text"
            autoComplete="off"
            autoCorrect="off"
            spellCheck={false}
            enterKeyHint="next"
            maxLength={JOIN_CODE_LENGTH}
            value={code}
            onChange={(e) => setCode(sanitizeJoinCode(e.target.value))}
            placeholder="ABC123"
            className="live-join-code-input"
            aria-invalid={Boolean(error) || undefined}
            aria-describedby="live-join-code-hint"
            autoFocus
          />
          <p id="live-join-code-hint" className="live-join-hint">
            Mã gồm 6 chữ cái hoặc số, không phân biệt hoa thường.
          </p>
        </div>

        <div className="live-join-field">
          <label htmlFor="live-join-name" className="live-join-label">Tên của bạn</label>
          <input
            id="live-join-name"
            type="text"
            autoComplete="nickname"
            enterKeyHint="go"
            maxLength={40}
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Ví dụ: Minh Anh"
            className="live-join-name-input"
          />
          <p className="live-join-hint">Tên này hiện trên bảng điểm của lớp, tối đa 40 ký tự.</p>
        </div>

        <button type="submit" className="live-join-submit" disabled={!canSubmit}>
          {loading ? "Đang vào phòng…" : "Vào phòng"}
        </button>
      </form>
    </section>
  );
}
