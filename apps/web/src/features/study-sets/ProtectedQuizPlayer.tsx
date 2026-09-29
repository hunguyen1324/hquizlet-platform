import React from "react";
import { useAuth } from "../auth/AuthContext";
import { protectedQuizApi, type ProtectedQuizView, type QuizStart } from "../../lib/api/protectedQuiz";
import { ApiError } from "../../lib/api/client";
import { contentProtection } from "./exam/contentProtection";
import { ExamTopBar } from "./exam/ExamTopBar";
import { ExamBottomBar } from "./exam/ExamBottomBar";
import { ExamTimer } from "./exam/ExamTimer";
import { ExamQuestionLayout, CustomAudioPlayer } from "./exam/ExamQuestionLayout";
import { QuestionNavigatorPanel } from "./exam/QuestionNavigatorPanel";
import { RichTextContent } from "./exam/RichTextContent";
import { PART_LABELS } from "./exam/toeic";
import type { ExamItem } from "./exam/types";
import type { QuizQuestion } from "../../types";

function AnswerInput({ question, selected, disabled, onAnswer }: {
 question: QuizQuestion; selected?: string; disabled: boolean; onAnswer: (answer: string) => void;
}) {
 const [draft, setDraft] = React.useState(selected ?? "");
 const [order, setOrder] = React.useState(() => (question.options ?? []).map((o) => o.text));
 if (question.questionType === "sorting") return <fieldset disabled={disabled} className="quiz-player-sorting">
   {order.map((text, i) => <div key={i} className="quiz-player-sort-row"><strong>{text}</strong><button disabled={disabled || i === 0} onClick={() => setOrder((old) => { const next = [...old]; [next[i - 1], next[i]] = [next[i]!, next[i - 1]!]; return next; })}>↑</button><button disabled={disabled || i === order.length - 1} onClick={() => setOrder((old) => { const next = [...old]; [next[i + 1], next[i]] = [next[i]!, next[i + 1]!]; return next; })}>↓</button></div>)}
   <button className="primary-button" onClick={() => onAnswer(order.join(" → "))}>Lưu thứ tự</button>
 </fieldset>;
 const options = question.questionType === "true_false" ? [{ text: "true", label: "Đúng" }, { text: "false", label: "Sai" }] : (question.options ?? []).map((o) => ({ text: o.text, label: o.text }));
 if (question.questionType !== "written" && options.length) return <fieldset disabled={disabled} className="quiz-player-options">{options.map((o, i) => <button key={i} className={`quiz-player-option${selected === o.text ? " selected" : ""}`} onClick={() => onAnswer(o.text)}><span>{String.fromCharCode(65 + i)}</span>{o.label}</button>)}</fieldset>;
 return <form className="quiz-player-written" onSubmit={(event) => { event.preventDefault(); if (draft.trim()) onAnswer(draft); }}><input aria-label="Câu trả lời" disabled={disabled} value={draft} onChange={(e) => setDraft(e.target.value)} /><button disabled={disabled || !draft.trim()} className="primary-button">Lưu đáp án</button></form>;
}

