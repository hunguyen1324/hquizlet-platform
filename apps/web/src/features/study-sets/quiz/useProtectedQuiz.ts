import React from "react";
import { useAuth } from "../../auth/AuthContext";
import { ApiError } from "../../../lib/api/client";
import { protectedQuizApi, type ProtectedQuizView, type QuizSessionMeta, type QuizStart } from "../../../lib/api/protectedQuiz";

export type QuizSummary = { total: number; parts: Record<number, number>; sessions: string[]; activeCount: number; limit: number; studySetId: number };
export type QuizSettings = { layout: QuizStart["layout"]; part: number; instant: boolean; minutes: number; timedPractice: boolean };
type Page = ProtectedQuizView["page"];

const TICKET_TTL_MS = 100 * 60 * 1000; // server tickets live 2 hours

/** Index of the first question of the page after the current one (or null at the end). */
export function nextPageIndex(view: ProtectedQuizView): number | null {
  const { manifest, currentIndex } = view;
  const here = manifest[currentIndex];
  if (!here) return null;
  for (let i = currentIndex + 1; i < manifest.length; i++) {
    if (view.layout !== "toeic" || manifest[i]!.group !== here.group) return i;
  }
  return null;
}

/** Index of the first question of the page before the current one (or null at the start). */
export function prevPageIndex(view: ProtectedQuizView): number | null {
  const { manifest, currentIndex } = view;
  const here = manifest[currentIndex];
  if (!here || currentIndex === 0) return null;
  if (view.layout !== "toeic") return currentIndex - 1;
  let i = currentIndex - 1;
  const group = manifest[i]!.group;
  while (i > 0 && manifest[i - 1]!.group === group) i--;
  return i;
}

/**
 * All server interaction for a protected quiz session.
 *
 * Speed comes from three things that keep waiting off the learner's screen:
 *  1. pages already seen are kept in memory, so going back is instant;
 *  2. the next page is fetched ahead of time (and its audio ticket warmed);
 *  3. answers show immediately and are sent in order in the background.
 * Questions are still only delivered one page at a time and answers are never sent early.
 */
