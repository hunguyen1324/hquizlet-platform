import React from "react";
import "./quiz/quiz-player.css";
import { contentProtection } from "./exam/contentProtection";
import { ExamTopBar } from "./exam/ExamTopBar";
import { ExamBottomBar } from "./exam/ExamBottomBar";
import { ExamQuestionLayout } from "./exam/ExamQuestionLayout";
import { QuestionNavigatorPanel } from "./exam/QuestionNavigatorPanel";
import type { ExamItem } from "./exam/types";
import { ViewerWatermark } from "../../components/learning/ViewerWatermark";
import { useProtectedQuiz, nextPageIndex, prevPageIndex } from "./quiz/useProtectedQuiz";
import { useQuizAudioUrl } from "./quiz/QuizAudio";
import { QuizStartScreen } from "./quiz/QuizStartScreen";
import { QuizSessionDefault } from "./quiz/QuizSessionDefault";
import { QuizNavigator } from "./quiz/QuizNavigator";
import { QuizResult } from "./quiz/QuizResult";

export function ProtectedQuizPlayer({ studySetId, onManageSessions, resumeSessionId }: { studySetId: number; resumeSessionId?: string; onManageSessions?: (studySetId: number) => void }) {
  const quiz = useProtectedQuiz(studySetId, resumeSessionId);
  const { view, pending, flags, setFlags } = quiz;
  const audio = useQuizAudioUrl(view, quiz.getAudioUrl, quiz.forgetAudio);
  const [reviewing, setReviewing] = React.useState(false);
  const [navigatorOpen, setNavigatorOpen] = React.useState(false);
  const [confirmSubmit, setConfirmSubmit] = React.useState(false);
  const [muted, setMuted] = React.useState(false);

  // A finished session opens on its result; reviewing is chosen from there.
  React.useEffect(() => { setReviewing(false); setNavigatorOpen(false); setConfirmSubmit(false); }, [view?.id, view?.submitted]);

  if (!view) {
    return <div className="qz"><QuizStartScreen summary={quiz.summary} recent={quiz.recent} savedId={quiz.savedId} settings={quiz.settings} setSettings={quiz.setSettings}
      busy={quiz.busy} error={quiz.error} sessionLimitError={quiz.sessionLimitError} onStart={quiz.start} onOpen={(id) => quiz.open(id)}
      onManageSessions={onManageSessions} studySetId={studySetId} /></div>;
  }

  // While answers are being saved, show them straight away. Instant mode waits for the server's verdict instead.
  const shownAnswers: Record<number, string> = view.instant ? view.answers : { ...pending, ...view.answers };
  const answeredCount = Object.keys(shownAnswers).length;
  const total = view.manifest.length;
  const toggleFlag = () => setFlags((old) => { const next = [...old]; next[view.currentIndex] = !next[view.currentIndex]; return next; });
  const requestSubmit = () => setConfirmSubmit(true);

  if (view.submitted && !reviewing) {
    return <div className="qz" {...contentProtection}>
      <QuizResult view={view} onReview={() => setReviewing(true)} onRestart={quiz.exit} /></div>;
  }

  const unanswered = view.manifest.findIndex((_, i) => shownAnswers[i] === undefined);
  const dialog = confirmSubmit && <div className="qz-dialog-wrap" onClick={() => setConfirmSubmit(false)}>
    <div className="qz-dialog" role="dialog" aria-modal="true" aria-label="Nộp bài" onClick={(e) => e.stopPropagation()}>
      <h3>Nộp bài?</h3>
      <p>Bạn đã trả lời {answeredCount}/{total} câu{total - answeredCount > 0 ? `, còn ${total - answeredCount} câu chưa làm` : ""}. Sau khi nộp sẽ không sửa được đáp án.</p>
      <div className="qz-dialog-actions">
        {unanswered >= 0 && <button className="qz-btn" onClick={() => { setConfirmSubmit(false); quiz.navigate(unanswered); }}>Đến câu chưa làm</button>}
        <button className="qz-btn" onClick={() => setConfirmSubmit(false)}>Làm tiếp</button>
        <button className="qz-btn primary" disabled={quiz.busy} onClick={() => { setConfirmSubmit(false); quiz.submit(); }}>Nộp bài</button>
      </div>
    </div>
  </div>;

  if (view.layout !== "toeic") {
    return <div className="qz" {...contentProtection}>
      <QuizSessionDefault view={view} answers={shownAnswers} pending={pending} flags={flags} onToggleFlag={toggleFlag} timeLeft={quiz.timeLeft} busy={quiz.busy}
        audio={audio} onNavigate={quiz.navigate} onAnswer={quiz.answer} onExit={quiz.exit} onRequestSubmit={requestSubmit}
        onOpenNavigator={() => setNavigatorOpen(true)} onBackToResult={() => setReviewing(false)} />
      {navigatorOpen && <QuizNavigator view={view} answers={shownAnswers} flags={flags} onNavigate={quiz.navigate} onClose={() => setNavigatorOpen(false)} />}
      {dialog}
    </div>;
  }

  // TOEIC (IIG) layout: two columns, one passage with its questions per page.
  const manifest: ExamItem[] = view.manifest.map((item, i) => ({ key: String(i), groupKey: String(item.group), questionNumber: item.number, part: item.part, question: { questionType: "multiple_choice", questionText: "" } }));
  const answers = manifest.map((_, i) => shownAnswers[i] ?? null);
  const audioItem = view.page.find((item) => item.hasAudio);
  const group: ExamItem[] = view.page.map((item) => ({
    ...manifest[item.index]!, question: { ...item.question, subQuestions: undefined, audioUrl: item.index === audioItem?.index ? audio.url : undefined },
    parent: item.question.paragraphText ? { questionType: "paragraph", questionText: "", paragraphText: item.question.paragraphText, audioUrl: audio.url } : undefined,
  }));
  const currentPart = view.manifest[view.currentIndex]?.part ?? 1;
  const results = view.submitted ? manifest.map((_, i) => ({ isCorrect: view.results[i] ?? false })) : undefined;
  const prev = prevPageIndex(view);
  const next = nextPageIndex(view);
  const audioNotice = audio.error
    ? <div role="alert" className="qz-error" style={{ margin: 8 }}>{audio.error} <button onClick={audio.retry}>Thử lại</button></div>
    : audioItem && !audio.url ? <p role="status" className="qz-hint" style={{ margin: 8 }}>Đang chuẩn bị âm thanh…</p> : null;

  return <div className="qz fixed inset-0 z-[130] flex flex-col bg-white" {...contentProtection}>
    <ViewerWatermark />
    {quiz.busy && <div className="qz-loadbar" role="progressbar" aria-label="Đang lưu" />}
    <button type="button" className="bg-[#1a2744] px-4 py-1 text-left text-sm text-white" onClick={view.submitted ? () => setReviewing(false) : quiz.exit}>
      {view.submitted ? "← Về kết quả" : "← Lưu và thoát"}
    </button>
    {quiz.error && <p role="alert" className="qz-error" style={{ margin: 8 }}>{quiz.error}</p>}
    <ExamTopBar currentPart={currentPart <= 4 ? "Listening" : "Reading"} questionRange={String(view.manifest[view.currentIndex]?.number ?? 1)} totalQuestions={total}
      answeredCount={answeredCount} timeLeft={quiz.timeLeft} showTimer={!!view.deadline && !view.submitted} muted={muted} onVolumeChange={() => setMuted((v) => !v)} onSubmit={requestSubmit} />
    {audioNotice}
    <div className="min-h-0 flex-1 overflow-hidden">
      <ExamQuestionLayout group={group} groupStart={view.page[0]?.index ?? 0} answers={answers} onAnswer={quiz.answer} muted={muted}
        lockAudio={view.mode === "exam" && !view.submitted} instantFeedback={view.instant || view.submitted} readOnly={view.submitted} resultByIndex={view.results} />
    </div>
    <ExamBottomBar isMarked={flags[view.currentIndex] ?? false} onToggleMark={toggleFlag} onOpenNavigator={() => setNavigatorOpen(true)}
      onPrev={() => prev !== null && quiz.navigate(prev)} onNext={() => next !== null && quiz.navigate(next)} canPrev={prev !== null} canNext={next !== null} />
    <QuestionNavigatorPanel items={manifest} currentIndex={view.currentIndex} answers={answers} flagged={flags} results={results} onNavigate={quiz.navigate}
      isOpen={navigatorOpen} onClose={() => setNavigatorOpen(false)} />
    {dialog}
  </div>;
}
