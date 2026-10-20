import React from "react";
import type { ProtectedQuizView } from "../../../lib/api/protectedQuiz";
import { ViewerWatermark } from "../../../components/learning/ViewerWatermark";
import { CustomAudioPlayer } from "../exam/ExamQuestionLayout";
import { QuizQuestionView } from "./QuizQuestionView";
import type { AudioState } from "./QuizAudio";
import { formatClock, isTypingTarget } from "./quizHelpers";

export function QuizSessionDefault({ view, answers, pending, flags, onToggleFlag, timeLeft, busy, audio, onNavigate, onAnswer, onExit, onRequestSubmit, onOpenNavigator, onBackToResult }: {
  view: ProtectedQuizView;
  answers: Record<number, string>;
  pending: Record<number, string>;
  flags: boolean[];
  onToggleFlag: () => void;
  timeLeft: number;
  busy: boolean;
  audio: AudioState;
  onNavigate: (index: number) => void;
  onAnswer: (index: number, value: string) => void;
  onExit: () => void;
  onRequestSubmit: () => void;
  onOpenNavigator: () => void;
  onBackToResult: () => void;
}) {
  const total = view.manifest.length;
  const index = view.currentIndex;
  const item = view.page.find((p) => p.index === index) ?? view.page[0];
  const answeredCount = Object.keys(answers).length;
  const last = index >= total - 1;
  const showTimer = !!view.deadline && !view.submitted;
  const canAdvance = !view.instant || view.submitted || answers[index] !== undefined;

  const goNext = React.useCallback(() => {
    if (index < total - 1) onNavigate(index + 1);
    else if (!view.submitted) onRequestSubmit();
  }, [index, total, onNavigate, onRequestSubmit, view.submitted]);
  const goPrev = React.useCallback(() => { if (index > 0) onNavigate(index - 1); }, [index, onNavigate]);

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.ctrlKey || e.metaKey || e.altKey || isTypingTarget(e.target)) return;
      if (document.querySelector("[role='dialog']")) return;
      if (e.key === "ArrowRight" || (e.key === "Enter" && canAdvance && !(e.target instanceof HTMLButtonElement))) { e.preventDefault(); goNext(); }
      else if (e.key === "ArrowLeft") { e.preventDefault(); goPrev(); }
      else if (e.key.toLowerCase() === "f" && !view.submitted) { e.preventDefault(); onToggleFlag(); }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [canAdvance, goNext, goPrev, onToggleFlag, view.submitted]);

  // Keep the next question at the top, like turning a page.
  const scroller = React.useRef<HTMLDivElement>(null);
  React.useEffect(() => { scroller.current?.scrollTo({ top: 0 }); }, [index]);

  const audioNode = item?.hasAudio ? (audio.url
    ? <CustomAudioPlayer key={audio.url} src={audio.url} muted={false} />
    : audio.error
      ? <div className="qz-error" role="alert">{audio.error} <button onClick={audio.retry}>Thử lại</button></div>
      : <p className="qz-hint" role="status">Đang chuẩn bị âm thanh…</p>) : null;

  return <div className="qz-shell">
    <ViewerWatermark />
    {busy && <div className="qz-loadbar" role="progressbar" aria-label="Đang lưu" />}
    <header className="qz-top">
      <button className="qz-btn" onClick={view.submitted ? onBackToResult : onExit}>{view.submitted ? "← Kết quả" : "← Lưu và thoát"}</button>
      <div className="qz-top-progress">
        <div><span>Câu {view.manifest[index]?.number ?? index + 1}/{total}</span><span className="qz-muted">Đã trả lời {answeredCount}</span></div>
        <div className="qz-bar" aria-hidden="true"><span style={{ width: `${((index + 1) / total) * 100}%` }} /></div>
      </div>
      {showTimer && <span className={`qz-timer${timeLeft <= 300 ? " low" : ""}`} role="timer" aria-label="Thời gian còn lại">{formatClock(timeLeft)}</span>}
      {!view.submitted && <button className="qz-btn primary keep-label" onClick={onRequestSubmit}><span className="qz-btn-label">Nộp bài</span></button>}
    </header>

    <div className="qz-scroll" ref={scroller}>
      {item
        ? <QuizQuestionView key={`${view.id}:${item.index}`} item={item} number={view.manifest[item.index]?.number ?? item.index + 1} total={total}
            answer={answers[item.index]} pendingValue={pending[item.index]} result={view.results[item.index]} instant={view.instant} submitted={view.submitted}
            audio={audioNode} onAnswer={(v) => onAnswer(item.index, v)} />
        : <div className="qz-column" aria-busy="true"><p className="qz-hint">Đang tải câu hỏi…</p></div>}
    </div>

    <footer className="qz-bottom">
      <div className="qz-bottom-side">
        <button className="qz-btn flag" aria-pressed={flags[index] ?? false} onClick={onToggleFlag} disabled={view.submitted} title="Đánh dấu để xem lại (phím F)">
          <span aria-hidden="true">{flags[index] ? "★" : "☆"}</span><span className="qz-btn-label">Đánh dấu</span>
        </button>
        <button className="qz-btn keep-label" onClick={onOpenNavigator}><span aria-hidden="true">▦</span><span className="qz-btn-label">Danh sách câu</span></button>
      </div>
      <div className="qz-bottom-side">
        <button className="qz-btn" onClick={goPrev} disabled={index === 0} title="Câu trước (←)"><span aria-hidden="true">←</span><span className="qz-btn-label">Câu trước</span></button>
        <button className={`qz-btn${canAdvance ? " primary" : ""}`} onClick={goNext} disabled={view.submitted && last} title="Câu sau (→)">
          <span className="qz-btn-label">{last && !view.submitted ? "Nộp bài" : "Câu sau"}</span><span aria-hidden="true">→</span>
        </button>
      </div>
    </footer>
  </div>;
}
