import React from "react";

import { useAuth } from "../auth/AuthContext";
import { mergeQuizView, protectedQuizApi, type ProtectedQuizPeek, type ProtectedQuizView, type QuizStart } from "../../lib/api/protectedQuiz";
import { ApiError } from "../../lib/api/client";
import { ExamQuestionLayout, CustomAudioPlayer } from "./exam/ExamQuestionLayout";
import { RichTextContent } from "./exam/RichTextContent";
import { ExamTimer } from "./exam/ExamTimer";
import { PART_LABELS } from "./exam/toeic";
import {
  AnswerOption,
  FeedbackSheet,
  QuestionGrid,
  QuizBottomBar,
  QuizPageSkeleton,
  QuizShell,
  QuizTopBar,
  type OptionState,
  type SaveState,
} from "./quiz-ui";
import "./quiz-ui/quiz-ui.css";
import type { ExamItem } from "./exam/types";
import type { QuizQuestion } from "../../types";

type PendingAnswer = { index: number; value: string };

function letterFor(index: number): string {
  return String.fromCharCode(65 + index);
}

/** Cache key for one delivered page: a single question, or a TOEIC group. */
function pageKeyFor(view: ProtectedQuizView, index: number): string {
  if (view.layout !== "toeic") return `i${index}`;
  return `g${view.manifest?.[index]?.group ?? index}`;
}

function AnswerInput({
  question,
  selected,
  pending,
  correctText,
  revealed,
  disabled,
  onAnswer,
}: {
  question: QuizQuestion;
  selected?: string;
  pending?: PendingAnswer;
  correctText?: string;
  revealed: boolean;
  disabled: boolean;
  onAnswer: (answer: string) => void;
}) {
  const [draft, setDraft] = React.useState(selected ?? "");
  const [order, setOrder] = React.useState(() => (question.options ?? []).map((o) => o.text));

  if (question.questionType === "sorting") {
    const move = (from: number, to: number) =>
      setOrder((old) => {
        const next = [...old];
        const [item] = next.splice(from, 1);
        next.splice(to, 0, item!);
        return next;
      });
    return (
      <div className="qz-answers">
        {order.map((text, i) => (
          <div key={i} className="qz-answer qz-answer--idle">
            <span className="qz-answer-letter" aria-hidden="true">
              {i + 1}
            </span>
            <span className="qz-answer-text">{text}</span>
            <span className="qz-answer-state">
              <button type="button" className="qz-chip" disabled={disabled || i === 0} aria-label={`Đưa "${text}" lên`} onClick={() => move(i, i - 1)}>
                ↑
              </button>
              <button
                type="button"
                className="qz-chip"
                disabled={disabled || i === order.length - 1}
                aria-label={`Đưa "${text}" xuống`}
                onClick={() => move(i, i + 1)}
              >
                ↓
              </button>
            </span>
          </div>
        ))}
        <button type="button" className="qz-btn qz-btn--primary" disabled={disabled || !!pending} onClick={() => onAnswer(order.join(" → "))}>
          Lưu thứ tự
        </button>
      </div>
    );
  }

  if (question.questionType === "written") {
    return (
      <form
        className="qz-written"
        onSubmit={(event) => {
          event.preventDefault();
          if (draft.trim()) onAnswer(draft);
        }}
      >
        <input
          aria-label="Câu trả lời"
          value={draft}
          placeholder="Nhập câu trả lời"
          disabled={disabled || !!pending}
          onChange={(event) => setDraft(event.target.value)}
        />
        <button type="submit" className="qz-btn qz-btn--primary" disabled={disabled || !!pending || !draft.trim()}>
          {pending ? "Đang chấm…" : "Lưu đáp án"}
        </button>
      </form>
    );
  }

  const options =
    question.questionType === "true_false"
      ? [
          { text: "true", label: "Đúng", isCorrect: false },
          { text: "false", label: "Sai", isCorrect: false },
        ]
      : (question.options ?? []).map((option) => ({ text: option.text, label: option.text, isCorrect: option.isCorrect === true }));
  const shown = pending?.value ?? selected;

  return (
    <div className="qz-answers" role="group" aria-label="Các đáp án">
      {options.map((option, i) => {
        const isSelected = shown === option.text;
        let state: OptionState = "idle";
        if (pending?.value === option.text) {
          state = "pending";
        } else if (revealed) {
          const isCorrectOption = option.isCorrect || (correctText !== undefined && option.text === correctText);
          if (isCorrectOption) state = isSelected ? "correct" : "passed";
          else if (isSelected) state = "wrong";
        } else if (isSelected) {
          state = "selected";
        }
        return (
          <AnswerOption
            key={i}
            letter={letterFor(i)}
            text={option.label}
            state={state}
            disabled={disabled || revealed || !!pending}
            onSelect={() => onAnswer(option.text)}
          />
        );
      })}
    </div>
  );
}

