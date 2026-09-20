// FlashcardsMode — Quizlet-style UI (PM rewrite)
// Unified design system: ql-* classes, full Quizlet parity

import React from "react";
import { useAuth } from "../auth/AuthContext";
import type { Flashcard } from "./types";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import { useQuizGeneration } from "./useQuizGeneration";
import { quizApi } from "../../lib/api/client";
import { FlashcardsSettingsDialog } from "./FlashcardsSettingsDialog";
import type { FlashcardsSettings } from "./FlashcardsSettingsDialog";
import "./learning.css";

/** Phát âm text qua Web Speech API */
function speakText(text: string) {
  if (!window.speechSynthesis || !text.trim()) return;
  window.speechSynthesis.cancel();
  const utt = new SpeechSynthesisUtterance(text);
  utt.lang = "vi-VN";
  utt.rate = 0.9;
  window.speechSynthesis.speak(utt);
}

type Props = {
  cards: Flashcard[];
  studySetId: number;
  totalCount?: number; // tổng thẻ thật từ server
};

export function FlashcardsMode({ cards, studySetId, totalCount }: Props) {
  const { token } = useAuth();
  // displayTotal được tính từ nhiều nguồn theo độ ưu tiên:
  // 1. total từ API generate (chính xác nhất — tổng thẻ thật trước limit/offset)
  // 2. totalCount prop từ StudyDetail (fallback khi batch đầu chưa ready)
  // 3. cards.length (last resort)
  const [apiTotal, setApiTotal] = React.useState<number | null>(null);
  const BATCH = 50; // thẻ mỗi lần fetch
  const PRELOAD_THRESHOLD = 20; // fetch thêm khi còn cách cuối 20 thẻ

  const generation = useQuizGeneration(studySetId, "flashcards", BATCH);

  // Lấy total từ API generate (nguồn chính xác nhất) và lưu vào state
  React.useEffect(() => {
    if (generation.state.state === "ready" && generation.state.data.total > 0) {
      setApiTotal(generation.state.data.total);
    }
  }, [generation.state]);

  // displayTotal: ưu tiên apiTotal → totalCount prop → cards.length
  const displayTotal = apiTotal ?? Math.max(totalCount ?? cards.length, cards.length);

  // Lưu seed của batch đầu để dùng lại khi preload batch tiếp theo.
  // Cùng seed + tăng offset → deck shuffle giống nhau, không bao giờ trùng thẻ.
  const initialSeedRef = React.useRef<number | null>(null);
  const nextOffsetRef = React.useRef<number>(0); // offset của batch tiếp theo = số thẻ đã nhận thực tế
  const [startedAt, setStartedAt] = React.useState(() => new Date());
  const [shuffled, setShuffled] = React.useState(false);
  const [starredOnly, setStarredOnly] = React.useState(false);
  const [showBothSides, setShowBothSides] = React.useState(false);
  const [deck, setDeck] = React.useState<Flashcard[]>(cards);
  const [index, setIndex] = React.useState(0);
  const [flipped, setFlipped] = React.useState(false);
  const [seenCardIds, setSeenCardIds] = React.useState<Set<number>>(new Set());
  const completionTriggered = React.useRef(false);
  const touchStartX = React.useRef<number | null>(null);
  const [showSettings, setShowSettings] = React.useState(false);
  const [fcSettings, setFcSettings] = React.useState<FlashcardsSettings>({
    startFromDefinition: false,
    autoPlay: false,
  });
  const [loadingMore, setLoadingMore] = React.useState(false);
  // Ref để handleNext luôn đọc được deck.length mới nhất trong setTimeout closure
  const deckLengthRef = React.useRef<number>(cards.length);
  const displayTotalRef = React.useRef<number>(0);
  const apiTotalRef = React.useRef<number | null>(null);

  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({
    studySetId,
    mode: "flashcards",
  });

  // Ref thay vì useMemo để tránh reference thay đổi mỗi khi allCards infinite-scroll
  // load trang mới, vì điều đó reset deck/index/seen trong khi đang học
  const cardByIdRef = React.useRef<Map<number, Flashcard>>(new Map());
  cardByIdRef.current = new Map(cards.map((c) => [c.id, c]));

  React.useEffect(() => {
    if (generation.state.state !== "ready") return;
    const generated = generation.state.data.items.map((item) => {
      // cardByIdRef là fallback cho các field phụ (imageUrl, hint) nếu backend chưa trả
      const full = cardByIdRef.current.get(item.flashcardId);
      return {
        id: item.flashcardId,
        studySetId,
        term: item.term ?? full?.term ?? "",
        definition: item.definition ?? full?.definition ?? "",
        starred: item.starred ?? full?.starred ?? false,
        // API generate đã trả imageUrl — dùng nó trước, fallback về cardByIdRef
        imageUrl: (item as { imageUrl?: string | null }).imageUrl ?? full?.imageUrl ?? null,
        exampleSentence: (item as { exampleSentence?: string | null }).exampleSentence ?? full?.exampleSentence ?? null,
        hintExplanation: (item as { hintExplanation?: string | null }).hintExplanation ?? full?.hintExplanation ?? null,
      };
    });
    const base = starredOnly ? generated.filter((c) => c.starred) : generated;
    // Ghi nhớ seed + offset ngay trong cùng effect để đảm bảo refs sẵn sàng
    // trước khi preload effect chạy (tránh race condition giữa 2 effects)
    initialSeedRef.current = generation.state.data.seed;
    nextOffsetRef.current = generation.state.data.items.length; // offset = số thẻ thực nhận
    setDeck(base);
    setIndex(0);
    setFlipped(false);
    setSeenCardIds(new Set());
    completionTriggered.current = false;
    resetSave();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  // cardByIdRef là ref, không cần trong deps — chỉ reset khi generation thật sự đổi
  }, [generation.state, starredOnly, studySetId, resetSave]);

  // Preload batch tiếp theo: trigger khi còn ít hơn PRELOAD_THRESHOLD thẻ phía trước
  // HOẶC ngay khi deck vừa được load mà vẫn còn thẻ chưa fetch (eager preload).
  // Dùng seed cố định (từ batch đầu) + offset tăng dần → không trùng thẻ giữa các batch.
  const preloadInProgress = React.useRef(false);
  React.useEffect(() => {
    // Chưa biết seed → chưa thể preload
    if (initialSeedRef.current === null) return;
    // Deck rỗng → chờ batch đầu về
    if (deck.length === 0) return;
    // Đã có request đang chạy
    if (preloadInProgress.current) return;
    // Đã load đủ hết rồi
    if (apiTotal !== null && deck.length >= apiTotal) return;

    const remaining = deck.length - index - 1;
    // Trigger preload khi còn <= PRELOAD_THRESHOLD thẻ HOẶC khi deck < 2*BATCH
    // (eager: không để user ngồi chờ khi deck chỉ có 50 thẻ)
    const shouldPreload = remaining <= PRELOAD_THRESHOLD || deck.length < BATCH * 2;
    if (!shouldPreload) return;

    const seedToUse = initialSeedRef.current;
    const offsetToUse = nextOffsetRef.current;

    preloadInProgress.current = true;
    setLoadingMore(true);
    quizApi
      .generate(token, studySetId, { mode: "flashcards", seed: seedToUse, limit: BATCH, offset: offsetToUse })
      .then((data) => {
        if (data.items.length === 0) return; // đã hết thẻ
        nextOffsetRef.current = offsetToUse + data.items.length; // tăng theo số thẻ thực nhận
        const newCards = data.items.map((item) => ({
          id: item.flashcardId,
          studySetId,
          term: item.term ?? "",
          definition: item.definition ?? "",
          starred: item.starred ?? false,
          imageUrl: (item as { imageUrl?: string | null }).imageUrl ?? null,
          exampleSentence: (item as { exampleSentence?: string | null }).exampleSentence ?? null,
          hintExplanation: (item as { hintExplanation?: string | null }).hintExplanation ?? null,
        }));
        const filtered = starredOnly ? newCards.filter((c) => c.starred) : newCards;
        // Dedup phòng thủ (offset đảm bảo không trùng nhưng phòng race condition)
        setDeck((prev) => {
          const existingIds = new Set(prev.map((c) => c.id));
          const unique = filtered.filter((c) => !existingIds.has(c.id));
          return unique.length > 0 ? [...prev, ...unique] : prev;
        });
      })
      .catch(() => { /* silent fail — user vẫn lướt được trong deck hiện tại */ })
      .finally(() => {
        preloadInProgress.current = false;
        setLoadingMore(false);
      });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [index, deck.length, apiTotal]);

  const current = deck[index];
  const total = deck.length;
  // Đồng bộ refs để closure trong setTimeout luôn có giá trị mới nhất
  deckLengthRef.current = total;
  displayTotalRef.current = displayTotal;
  apiTotalRef.current = apiTotal;

  React.useEffect(() => {
    if (!current) return;
    setSeenCardIds((prev) => {
      if (prev.has(current.id)) return prev;
      const next = new Set(prev);
      next.add(current.id);
      return next;
    });
  }, [current]);

  React.useEffect(() => {
    if (completionTriggered.current || total === 0) return;
    // QUAN TRỌNG: chỉ trigger completion khi đã biết tổng thật từ API (apiTotal != null).
    // Nếu dùng displayTotal (có thể vẫn là fallback từ totalCount prop = 50 thẻ đầu),
    // sẽ trigger sai khi deck chỉ mới load 50/3188 thẻ.
    if (apiTotal === null) return; // chưa biết tổng thật — chờ API trả về
    const allRealCardsLoaded = total >= apiTotal;
    if (allRealCardsLoaded && seenCardIds.size >= total) {
      completionTriggered.current = true;
      onSessionComplete({
        score: total,
        total,
        cardResults: deck.map((c) => ({ flashcardId: c.id, correct: true, attempts: 1 })),
        startedAt,
      });
    }
  }, [seenCardIds, total, displayTotal, apiTotal]); // eslint-disable-line react-hooks/exhaustive-deps

  // Auto-play TTS when card becomes front
  React.useEffect(() => {
    if (!fcSettings.autoPlay || !current || flipped) return;
    const text = fcSettings.startFromDefinition ? current.definition : current.term;
    speakText(text);
  }, [index, flipped, fcSettings.autoPlay, fcSettings.startFromDefinition, current]);

  function handleAudio() {
    if (!current) return;
    const text = flipped
      ? (fcSettings.startFromDefinition ? current.term : current.definition)
      : (fcSettings.startFromDefinition ? current.definition : current.term);
    speakText(text);
  }

  function handlePrev() {
    setFlipped(false);
    setTimeout(() => setIndex((i) => Math.max(0, i - 1)), 60);
  }
  function handleNext() {
    setFlipped(false);
    setTimeout(() => setIndex((i) => {
      const nextIdx = i + 1;
      const currentDeckLen = deckLengthRef.current;
      const currentApiTotal = apiTotalRef.current;
      // Nếu còn thẻ trong deck → tiến thẳng, không wrap
      if (nextIdx < currentDeckLen) return nextIdx;
      // Nếu chưa biết tổng thật từ API → giữ nguyên, chờ batch đầu về
      if (currentApiTotal === null) return i;
      // Nếu đã load đủ tất cả thẻ thật → wrap về 0 (đã xem hết)
      if (currentDeckLen >= currentApiTotal) return 0;
      // Đang chờ load batch mới → giữ nguyên vị trí cuối, KHÔNG wrap về 0
      return i;
    }), 60);
  }
  function handleRestart() {
    completionTriggered.current = false;
    resetSave();
    setStartedAt(new Date());
    setIndex(0);
    setFlipped(false);
    setSeenCardIds(new Set());
    generation.regenerate();
  }

  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.target instanceof HTMLButtonElement || e.target instanceof HTMLInputElement) return;
      if (e.key === " ") { e.preventDefault(); setFlipped((f) => !f); }
      if (e.key === "ArrowLeft") { e.preventDefault(); handlePrev(); }
      if (e.key === "ArrowRight") { e.preventDefault(); handleNext(); }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [total, index]);

  const starredCount = cards.filter((c) => c.starred).length;
  // allSeen chỉ true khi đã biết tổng thật từ API, đã load đủ tất cả thẻ, VÀ đã xem hết
  const allLoadedAndSeen = apiTotal !== null && deck.length >= apiTotal && seenCardIds.size >= total && total > 0;
  const allSeen = allLoadedAndSeen;
  // DEBUG — xoá sau khi xác nhận fix
  if (allSeen) console.debug("[FC] allSeen=true", { deckLen: deck.length, displayTotal, seenSize: seenCardIds.size, total });
  // Dùng displayTotal (tổng thẻ thật từ server) để progress phản ánh toàn bộ set,
  // không bị kẹt ở 50% khi deck chỉ load batch 100/3188 thẻ.
  const progressPct = displayTotal > 0 ? ((index + 1) / displayTotal) * 100 : 0;

  if (cards.length === 0) return <LearningEmptyState />;

  if (generation.state.state === "loading") {
    return (
      <div className="ql-loading" role="status">
        <div className="ql-spinner" />
        <span>Đang tạo bộ flashcards…</span>
      </div>
    );
  }
  if (generation.state.state === "error") {
    return (
      <div className="ql-error" role="alert">
        <span>Không thể tải Flashcards: {generation.state.error.message}</span>
        <button className="ql-ghost-btn" onClick={generation.regenerate}>Thử lại</button>
      </div>
    );
  }
  if (total === 0 && starredOnly) {
    return (
      <LearningEmptyState
        message="Chưa có thẻ nào được đánh dấu sao."
        hint="Đánh dấu sao trong danh sách thuật ngữ bên dưới, rồi bật lọc lại."
      />
    );
  }

  // Flip respects startFromDefinition setting
  const frontLabel = fcSettings.startFromDefinition ? "Định nghĩa" : "Thuật ngữ";
  const backLabel = fcSettings.startFromDefinition ? "Thuật ngữ" : "Định nghĩa";
  const frontText = fcSettings.startFromDefinition ? current.definition : current.term;
  const backText = fcSettings.startFromDefinition ? current.term : current.definition;

  return (
    <div className="ql-root">
      {showSettings && (
        <FlashcardsSettingsDialog
          settings={fcSettings}
          onClose={() => setShowSettings(false)}
          onChange={(s) => { setFcSettings(s); setShowSettings(false); }}
        />
      )}
      {/* ── Progress bar ── */}
      <div className="ql-progress-wrap">
        <div
          className="ql-progress-bar"
          role="progressbar"
          aria-valuenow={index + 1}
          aria-valuemax={displayTotal}
          style={{ width: `${progressPct}%` }}
        />
      </div>

      {/* ── Counter + toolbar ── */}
      <div className="ql-topbar">
        <span className="ql-counter" aria-live="polite">
          <strong>{index + 1}</strong> / {displayTotal}
        </span>

        <div className="ql-actions">
          {/* Audio button */}
          <button
            className="ql-icon-btn"
            onClick={handleAudio}
            title="Phát âm"
            aria-label="Phát âm thẻ hiện tại"
          >
            <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>
              <path d="M15.54 8.46a5 5 0 0 1 0 7.07"/>
              <path d="M19.07 4.93a10 10 0 0 1 0 14.14"/>
            </svg>
          </button>

          {/* Starred filter */}
          {starredCount > 0 && (
            <button
              className={`ql-icon-btn${starredOnly ? " ql-icon-btn--on" : ""}`}
              onClick={() => setStarredOnly((s) => !s)}
              title={starredOnly ? "Bỏ lọc sao" : "Chỉ thẻ đã đánh dấu sao"}
              aria-pressed={starredOnly}
            >
              <svg width="17" height="17" viewBox="0 0 24 24" fill={starredOnly ? "currentColor" : "none"} stroke="currentColor" strokeWidth="2">
                <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"/>
              </svg>
            </button>
          )}

          {/* Show both sides toggle */}
          <button
            className={`ql-icon-btn${showBothSides ? " ql-icon-btn--on" : ""}`}
            onClick={() => setShowBothSides((v) => !v)}
            title="Hiện cả hai mặt"
            aria-pressed={showBothSides}
          >
            <svg width="17" height="17" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
              <rect x="2" y="4" width="9" height="16" rx="2"/>
              <rect x="13" y="4" width="9" height="16" rx="2"/>
            </svg>
          </button>

          {/* Shuffle */}
          <button
            className={`ql-icon-btn${shuffled ? " ql-icon-btn--on" : ""}`}
            onClick={() => { setShuffled(true); generation.regenerate(); }}
            title="Xáo trộn thẻ"
            aria-pressed={shuffled}
          >
            <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/>
              <polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/>
              <line x1="4" y1="4" x2="9" y2="9"/>
            </svg>
          </button>

          {/* Settings */}
          <button
            className="ql-icon-btn"
            onClick={() => setShowSettings(true)}
            title="Cài đặt"
            aria-label="Cài đặt Flashcards"
          >
            <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="3"/>
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
            </svg>
          </button>

          {/* Restart */}
          <button className="ql-icon-btn" onClick={handleRestart} title="Bắt đầu lại">
            <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 .49-4.15"/>
            </svg>
          </button>
        </div>
      </div>

      {/* ── Card ── */}
      {showBothSides ? (
        /* Both-sides view */
        <div className="ql-bothsides">
          <div className="ql-bothside-card ql-bothside-card--front">
            <span className="ql-face-label">{frontLabel}</span>
            <p className="ql-card-text">{frontText}</p>
          </div>
          <div className="ql-bothside-card ql-bothside-card--back">
            <span className="ql-face-label">{backLabel}</span>
            <p className="ql-card-text ql-card-text--def">{backText}</p>
          </div>
        </div>
      ) : (
        /* Flip card */
        <div
          className={`ql-card${flipped ? " ql-card--flipped" : ""}`}
          onClick={() => setFlipped((f) => !f)}
          onKeyDown={(e) => {
            if (e.key === " " || e.key === "Enter") { e.preventDefault(); setFlipped((f) => !f); }
          }}
          onTouchStart={(e) => { touchStartX.current = e.touches[0].clientX; }}
          onTouchEnd={(e) => {
            if (touchStartX.current === null) return;
            const dx = e.changedTouches[0].clientX - touchStartX.current;
            touchStartX.current = null;
            if (Math.abs(dx) < 40) { setFlipped((f) => !f); return; }
            if (dx < 0) handleNext(); else handlePrev();
          }}
          tabIndex={0}
          role="button"
          aria-label={flipped
            ? `${backLabel}: ${backText}. Nhấn Space để lật lại.`
            : `${frontLabel}: ${frontText}. Nhấn Space để xem ${backLabel.toLowerCase()}.`}
        >
          <div className="ql-card-inner">
            {/* Front */}
            <div className="ql-face ql-face--front">
              <span className="ql-face-label">{frontLabel}</span>
              <div className="ql-face-main">
                {current.imageUrl && (
                  <div className="ql-img-wrap">
                    <img src={current.imageUrl} alt={frontText} className="ql-img" />
                  </div>
                )}
                <p className="ql-card-text">{frontText}</p>
              </div>
              {current.exampleSentence && !fcSettings.startFromDefinition && (
                <div className="ql-face-extra">
                  <span className="ql-face-extra-label">Example</span>
                  <p className="ql-face-extra-text">{current.exampleSentence}</p>
                </div>
              )}
              <span className="ql-flip-hint">
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M7 12h10M12 7v10"/></svg>
                Nhấn để lật
              </span>
            </div>

            {/* Back */}
            <div className="ql-face ql-face--back">
              <span className="ql-face-label">{backLabel}</span>
              <div className="ql-face-main">
                <p className="ql-card-text ql-card-text--def">{backText}</p>
              </div>
              {current.hintExplanation && !fcSettings.startFromDefinition && (
                <div className="ql-face-extra">
                  <span className="ql-face-extra-label">Hint</span>
                  <p className="ql-face-extra-text">{current.hintExplanation}</p>
                </div>
              )}
              {current.starred && (
                <span className="ql-starred">
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"/></svg>
                  Đã đánh dấu sao
                </span>
              )}
            </div>
          </div>
        </div>
      )}

      {/* ── Navigation ── */}
      <div className="ql-nav">
        <button className="ql-nav-btn" onClick={handlePrev} disabled={total <= 1} aria-label="Thẻ trước">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M15 18l-6-6 6-6"/>
          </svg>
        </button>

        <div className="ql-dots" aria-hidden="true">
          {total <= 20 ? deck.map((c, i) => (
            <button
              key={i}
              className={`ql-dot${i === index ? " ql-dot--active" : ""}${seenCardIds.has(c.id) ? " ql-dot--seen" : ""}`}
              onClick={(e) => { e.stopPropagation(); setFlipped(false); setIndex(i); }}
              tabIndex={-1}
            />
          )) : (
            <span className="ql-nav-count">{index + 1} / {displayTotal}</span>
          )}
        </div>

        <button className="ql-nav-btn" onClick={handleNext} disabled={total <= 1} aria-label="Thẻ tiếp theo">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9 18l6-6-6-6"/>
          </svg>
        </button>
      </div>

      {/* ── Completion ── */}
      {allSeen && (
        <div className="ql-completion">
          <ProgressSaveStatus status={saveStatus} />
        </div>
      )}
      {/* DEBUG BANNER — xoá sau khi fix */}
      <div style={{fontSize:"11px",color:"#888",textAlign:"center",padding:"4px"}}>
        deck:{deck.length} displayTotal:{displayTotal} totalCount:{totalCount ?? "undefined"} cards:{cards.length} seen:{seenCardIds.size} allSeen:{String(allSeen)}
      </div>

      <p className="ql-kbd-hint" aria-hidden="true">
        ← → điều hướng · Space lật thẻ · vuốt trên mobile
        {loadingMore && (
          <span className="ql-loading-more">
            {" "}· Đang tải thẻ {deck.length + 1}–{Math.min(deck.length + BATCH, displayTotal)}…
          </span>
        )}
      </p>
    </div>
  );
}