export function ProtectedQuizPlayer({ studySetId }: { studySetId: number }) {
 const { token, user } = useAuth();
 const storageKey = `hquizlet:protected-quiz:${user?.id}:${studySetId}`;
 const [summary, setSummary] = React.useState<{ total: number; parts: Record<number, number>; sessions: string[] } | null>(null);
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
 const [audioError, setAudioError] = React.useState("");
 const [audioRetry, setAudioRetry] = React.useState(0);
 const [muted, setMuted] = React.useState(false);
 const expirySubmitted = React.useRef<string | null>(null);
 const [clockOffset, setClockOffset] = React.useState(0);

 React.useEffect(() => {
   let active = true;
   // Legacy sessions could contain answers; they are no longer usable.
   localStorage.removeItem(`hquizlet:quiz-session:${studySetId}`);
   protectedQuizApi.summary(token, studySetId).then((data) => { if (active) setSummary(data); }).catch((e: Error) => { if (active) setError(e.message); });
   return () => { active = false; };
 }, [token, studySetId]);

 async function run(request: () => Promise<ProtectedQuizView>) {
   if (busyRef.current) return;
   busyRef.current = true; setBusy(true); setError("");
   try {
     const next = await request();
     setClockOffset(Date.parse(next.serverTime) - Date.now());
     setView(next); setLayout(next.layout); setInstant(next.instant);
     localStorage.setItem(storageKey, next.id); setSavedId(next.id);
   } catch (e) {
     setError(e instanceof Error ? e.message : "Không thực hiện được yêu cầu.");
     if (e instanceof ApiError && [401, 403, 404].includes(e.status)) {
       setView(null); setAudio(null);
       localStorage.removeItem(storageKey); setSavedId(null);
     }
   } finally { busyRef.current = false; setBusy(false); }
 }
 function start(mode: QuizStart["mode"]) {
   setFlags([]); expirySubmitted.current = null;
   void run(() => protectedQuizApi.start(token, studySetId, { mode, layout, part: layout === "toeic" ? part : 0, instant, durationSeconds: mode === "exam" || timedPractice ? minutes * 60 : 0 }));
 }
 function navigate(index: number) {
   if (!view) return;
   setNavigator(false);
   void run(() => protectedQuizApi.page(token, studySetId, view.id, index));
 }
 function submit() { if (view) { setConfirmSubmit(false); void run(() => protectedQuizApi.submit(token, studySetId, view.id)); } }
 function answer(index: number, value: string) { if (view && !view.submitted) void run(() => protectedQuizApi.answer(token, studySetId, view.id, index, value)); }

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
   tick(); const timer = window.setInterval(tick, 500); return () => window.clearInterval(timer);
 }, [view?.id, view?.deadline, view?.submitted, clockOffset, token, studySetId]);

 const audioItem = view?.page.find((item) => item.hasAudio);
 const audioKey = view && audioItem ? `${view.id}:${view.manifest[audioItem.index]?.group}:${audioItem.index}` : "";
 React.useEffect(() => {
   setAudio(null); setAudioError("");
   if (!view || !audioItem) return;
   const controller = new AbortController();
   protectedQuizApi.audio(token, studySetId, view.id, audioItem.index, controller.signal).then((url) => {
     if (controller.signal.aborted) return;
     setAudio({ key: audioKey, url });
   }).catch((e: Error) => { if (!controller.signal.aborted) setAudioError(e.message); });
   return () => { controller.abort(); };
 }, [audioKey, audioRetry, token, studySetId]); // Only fetch when the displayed media changes.
 const audioUrl = audio?.key === audioKey ? audio.url : undefined;

 const manifest: ExamItem[] = view?.manifest.map((item, i) => ({ key: String(i), groupKey: String(item.group), questionNumber: item.number, part: item.part, question: { questionType: "multiple_choice", questionText: "" } })) ?? [];
 const answers = manifest.map((_, i) => view?.answers[i] ?? null);
 const group: ExamItem[] = view?.page.map((item) => ({
   ...manifest[item.index]!, question: { ...item.question, subQuestions: undefined, audioUrl: item.index === audioItem?.index ? audioUrl : undefined },
   parent: item.question.paragraphText ? { questionType: "paragraph", questionText: "", paragraphText: item.question.paragraphText, audioUrl } : undefined,
 })) ?? [];
 const answeredCount = view ? Object.keys(view.answers).length : 0;
 const currentPart = view?.manifest[view.currentIndex]?.part ?? 1;
 const results = view?.submitted ? manifest.map((_, i) => ({ isCorrect: view.results[i] ?? false })) : undefined;
 const audioNotice = audioError ? <div role="alert" className="quiz-security-error">{audioError} <button onClick={() => setAudioRetry((n) => n + 1)}>Thử lại</button></div> : audioItem && !audioUrl ? <p role="status">Đang tải âm thanh…</p> : null;

 if (!view) return <section className="quiz-mode-picker">
   <div className="quiz-mode-heading"><p className="eyebrow">Quiz</p><h2>Chọn cách học</h2><p>{summary ? `${summary.total} câu hỏi` : "Đang tải thông tin bài…"}</p>
     {error && <p role="alert" className="quiz-security-error">{error}</p>}
     <div className="quiz-layout-picker" role="group" aria-label="Giao diện làm bài">{(["default", "toeic"] as const).map((value) => <button key={value} className={`quiz-layout-option${layout === value ? " active" : ""}`} onClick={() => setLayout(value)}>{value === "default" ? "Mặc định" : "TOEIC (IIG)"}</button>)}</div>
     <div className="quiz-settings">
       {layout === "toeic" && <label>Chọn Part<select value={part} onChange={(e) => setPart(Number(e.target.value))}><option value={0}>Tất cả các Part</option>{Object.entries(summary?.parts ?? {}).map(([p, count]) => <option key={p} value={p}>{PART_LABELS[Number(p)]} ({count} câu)</option>)}</select></label>}
       <label>Xem kết quả<select value={instant ? "instant" : "end"} onChange={(e) => setInstant(e.target.value === "instant")}><option value="instant">Ngay sau mỗi câu</option><option value="end">Sau khi nộp bài</option></select></label>
       <label>Thời gian làm bài (phút)<input type="number" min={1} max={600} value={minutes} onChange={(e) => setMinutes(Math.max(1, Math.min(600, Number(e.target.value) || 1)))} /></label>
       <label><input type="checkbox" checked={timedPractice} onChange={(e) => setTimedPractice(e.target.checked)} /> Đếm ngược khi luyện tập</label>
     </div>
     {(summary?.sessions ?? []).filter((id) => id !== savedId).map((id, i) => <button key={id} disabled={busy} className="secondary-button" onClick={() => void run(() => protectedQuizApi.page(token, studySetId, id))}>Mở phiên trước #{i + 1}</button>)}
     <small>Tiến độ và kết quả được lưu trong 24 giờ. Tối đa 2 phiên chưa nộp cùng lúc.</small>
     {savedId && <button className="secondary-button" disabled={busy} onClick={() => void run(() => protectedQuizApi.page(token, studySetId, savedId))}>Mở phiên gần nhất / kết quả</button>}
   </div>
   <button className="quiz-mode-card" disabled={busy || !summary?.total} onClick={() => start("practice")}><strong>Luyện tập</strong><small>Luyện theo Part, chọn thời điểm xem đáp án.</small></button>
   <button className="quiz-mode-card" disabled={busy || !summary?.total} onClick={() => start("exam")}><strong>Thi thử</strong><small>Đếm ngược và xem kết quả sau khi nộp.</small></button>
 </section>;

 const questionContent = <>
   {audioNotice}
   {audioUrl && <button className="secondary-button" onClick={() => setAudioRetry((n) => n + 1)}>Tải lại âm thanh</button>}
   {view.layout === "toeic" ? <div className="min-h-0 flex-1 overflow-hidden"><ExamQuestionLayout group={group} groupStart={view.page[0]?.index ?? 0} answers={answers} onAnswer={answer} muted={muted} lockAudio={view.mode === "exam" && !view.submitted} instantFeedback={view.instant || view.submitted} readOnly={busy || view.submitted} resultByIndex={view.results} /></div> : <main className="quiz-player quiz-player--active">
     {audioUrl && <CustomAudioPlayer key={audioUrl} src={audioUrl} muted={false} />}
     {view.page.map((item) => <article key={item.index}>
       <p className="eyebrow">Câu {view.manifest[item.index]?.number} / {view.manifest.length}</p>
       {item.question.paragraphText && <RichTextContent value={item.question.paragraphText} className="quiz-player-passage" />}
       <RichTextContent value={item.question.questionText} className="quiz-player-question" />
       <AnswerInput key={`${view.id}:${item.index}`} question={item.question} selected={view.answers[item.index]} disabled={busy || view.submitted || (view.instant && view.answers[item.index] !== undefined)} onAnswer={(value) => answer(item.index, value)} />
       {item.revealed && <div className={`quiz-player-feedback ${view.results[item.index] ? "correct" : "wrong"}`}><strong>{view.results[item.index] ? "Chính xác" : "Chưa đúng"}</strong><p>Đáp án: {item.question.options?.find((o) => o.isCorrect)?.text ?? item.question.correctAnswer}</p><RichTextContent value={item.question.answerExplanation} /></div>}
       {view.submitted && !item.revealed && <p>Câu bỏ trống. Hãy luyện lại để xem giải thích sau khi trả lời.</p>}
     </article>)}
   </main>}
 </>;
 return <section {...contentProtection} className={view.layout === "toeic" ? "fixed inset-0 z-[130] flex flex-col bg-white" : "quiz-session"}>
   <header className="quiz-session-toolbar"><button disabled={busy} className="quiz-exit" onClick={() => setView(null)}>← Lưu và thoát</button><strong>{view.submitted ? "Kết quả" : "Làm bài"}</strong><span>{answeredCount}/{manifest.length} câu</span>{view.deadline && !view.submitted && <ExamTimer secondsLeft={timeLeft} />}<button disabled={busy || view.submitted} className="primary-button" onClick={() => setConfirmSubmit(true)}>Nộp bài</button></header>
   {error && <p role="alert" className="quiz-security-error">{error}{timeLeft === 0 && view.deadline && !view.submitted && <button onClick={submit}>Thử nộp lại</button>}</p>}
   {busy && <p role="status" className="text-center text-sm">Đang lưu / tải câu hỏi…</p>}
   {view.submitted && <div className="quiz-protected-results"><strong>Đúng {view.score}/{manifest.length} · {Math.round((view.score ?? 0) / manifest.length * 100)}%</strong><span>Thời gian: {Math.floor(view.timeUsed / 60)} phút {view.timeUsed % 60} giây</span><div>{Object.keys(summary?.parts ?? {}).map(Number).map((p) => { const indices = manifest.flatMap((item, i) => item.part === p ? [i] : []); return indices.length ? <span key={p}>Part {p}: {indices.filter((i) => view.results[i]).length}/{indices.length} · </span> : null; })}</div><button disabled={busy} className="secondary-button" onClick={() => setView(null)}>Chọn Part / làm lại</button></div>}
   {view.layout === "toeic" && !view.submitted && <ExamTopBar currentPart={currentPart <= 4 ? "Listening" : "Reading"} questionRange={String(view.manifest[view.currentIndex]?.number ?? 1)} totalQuestions={manifest.length} answeredCount={answeredCount} timeLeft={timeLeft} showTimer={false} muted={muted} onVolumeChange={() => setMuted((v) => !v)} onSubmit={() => setConfirmSubmit(true)} />}
   {questionContent}
   <ExamBottomBar isMarked={flags[view.currentIndex] ?? false} onToggleMark={() => setFlags((old) => { const next = [...old]; next[view.currentIndex] = !next[view.currentIndex]; return next; })} onOpenNavigator={() => setNavigator(true)} onPrev={() => navigate(view.currentIndex - 1)} onNext={() => navigate(view.currentIndex + 1)} canPrev={!busy && view.currentIndex > 0} canNext={!busy && view.currentIndex < manifest.length - 1} />
   <QuestionNavigatorPanel items={manifest} currentIndex={view.currentIndex} answers={answers} flagged={flags} results={results} onNavigate={navigate} isOpen={navigator} onClose={() => setNavigator(false)} />
   {confirmSubmit && <div className="fixed inset-0 z-[180] flex items-center justify-center bg-black/50 p-4"><div role="dialog" aria-modal="true" aria-label="Nộp bài" className="rounded-xl bg-white p-6 text-zinc-900"><h3>Nộp bài?</h3><p>Còn {manifest.length - answeredCount} câu chưa trả lời.</p><button className="secondary-button" onClick={() => setConfirmSubmit(false)}>Tiếp tục</button> <button className="primary-button" disabled={busy} onClick={submit}>Xác nhận nộp bài</button></div></div>}
 </section>;
}