export function ProtectedQuizPlayer({
  studySetId,
  onManageSessions,
  resumeSessionId,
}: {
  studySetId: number;
  resumeSessionId?: string;
  onManageSessions?: (studySetId: number) => void;
}) {
  const { token, user } = useAuth();
  const storageKey = `hquizlet:protected-quiz:${user?.id}:${studySetId}`;
  const [summary, setSummary] = React.useState<{
    total: number;
    parts: Record<number, number>;
    sessions: string[];
    activeCount: number;
    limit: number;
    studySetId: number;
  } | null>(null);
  const [sessionLimitError, setSessionLimitError] = React.useState(false);
  const [view, setView] = React.useState<ProtectedQuizView | null>(null);
  const [savedId, setSavedId] = React.useState<string | null>(() => localStorage.getItem(storageKey));
  const [layout, setLayout] = React.useState<QuizStart["layout"]>("default");
  const [part, setPart] = React.useState(0);
  const [instant, setInstant] = React.useState(true);
  const [minutes, setMinutes] = React.useState(120);
  const [timedPractice, setTimedPractice] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const busyRef = React.useRef(false);
  const [error, setError] = React.useState("");
  const [timeLeft, setTimeLeft] = React.useState(0);
  const [navigator, setNavigator] = React.useState(false);
  const [flags, setFlags] = React.useState<boolean[]>([]);
  const [confirmSubmit, setConfirmSubmit] = React.useState(false);
  const [audio, setAudio] = React.useState<{ key: string; url: string } | null>(null);
  const mediaCache = React.useRef(new Map<string, { url: string; bytes: number }>());
  const [audioError, setAudioError] = React.useState("");
  const [audioRetry, setAudioRetry] = React.useState(0);
  const [muted, setMuted] = React.useState(false);
  const expirySubmitted = React.useRef<string | null>(null);
  const [clockOffset, setClockOffset] = React.useState(0);

  // Phase 1 UI: optimistic answer, save indicator and prefetched pages.
  const [pending, setPending] = React.useState<PendingAnswer | null>(null);
  const [saveState, setSaveState] = React.useState<SaveState>("idle");
  const [failedAnswer, setFailedAnswer] = React.useState<PendingAnswer | null>(null);
  const [loadingPage, setLoadingPage] = React.useState(false);
  // A cached page is painted before the server confirms it, and only the server
  // marks a page as delivered. Audio rights follow that confirmation.
  const [pageConfirmed, setPageConfirmed] = React.useState(true);
  const [sheetDismissed, setSheetDismissed] = React.useState<number | null>(null);
  const peekCache = React.useRef(new Map<string, ProtectedQuizPeek>());
  const peekInFlight = React.useRef(new Set<string>());
  const [, bumpPeek] = React.useState(0);

  React.useEffect(() => {
    let active = true;
    // Legacy sessions could contain answers; they are no longer usable.
    localStorage.removeItem(`hquizlet:quiz-session:${studySetId}`);
    protectedQuizApi
      .summary(token, studySetId)
      .then((data) => {
        if (active) setSummary(data);
      })
      .catch((e: Error) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [token, studySetId]);

  const applyView = React.useCallback(
    (next: ProtectedQuizView) => {
      setClockOffset(Date.parse(next.serverTime) - Date.now());
      setView((prev) => mergeQuizView(prev, next));
      setPageConfirmed(true);
      setLayout(next.layout);
      setInstant(next.instant);
      localStorage.setItem(storageKey, next.id);
      setSavedId(next.id);
    },
    [storageKey],
  );

  async function run(request: () => Promise<ProtectedQuizView>) {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    setError("");
    setSessionLimitError(false);
    try {
      applyView(await request());
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setSessionLimitError(true);
        return;
      }
      setError(e instanceof Error ? e.message : "Không thực hiện được yêu cầu.");
      if (e instanceof ApiError && [401, 403, 404].includes(e.status)) {
        setView(null);
        setAudio(null);
        localStorage.removeItem(storageKey);
        setSavedId(null);
      }
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }

  React.useEffect(() => {
    if (resumeSessionId) void run(() => protectedQuizApi.page(token, studySetId, resumeSessionId, -1, { manifest: true }));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, studySetId, resumeSessionId]);

  function start(mode: QuizStart["mode"]) {
    setFlags([]);
    expirySubmitted.current = null;
    peekCache.current.clear();
    peekInFlight.current.clear();
    setSaveState("idle");
    setPending(null);
    void run(() =>
      protectedQuizApi.start(token, studySetId, {
        mode,
        layout,
        part: layout === "toeic" ? part : 0,
        instant: mode === "practice" ? instant : false,
        durationSeconds: mode === "exam" || timedPractice ? minutes * 60 : 0,
      }),
    );
  }

  function navigate(index: number) {
    if (!view || busyRef.current) return;
    if (index < 0 || index >= (view.manifest?.length ?? view.total)) return;
    setNavigator(false);
    setSheetDismissed(null);
    setPending(null);
    setError("");
    const cached = peekCache.current.get(pageKeyFor(view, index));
    if (cached?.page?.length) {
      // Prefetched: paint the page immediately, confirm with the server behind it.
      setView({ ...view, currentIndex: index, page: cached.page });
      setPageConfirmed(false);
      setLoadingPage(false);
    } else {
      setPageConfirmed(false);
      setLoadingPage(true);
    }
    void run(() => protectedQuizApi.page(token, studySetId, view.id, index)).finally(() => setLoadingPage(false));
  }

  function submit() {
    if (view) {
      setConfirmSubmit(false);
      void run(() => protectedQuizApi.submit(token, studySetId, view.id));
    }
  }

  async function answer(index: number, value: string) {
    if (!view || busyRef.current || view.submitted) return;
    if (view.instant && view.answers[index] !== undefined) return;
    // Paint the choice now: the learner never stares at an unchanged screen.
    setPending({ index, value });
    setSaveState("saving");
    setFailedAnswer(null);
    setError("");
    busyRef.current = true;
    setBusy(true);
    try {
      applyView(await protectedQuizApi.answer(token, studySetId, view.id, index, value));
      setSaveState("saved");
    } catch (e) {
      setSaveState("error");
      setFailedAnswer({ index, value });
      setError(e instanceof Error ? e.message : "Chưa lưu được đáp án.");
    } finally {
      busyRef.current = false;
      setBusy(false);
      setPending(null);
    }
  }

  React.useEffect(() => {
    if (!view?.deadline || view.submitted) return;
    const tick = () => {
      const left = Math.max(0, Math.ceil((Date.parse(view.deadline!) - Date.now() - clockOffset) / 1000));
      setTimeLeft(left);
      if (left === 0 && !busyRef.current && expirySubmitted.current !== view.id) {
        expirySubmitted.current = view.id;
        void run(() => protectedQuizApi.submit(token, studySetId, view.id));
      }
    };
    tick();
    const timer = window.setInterval(tick, 500);
    return () => window.clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view?.id, view?.deadline, view?.submitted, clockOffset, token, studySetId]);

  // Prefetch 3 pages forward and 1 back, sequentially and off the critical path.
  React.useEffect(() => {
    if (!view || view.submitted) return;
    const { id, currentIndex } = view;
    const total = view.manifest?.length ?? view.total;
    const targets = [1, 2, 3, -1]
      .map((delta) => currentIndex + delta)
      .filter((target) => target >= 0 && target < total);
    let cancelled = false;
    const timer = window.setTimeout(() => {
      void (async () => {
        for (const target of targets) {
          if (cancelled) return;
          const key = pageKeyFor(view, target);
          if (peekCache.current.has(key) || peekInFlight.current.has(key)) continue;
          peekInFlight.current.add(key);
          try {
            const peek = await protectedQuizApi.peek(token, studySetId, id, target);
            if (cancelled) return;
            peekCache.current.set(key, peek);
            while (peekCache.current.size > 12) {
              const oldest = peekCache.current.keys().next().value;
              if (oldest === undefined) break;
              peekCache.current.delete(oldest);
            }
            bumpPeek((n) => n + 1);
          } catch {
            // Prefetch is best effort; the real navigation still works.
          } finally {
            peekInFlight.current.delete(key);
          }
        }
      })();
    }, 60);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view?.id, view?.currentIndex, view?.submitted, view?.total, view?.manifest, token, studySetId]);

  const audioItem = view?.page.find((item) => item.hasAudio);
  const audioKey = view && audioItem ? `${view.id}:${view.manifest?.[audioItem.index]?.group}:${audioItem.index}` : "";
  React.useEffect(() => {
    setAudio(null);
    setAudioError("");
    // Never ask for audio for a page the server has not marked as delivered.
    if (!view || !audioItem || !pageConfirmed) return;
    const controller = new AbortController();
    protectedQuizApi
      .audio(token, studySetId, view.id, audioItem.index, controller.signal)
      .then(async (url) => {
        const cached = mediaCache.current.get(audioKey);
        if (cached) {
          if (!controller.signal.aborted) setAudio({ key: audioKey, url: cached.url });
          return;
        }
        // Download once, then reuse the bounded in-memory copy.
        const response = await fetch(url, { signal: controller.signal, cache: "no-store" });
        if (!response.ok) throw new Error("Không tải được âm thanh.");
        const blob = await response.blob();
        if (controller.signal.aborted) return;
        if (blob.size > 30 * 1024 * 1024) {
          setAudio({ key: audioKey, url });
          return;
        }
        let bytes = [...mediaCache.current.values()].reduce((sum, entry) => sum + entry.bytes, 0);
        while (mediaCache.current.size && (bytes + blob.size > 40 * 1024 * 1024 || mediaCache.current.size >= 8)) {
          const key = mediaCache.current.keys().next().value!;
          const entry = mediaCache.current.get(key)!;
          bytes -= entry.bytes;
          URL.revokeObjectURL(entry.url);
          mediaCache.current.delete(key);
        }
        const cachedUrl = URL.createObjectURL(blob);
        mediaCache.current.set(audioKey, { url: cachedUrl, bytes: blob.size });
        setAudio({ key: audioKey, url: cachedUrl });
      })
      .catch((e: Error) => {
        if (!controller.signal.aborted) setAudioError(e.message);
      });
    return () => {
      controller.abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [audioKey, audioRetry, token, studySetId, pageConfirmed]);
  React.useEffect(
    () => () => {
      for (const entry of mediaCache.current.values()) URL.revokeObjectURL(entry.url);
      mediaCache.current.clear();
    },
    [token, studySetId],
  );
  const audioUrl = audio?.key === audioKey ? audio.url : undefined;

  if (!view) {
    const parts = Object.entries(summary?.parts ?? {});
    return (
      <section className="qz-shell qz-start">
        <header className="qz-start-head">
          <p className="qz-eyebrow">Quiz</p>
          <h2>Chọn cách làm</h2>
          <p className="qz-muted">{summary ? `${summary.total} câu hỏi` : "Đang tải thông tin bài…"}</p>
          {error && (
            <p role="alert" className="qz-error">
              {error}
            </p>
          )}
          {sessionLimitError && (
            <p role="alert" className="qz-error">
              Đã đạt giới hạn {summary?.limit ?? 5} phiên chưa nộp cho bài này. Hãy nộp hoặc xoá một phiên cũ.
            </p>
          )}
        </header>

        {savedId && (
          <button type="button" className="qz-resume" disabled={busy} onClick={() => void run(() => protectedQuizApi.page(token, studySetId, savedId, -1, { manifest: true }))}>
            <strong>Làm tiếp phiên gần nhất</strong>
            <span>Tiến độ được lưu trong 24 giờ</span>
          </button>
        )}

        <div className="qz-modes">
          <button type="button" className="qz-mode" disabled={busy || !summary?.total} onClick={() => start("practice")}>
            <strong>Luyện tập</strong>
            <small>Xem đáp án ngay sau mỗi câu</small>
          </button>
          <button type="button" className="qz-mode" disabled={busy || !summary?.total} onClick={() => start("exam")}>
            <strong>Thi thử</strong>
            <small>Tính giờ như thật, nộp bài mới biết điểm</small>
          </button>
        </div>

        <fieldset className="qz-options">
          <legend>Tuỳ chọn</legend>
          <div className="qz-field">
            <span className="qz-label">Cách hiển thị</span>
            <div className="qz-filters" role="group" aria-label="Cách hiển thị câu hỏi">
              <button type="button" className={`qz-chip${layout === "default" ? " qz-chip--on" : ""}`} aria-pressed={layout === "default"} onClick={() => setLayout("default")}>
                Từng câu
              </button>
              <button type="button" className={`qz-chip${layout === "toeic" ? " qz-chip--on" : ""}`} aria-pressed={layout === "toeic"} onClick={() => setLayout("toeic")}>
                Theo nhóm (TOEIC)
              </button>
            </div>
          </div>
          {layout === "toeic" && parts.length > 0 && (
            <div className="qz-field">
              <span className="qz-label">Phạm vi</span>
              <div className="qz-filters" role="group" aria-label="Chọn Part">
                <button type="button" className={`qz-chip${part === 0 ? " qz-chip--on" : ""}`} aria-pressed={part === 0} onClick={() => setPart(0)}>
                  Tất cả
                </button>
                {parts.map(([p, count]) => (
                  <button
                    key={p}
                    type="button"
                    className={`qz-chip${part === Number(p) ? " qz-chip--on" : ""}`}
                    aria-pressed={part === Number(p)}
                    onClick={() => setPart(Number(p))}
                  >
                    {PART_LABELS[Number(p)] ?? `Part ${p}`} · {count}
                  </button>
                ))}
              </div>
            </div>
          )}
          <div className="qz-field">
            <span className="qz-label">Thời gian</span>
            <div className="qz-filters" role="group" aria-label="Thời gian làm bài">
              {[30, 60, 120].map((value) => (
                <button key={value} type="button" className={`qz-chip${minutes === value ? " qz-chip--on" : ""}`} aria-pressed={minutes === value} onClick={() => setMinutes(value)}>
                  {value}′
                </button>
              ))}
              <input
                type="number"
                min={1}
                max={600}
                value={minutes}
                aria-label="Số phút tuỳ chỉnh"
                className="qz-input-sm"
                onChange={(event) => setMinutes(Math.max(1, Math.min(600, Number(event.target.value) || 1)))}
              />
            </div>
          </div>
          <label className="qz-check">
            <input type="checkbox" checked={instant} onChange={(event) => setInstant(event.target.checked)} /> Xem đáp án ngay khi luyện tập
          </label>
          <label className="qz-check">
            <input type="checkbox" checked={timedPractice} onChange={(event) => setTimedPractice(event.target.checked)} /> Đếm ngược cả khi luyện tập
          </label>
          <p className="qz-muted">
            Tiến độ được lưu 24 giờ, tối đa {summary?.limit ?? 5} phiên chưa nộp trên mỗi bài.
            {onManageSessions && (
              <>
                {" "}
                <button type="button" className="qz-link" onClick={() => onManageSessions(studySetId)}>
                  Quản lý phiên
                </button>
              </>
            )}
          </p>
        </fieldset>

        {(summary?.sessions ?? []).filter((id) => id !== savedId).length > 0 && (
          <div className="qz-field">
            <span className="qz-label">Phiên trước</span>
            <div className="qz-filters">
              {(summary?.sessions ?? [])
                .filter((id) => id !== savedId)
                .map((id, i) => (
                  <button key={id} type="button" className="qz-chip" disabled={busy} onClick={() => void run(() => protectedQuizApi.page(token, studySetId, id, -1, { manifest: true }))}>
                    Mở phiên #{i + 1}
                  </button>
                ))}
            </div>
          </div>
        )}
      </section>
    );
  }

  const manifest: ExamItem[] =
    (view.manifest ?? []).map((item, i) => ({
      key: String(i),
      groupKey: String(item.group),
      questionNumber: item.number,
      part: item.part,
      question: { questionType: "multiple_choice", questionText: "" },
    })) ?? [];
  const answers = manifest.map((_, i) => view.answers[i] ?? null);
  const group: ExamItem[] = view.page.map((item) => ({
    ...manifest[item.index]!,
    question: { ...item.question, subQuestions: undefined, audioUrl: item.index === audioItem?.index ? audioUrl : undefined },
    parent: item.question.paragraphText
      ? { questionType: "paragraph", questionText: "", paragraphText: item.question.paragraphText, audioUrl }
      : undefined,
  }));
  const answeredCount = view.answered ?? Object.keys(view.answers).length;
  const position = view.manifest?.[view.currentIndex]?.number ?? 1;
  const audioNotice = audioError ? (
    <div role="alert" className="qz-error">
      {audioError}{" "}
      <button type="button" className="qz-link" onClick={() => setAudioRetry((n) => n + 1)}>
        Thử lại
      </button>
    </div>
  ) : audioItem && !audioUrl ? (
    <p className="qz-hint" role="status">
      Đang tải âm thanh…
    </p>
  ) : null;

  const pageItem = view.page[0];
  const pageRevealed = !!pageItem?.revealed;
  const correctText =
    (pageItem?.question.options?.find((option) => option.isCorrect)?.text ?? pageItem?.question.correctAnswer) || undefined;
  const feedbackOpen =
    view.layout === "default" && view.instant && !view.submitted && pageRevealed && !pending && sheetDismissed !== pageItem?.index;
  const timeWarning = !view.deadline || view.submitted ? "" : timeLeft <= 60 ? "Còn dưới 1 phút" : timeLeft <= 300 ? "Còn dưới 5 phút" : "";

  return (
    <QuizShell wide={view.layout === "toeic"}>
      <QuizTopBar
        title={view.submitted ? "Kết quả" : view.mode === "exam" ? "Thi thử" : "Luyện tập"}
        position={position}
        total={manifest.length}
        answered={answeredCount}
        timer={view.deadline && !view.submitted ? <ExamTimer secondsLeft={timeLeft} /> : undefined}
        marked={flags[view.currentIndex] ?? false}
        onToggleMark={() =>
          setFlags((old) => {
            const next = [...old];
            next[view.currentIndex] = !next[view.currentIndex];
            return next;
          })
        }
        canMark={!view.submitted}
        canSubmit={!view.submitted && !busy}
        saveState={saveState}
        onRetrySave={failedAnswer ? () => void answer(failedAnswer.index, failedAnswer.value) : undefined}
        onExit={() => setView(null)}
        onSubmit={() => setConfirmSubmit(true)}
      />
      {timeWarning && (
        <p className="qz-hint" role="status" aria-live="polite">
          {timeWarning}
        </p>
      )}
      {error && (
        <p role="alert" className="qz-error">
          {failedAnswer ? `Chưa lưu được câu ${view.manifest?.[failedAnswer.index]?.number ?? failedAnswer.index + 1} — ` : ""}
          {error}
          {failedAnswer && (
            <button type="button" className="qz-link" onClick={() => void answer(failedAnswer.index, failedAnswer.value)}>
              Thử lại
            </button>
          )}
          {!pageConfirmed && (
            <button type="button" className="qz-link" onClick={() => navigate(view.currentIndex)}>
              Tải lại câu
            </button>
          )}
          {timeLeft === 0 && view.deadline && !view.submitted && (
            <button type="button" className="qz-link" onClick={submit}>
              Thử nộp lại
            </button>
          )}
        </p>
      )}

      {view.submitted && (
        <section className="qz-result" aria-live="polite">
          <p className="qz-result-score">
            <strong>{view.score ?? 0}</strong>/{manifest.length}
            <span>{Math.round(((view.score ?? 0) / Math.max(1, manifest.length)) * 100)}%</span>
          </p>
          <p className="qz-muted">
            Thời gian: {Math.floor(view.timeUsed / 60)} phút {view.timeUsed % 60} giây
          </p>
          <div className="qz-parts">
            {Object.keys(summary?.parts ?? {})
              .map(Number)
              .map((p) => {
                const indices = manifest.flatMap((item, i) => (item.part === p ? [i] : []));
                if (!indices.length) return null;
                const right = indices.filter((i) => view.results[i]).length;
                return (
                  <div key={p} className="qz-part">
                    <span>{PART_LABELS[p] ?? `Part ${p}`}</span>
                    <progress value={right} max={indices.length} />
                    <b>
                      {right}/{indices.length}
                    </b>
                  </div>
                );
              })}
          </div>
          <div className="qz-result-actions">
            <button type="button" className="qz-btn qz-btn--ghost" onClick={() => setNavigator(true)}>
              Xem lại câu sai
            </button>
            <button type="button" className="qz-btn qz-btn--primary" onClick={() => setView(null)}>
              Làm lại
            </button>
          </div>
        </section>
      )}

      <div className="qz-body">
        {audioNotice}
        {audioUrl && view.layout === "default" && <CustomAudioPlayer key={audioUrl} src={audioUrl} muted={false} />}
        {audioUrl && view.layout === "default" && (
          <button
            type="button"
            className="qz-link"
            onClick={() => {
              const entry = mediaCache.current.get(audioKey);
              if (entry) URL.revokeObjectURL(entry.url);
              mediaCache.current.delete(audioKey);
              setAudioRetry((n) => n + 1);
            }}
          >
            Tải lại âm thanh
          </button>
        )}

        {view.layout === "toeic" ? (
          <div className="qz-toeic">
            <ExamQuestionLayout
              group={group}
              groupStart={view.page[0]?.index ?? 0}
              answers={answers}
              onAnswer={(flatIndex, value) => void answer(flatIndex, value)}
              muted={muted}
              lockAudio={view.mode === "exam" && !view.submitted}
              instantFeedback={view.instant || view.submitted}
              readOnly={busy || view.submitted}
              resultByIndex={view.results}
            />
            <button type="button" className="qz-btn qz-btn--ghost qz-btn--sm" onClick={() => setMuted((v) => !v)}>
              {muted ? "Bật âm thanh" : "Tắt âm thanh"}
            </button>
          </div>
        ) : loadingPage ? (
          <QuizPageSkeleton />
        ) : (
          view.page.map((item) => (
            <article key={item.index}>
              <p className="qz-eyebrow">
                Câu {view.manifest?.[item.index]?.number} / {manifest.length}
                {view.manifest?.[item.index]?.part ? ` · ${PART_LABELS[view.manifest[item.index]!.part] ?? `Part ${view.manifest[item.index]!.part}`}` : ""}
              </p>
              {item.question.paragraphText && <RichTextContent value={item.question.paragraphText} className="qz-passage" />}
              <RichTextContent value={item.question.questionText} className="qz-question" />
              {item.question.imageUrl && (
                <img className="qz-image" src={item.question.imageUrl} alt="Hình minh hoạ câu hỏi" decoding="async" draggable={false} />
              )}
              <AnswerInput
                key={`${view.id}:${item.index}`}
                question={item.question}
                selected={view.answers[item.index]}
                pending={pending?.index === item.index ? pending : undefined}
                correctText={correctText}
                revealed={item.revealed}
                disabled={busy || view.submitted || (view.instant && view.answers[item.index] !== undefined)}
                onAnswer={(value) => void answer(item.index, value)}
              />
              {item.revealed && !view.instant && (
                <div className={`qz-feedback ${view.results[item.index] ? "qz-feedback--ok" : "qz-feedback--bad"}`} aria-live="polite">
                  <strong>{view.results[item.index] ? "✓ Chính xác" : "✗ Chưa đúng"}</strong>
                  <p>Đáp án: {item.question.options?.find((o) => o.isCorrect)?.text ?? item.question.correctAnswer}</p>
                  <RichTextContent value={item.question.answerExplanation} />
                </div>
              )}
              {view.submitted && !item.revealed && <p className="qz-muted">Câu bỏ trống. Hãy luyện lại để xem giải thích sau khi trả lời.</p>}
            </article>
          ))
        )}
      </div>

      <QuizBottomBar
        canPrev={!busy && view.currentIndex > 0}
        canNext={!busy && view.currentIndex < manifest.length - 1}
        onPrev={() => navigate(view.currentIndex - 1)}
        onNext={() => navigate(view.currentIndex + 1)}
        onOpenGrid={() => setNavigator(true)}
        nextLabel={view.submitted ? "Tiếp" : "Tiếp"}
        hint={saveState === "saving" ? "Đang lưu đáp án…" : undefined}
      />

      <FeedbackSheet
        open={feedbackOpen}
        correct={!!view.results[pageItem?.index ?? -1]}
        correctAnswer={correctText}
        explanation={pageItem?.question.answerExplanation}
        canNext={!busy && view.currentIndex < manifest.length - 1}
        onNext={() => pageItem && navigate(pageItem.index + 1)}
        onClose={() => setSheetDismissed(pageItem?.index ?? null)}
      />

      <QuestionGrid
        open={navigator}
        items={view.manifest ?? []}
        answers={view.answers}
        flags={flags}
        results={view.submitted ? view.results : undefined}
        currentIndex={view.currentIndex}
        onNavigate={navigate}
        onClose={() => setNavigator(false)}
      />

      {confirmSubmit && (
        <div className="qz-sheet-backdrop">
          <section role="dialog" aria-modal="true" aria-label="Nộp bài" className="qz-sheet qz-sheet--confirm">
            <h3>Nộp bài?</h3>
            <p>Còn {manifest.length - answeredCount} câu chưa trả lời.</p>
            <div className="qz-sheet-actions">
              <button type="button" className="qz-btn qz-btn--ghost" onClick={() => setConfirmSubmit(false)}>
                Tiếp tục
              </button>
              {manifest.length > answeredCount && (
                <button
                  type="button"
                  className="qz-btn qz-btn--ghost"
                  onClick={() => {
                    setConfirmSubmit(false);
                    const next = manifest.findIndex((_, i) => view.answers[i] === undefined);
                    if (next >= 0) navigate(next);
                  }}
                >
                  Đến câu chưa làm
                </button>
              )}
              <button type="button" className="qz-btn qz-btn--primary" disabled={busy} onClick={submit}>
                Xác nhận nộp bài
              </button>
            </div>
          </section>
        </div>
      )}
    </QuizShell>
  );
}