export function useProtectedQuiz(studySetId: number, resumeSessionId?: string) {
  const { token, user } = useAuth();
  const storageKey = `hquizlet:protected-quiz:${user?.id}:${studySetId}`;

  const [summary, setSummary] = React.useState<QuizSummary | null>(null);
  const [recent, setRecent] = React.useState<QuizSessionMeta[]>([]);
  const [view, setView] = React.useState<ProtectedQuizView | null>(null);
  const [savedId, setSavedId] = React.useState<string | null>(() => localStorage.getItem(storageKey));
  const [settings, setSettings] = React.useState<QuizSettings>({ layout: "default", part: 0, instant: true, minutes: 120, timedPractice: false });
  const [inflight, setInflight] = React.useState(0);
  const [error, setError] = React.useState("");
  const [sessionLimitError, setSessionLimitError] = React.useState(false);
  const [pending, setPending] = React.useState<Record<number, string>>({});
  const [flags, setFlags] = React.useState<boolean[]>([]);
  const [timeLeft, setTimeLeft] = React.useState(0);
  const [clockOffset, setClockOffset] = React.useState(0);

  const pages = React.useRef(new Map<number, Page>());
  const queue = React.useRef<Promise<unknown>>(Promise.resolve());
  const navSeq = React.useRef(0);
  const syncTimer = React.useRef<number | undefined>(undefined);
  const expirySubmitted = React.useRef<string | null>(null);
  const viewRef = React.useRef<ProtectedQuizView | null>(null);
  viewRef.current = view;

  const busy = inflight > 0;

  const dropSession = React.useCallback(() => {
    setView(null);
    pages.current.clear();
    localStorage.removeItem(storageKey);
    setSavedId(null);
  }, [storageKey]);

  const remember = React.useCallback((next: ProtectedQuizView) => {
    for (const item of next.page) pages.current.set(item.index, next.page);
  }, []);

  /** Run server calls one at a time, in the order the learner made them. */
  const enqueue = React.useCallback(<T,>(work: () => Promise<T>): Promise<T | undefined> => {
    setInflight((n) => n + 1);
    const run = queue.current.then(work, work);
    queue.current = run.catch(() => undefined);
    return run.then((value) => value, (e: unknown) => {
      if (e instanceof ApiError && e.status === 409) { setSessionLimitError(true); return undefined; }
      setError(e instanceof Error ? e.message : "Không thực hiện được yêu cầu.");
      if (e instanceof ApiError && [401, 403, 404].includes(e.status)) dropSession();
      return undefined;
    }).finally(() => setInflight((n) => Math.max(0, n - 1)));
  }, [dropSession]);

  // ── start screen data ────────────────────────────────────
  React.useEffect(() => {
    let active = true;
    localStorage.removeItem(`hquizlet:quiz-session:${studySetId}`); // legacy local sessions contained answers
    protectedQuizApi.summary(token, studySetId).then((data) => { if (active) setSummary(data); }).catch((e: Error) => { if (active) setError(e.message); });
    protectedQuizApi.listSessions(token, { studySetId, perPage: 5 }).then((r) => { if (active) setRecent(r.items); }).catch(() => undefined);
    return () => { active = false; };
  }, [token, studySetId, view === null]); // eslint-disable-line react-hooks/exhaustive-deps

  // ── applying server responses ────────────────────────────
  const replaceView = React.useCallback((next: ProtectedQuizView) => {
    pages.current.clear();
    remember(next);
    setPending({});
    setClockOffset(Date.parse(next.serverTime) - Date.now());
    setView(next);
    setSettings((s) => ({ ...s, layout: next.layout, instant: next.instant }));
    localStorage.setItem(storageKey, next.id);
    setSavedId(next.id);
  }, [remember, storageKey]);

  /** Take answers/results from a response but leave the page the learner is looking at alone. */
  const mergeView = React.useCallback((next: ProtectedQuizView) => {
    remember(next);
    setClockOffset(Date.parse(next.serverTime) - Date.now());
    setView((prev) => {
      if (!prev || prev.id !== next.id) return prev;
      const showsSamePage = next.page.some((item) => item.index === prev.currentIndex);
      return { ...next, currentIndex: prev.currentIndex, page: showsSamePage ? next.page : (pages.current.get(prev.currentIndex) ?? prev.page) };
    });
  }, [remember]);

  const open = React.useCallback((id: string, index = -1) => {
    setError(""); setSessionLimitError(false); setFlags([]); expirySubmitted.current = null;
    void enqueue(() => protectedQuizApi.page(token, studySetId, id, index)).then((next) => { if (next) replaceView(next); });
  }, [enqueue, replaceView, studySetId, token]);

  React.useEffect(() => { if (resumeSessionId) open(resumeSessionId); }, [resumeSessionId]); // eslint-disable-line react-hooks/exhaustive-deps

  const start = React.useCallback((mode: QuizStart["mode"]) => {
    if (busy) return;
    setError(""); setSessionLimitError(false); setFlags([]); expirySubmitted.current = null;
    const s = settings;
    void enqueue(() => protectedQuizApi.start(token, studySetId, {
      mode, layout: s.layout, part: s.layout === "toeic" ? s.part : 0, instant: s.instant,
      durationSeconds: mode === "exam" || s.timedPractice ? s.minutes * 60 : 0,
    })).then((next) => { if (next) replaceView(next); });
  }, [busy, enqueue, replaceView, settings, studySetId, token]);

  // ── navigation: memory first, server in the background ───
  const scheduleSync = React.useCallback((id: string, index: number) => {
    window.clearTimeout(syncTimer.current);
    syncTimer.current = window.setTimeout(() => {
      // Only the saved position matters here; the page is already on screen.
      void enqueue(() => protectedQuizApi.page(token, studySetId, id, index));
    }, 500);
  }, [enqueue, studySetId, token]);
  React.useEffect(() => () => window.clearTimeout(syncTimer.current), []);

  const navigate = React.useCallback((index: number) => {
    const current = viewRef.current;
    if (!current || index < 0 || index >= current.manifest.length) return;
    const seq = ++navSeq.current;
    const cached = pages.current.get(index);
    if (cached) {
      setView((prev) => (prev ? { ...prev, currentIndex: index, page: cached } : prev));
      scheduleSync(current.id, index);
      return;
    }
    void enqueue(() => protectedQuizApi.page(token, studySetId, current.id, index)).then((next) => {
      if (!next) return;
      if (seq === navSeq.current) replaceViewKeepingAnswers(next); else remember(next);
    });
  }, [enqueue, remember, scheduleSync, studySetId, token]); // eslint-disable-line react-hooks/exhaustive-deps

  // A page response replaces the page but must not wipe optimistic answers that are still in flight.
  function replaceViewKeepingAnswers(next: ProtectedQuizView) {
    remember(next);
    setClockOffset(Date.parse(next.serverTime) - Date.now());
    setView(next);
  }

  const answer = React.useCallback((index: number, value: string) => {
    const current = viewRef.current;
    if (!current || current.submitted) return;
    if (current.instant && current.answers[index] !== undefined) return;
    setPending((old) => ({ ...old, [index]: value }));
    void enqueue(() => protectedQuizApi.answer(token, studySetId, current.id, index, value)).then((next) => {
      setPending((old) => { const rest = { ...old }; if (rest[index] === value) delete rest[index]; return rest; });
      if (next) mergeView(next);
    });
  }, [enqueue, mergeView, studySetId, token]);

  const submit = React.useCallback(() => {
    const current = viewRef.current;
    if (!current || current.submitted) return;
    void enqueue(() => protectedQuizApi.submit(token, studySetId, current.id)).then((next) => {
      if (!next) return;
      pages.current.clear(); // explanations are only revealed after submission
      replaceView(next);
    });
  }, [enqueue, replaceView, studySetId, token]);

  const exit = React.useCallback(() => {
    window.clearTimeout(syncTimer.current);
    setView(null);
    pages.current.clear();
    setPending({});
  }, []);

  // ── deadline ─────────────────────────────────────────────
  React.useEffect(() => {
    if (!view?.deadline || view.submitted) return;
    const tick = () => {
      const left = Math.max(0, Math.ceil((Date.parse(view.deadline!) - Date.now() - clockOffset) / 1000));
      setTimeLeft(left);
      if (left === 0 && expirySubmitted.current !== view.id) { expirySubmitted.current = view.id; submit(); }
    };
    tick();
    const timer = window.setInterval(tick, 500);
    return () => window.clearInterval(timer);
  }, [view?.id, view?.deadline, view?.submitted, clockOffset, submit]);

  // ── audio: a short-lived ticket per question, streamed by the browser ──
  const tickets = React.useRef(new Map<string, { url: string; at: number }>());
  const getAudioUrl = React.useCallback(async (sessionId: string, index: number, signal?: AbortSignal) => {
    const key = `${sessionId}:${index}`;
    const hit = tickets.current.get(key);
    if (hit && Date.now() - hit.at < TICKET_TTL_MS) return hit.url;
    const url = await protectedQuizApi.audio(token, studySetId, sessionId, index, signal ?? new AbortController().signal);
    tickets.current.set(key, { url, at: Date.now() });
    return url;
  }, [studySetId, token]);
  const forgetAudio = React.useCallback((sessionId: string, index: number) => { tickets.current.delete(`${sessionId}:${index}`); }, []);
  React.useEffect(() => { tickets.current.clear(); }, [token, studySetId]);

  // ── prefetch the next page while the learner reads this one ──
  React.useEffect(() => {
    if (!view || view.submitted || busy) return;
    const next = nextPageIndex(view);
    if (next === null || pages.current.has(next)) return;
    const sessionId = view.id;
    let cancelled = false;
    const timer = window.setTimeout(() => {
      protectedQuizApi.peek(token, studySetId, sessionId, next).then(async (peeked) => {
        if (cancelled) return;
        remember(peeked);
        const withAudio = peeked.page.find((item) => item.hasAudio);
        if (!withAudio) return;
        // Ask for the first byte so the server has the file ready when playback starts.
        const url = await getAudioUrl(sessionId, withAudio.index);
        if (!cancelled) await fetch(url, { headers: { Range: "bytes=0-0" }, cache: "no-store" }).catch(() => undefined);
      }).catch(() => undefined); // prefetch is best-effort
    }, 250);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [view?.id, view?.currentIndex, view?.submitted, busy, getAudioUrl, remember, studySetId, token]); // eslint-disable-line react-hooks/exhaustive-deps

  return {
    summary, recent, view, settings, setSettings, busy, error, setError, sessionLimitError, savedId, pending,
    flags, setFlags, timeLeft, start, open, navigate, answer, submit, exit, getAudioUrl, forgetAudio,
  };
}
