import React from "react";
import type { QuizQuestion, StudySet } from "../../types";
import { ApiError, quizQuestionApi } from "../../lib/api";
import { useAuth } from "../auth/AuthContext";
import { ProtectedQuizPlayer } from "./ProtectedQuizPlayer";
import { StudyActionIcon } from "./StudyActionIcon";
import { RichTextContent } from "./exam/RichTextContent";

const TYPE_LABELS: Record<string, string> = {
  multiple_choice: "Nhiều lựa chọn", true_false: "Đúng / Sai", written: "Tự luận",
  paragraph: "Đọc hiểu", sorting: "Sắp xếp",
};

function QuestionPreview({ question, number, showAnswers, onEdit }: { question: QuizQuestion; number: string; showAnswers: boolean; onEdit?: () => void }) {
  const children = Array.isArray(question.subQuestions) ? question.subQuestions as QuizQuestion[] : [];
  const options = question.questionType === "true_false"
    ? [{ text: "true", label: "Đúng", isCorrect: question.correctAnswer === "true" }, { text: "false", label: "Sai", isCorrect: question.correctAnswer === "false" }]
    : (question.options ?? []).map(option => ({ ...option, label: option.text }));
  return <article className="qd-question">
    <header className="qd-question-head"><p className="qd-question-meta">{number}. {TYPE_LABELS[question.questionType] ?? "Câu hỏi"}{question.timeInSeconds ? ` · ${question.timeInSeconds} giây` : ""}</p>{onEdit && <button type="button" className="qd-edit-link" onClick={onEdit} aria-label={`Chỉnh sửa câu ${number}`}><StudyActionIcon name="edit" /> Chỉnh sửa</button>}</header>
    {question.paragraphText && <RichTextContent value={question.paragraphText} className="qd-passage" />}
    <RichTextContent value={question.questionText} className="qd-question-title" />
    {question.imageUrl && <img className="qd-question-image" src={question.imageUrl} alt="Hình ảnh câu hỏi" loading="lazy" />}
    <div className="qd-options">{options.map((option, i) => {
      const correct = showAnswers && (option.isCorrect || option.text === question.correctAnswer);
      return <div key={i} className={`qd-option${correct ? " qd-option--correct" : ""}`}>
        <span className="qd-option-mark" aria-label={correct ? "Đáp án đúng" : undefined}>{correct ? "✓" : ""}</span>
        <RichTextContent value={option.label} />
      </div>;
    })}</div>
    {showAnswers && question.correctAnswer && !options.length && <div className="qd-answer"><strong>Đáp án: </strong><RichTextContent value={question.correctAnswer} /></div>}
    {showAnswers && question.answerExplanation && <details className="qd-explanation"><summary>Xem giải thích câu trả lời</summary><RichTextContent value={question.answerExplanation} /></details>}
    {children.map((child, index) => <QuestionPreview key={child.id ?? index} question={child} number={`${number}.${index + 1}`} showAnswers={showAnswers} />)}
  </article>;
}

function QuizPreviewDialog({ questions, onClose }: { questions: QuizQuestion[]; onClose: () => void }) {
  const dialog = React.useRef<HTMLDialogElement>(null);
  const [index, setIndex] = React.useState(0);
  const [showAnswer, setShowAnswer] = React.useState(false);
  const [showExplanation, setShowExplanation] = React.useState(false);
  React.useEffect(() => {
    const element = dialog.current;
    if (element && !element.open) element.showModal();
    return () => { if (element?.open) element.close(); };
  }, []);
  const items = questions.flatMap(question => {
    const children = Array.isArray(question.subQuestions) ? question.subQuestions as QuizQuestion[] : [];
    return children.length ? children.map(child => ({
      ...child, paragraphText: child.paragraphText ?? question.paragraphText ?? question.questionText,
      imageUrl: child.imageUrl ?? question.imageUrl, audioUrl: child.audioUrl ?? question.audioUrl,
    })) : [question];
  });
  const question = items[index];
  if (!question) return null;
  const options = question.questionType === "true_false"
    ? [{ text: "true", label: "Đúng", isCorrect: false }, { text: "false", label: "Sai", isCorrect: false }]
    : (question.options ?? []).map(option => ({ ...option, label: option.text }));
  const answer = question.correctAnswer;
  const correctIndex = options.findIndex(option => option.isCorrect === true);
  const textIndex = options.findIndex(option => option.text === answer);
  const letterIndex = answer && /^[A-Z]$/i.test(answer.trim()) ? answer.trim().toUpperCase().charCodeAt(0) - 65 : -1;
  const resolvedIndex = correctIndex >= 0 ? correctIndex : letterIndex >= 0 && letterIndex < options.length ? letterIndex : textIndex;
  function goTo(next: number) {
    if (next < 0 || next >= items.length) return;
    setIndex(next);
    setShowAnswer(false);
    setShowExplanation(false);
  }
  return <dialog ref={dialog} className="qd-play-preview" aria-label="Xem trước quiz"
    onCancel={event => { event.preventDefault(); onClose(); }}
    onKeyDown={event => {
      if (event.target instanceof HTMLMediaElement) return;
      if (event.key === "ArrowLeft") goTo(index - 1);
      if (event.key === "ArrowRight") goTo(index + 1);
    }}>
    <div className="qd-play-frame">
      <header className="qd-play-head">
        <div><strong>Xem trước quiz</strong><span>{TYPE_LABELS[question.questionType] ?? "Câu hỏi"}</span></div>
        <span aria-live="polite">Câu {index + 1} / {items.length}</span>
        <button type="button" className="qz-btn" onClick={onClose} aria-label="Đóng xem trước">✕</button>
      </header>
      <div key={index} className="qd-play-content">
        <section className="qd-play-prompt">
          {question.paragraphText && <RichTextContent value={question.paragraphText} />}
          <RichTextContent value={question.questionText} />
          {question.imageUrl && <img src={question.imageUrl} alt="Hình minh họa câu hỏi" />}
          {question.audioUrl && !question.audioUrl.startsWith("storage:") && /^https?:\/\//i.test(question.audioUrl) &&
            <audio controls preload="none" src={question.audioUrl} aria-label="Âm thanh câu hỏi" />}
        </section>
        {!!options.length && <div className="qd-play-options" data-four={options.length === 4}>
          {options.map((option, optionIndex) => {
            const correct = showAnswer && (question.questionType === "sorting" || optionIndex === resolvedIndex);
            return <div key={optionIndex} className={correct ? "qd-play-correct" : undefined}>
              <span className="qd-play-letter">{question.questionType === "sorting" ? optionIndex + 1 : String.fromCharCode(65 + optionIndex)}</span>
              <RichTextContent value={option.label} />
              {correct && <span className="qd-play-correct-label">{question.questionType === "sorting" ? "Đúng thứ tự" : "✓ Đáp án đúng"}</span>}
            </div>;
          })}
        </div>}
        {showAnswer && !options.length && <section className="qd-play-feedback"><h3>Đáp án</h3><RichTextContent value={answer} />{!answer && <p>Câu này chưa có đáp án.</p>}</section>}
        {showExplanation && <section className="qd-play-feedback"><h3>Giải thích</h3><RichTextContent value={question.answerExplanation} /></section>}
      </div>
      <footer className="qd-play-footer">
        <button type="button" className="qz-btn qz-btn--ghost" onClick={() => goTo(index - 1)} disabled={index === 0}>‹ Trước</button>
        <div>
          <button type="button" className="qz-btn qz-btn--primary" aria-pressed={showAnswer} onClick={() => setShowAnswer(value => !value)}>{showAnswer ? "Ẩn đáp án" : "Hiện đáp án"}</button>
          {question.answerExplanation && <button type="button" className="qz-btn qz-btn--ghost" aria-pressed={showExplanation} onClick={() => setShowExplanation(value => !value)}>Giải thích</button>}
        </div>
        <button type="button" className="qz-btn qz-btn--ghost" onClick={() => goTo(index + 1)} disabled={index === items.length - 1}>Tiếp ›</button>
      </footer>
    </div>
  </dialog>;
}

export function QuizDetailOverview({ set, onEdit, onSave, resumeSessionId, onManageSessions }: {
  set: StudySet; onEdit: (questionId?: number) => void; onSave: () => void; resumeSessionId?: string;
  onManageSessions?: (setId: number) => void;
}) {
  const { token } = useAuth();
  const [questions, setQuestions] = React.useState<QuizQuestion[]>([]);
  const [showAnswers, setShowAnswers] = React.useState(true);
  const [playing, setPlaying] = React.useState(Boolean(resumeSessionId));
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const [restricted, setRestricted] = React.useState(false);
  const [retry, setRetry] = React.useState(0);
  const [visible, setVisible] = React.useState(50);
  const [previewOpen, setPreviewOpen] = React.useState(false);
  React.useEffect(() => {
    let active = true;
    setLoading(true); setError(""); setRestricted(false);
    quizQuestionApi.list(token, set.id).then(items => { if (active) setQuestions(items); })
      .catch(reason => { if (active) {
        if (reason instanceof ApiError && reason.status === 403) setRestricted(true);
        else setError(reason instanceof Error ? reason.message : "Không tải được câu hỏi.");
      } }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [token, set.id, retry]);
  if (playing) return <div>
    <button type="button" className="sd-back" onClick={() => setPlaying(false)}>← Chi tiết quiz</button>
    <ProtectedQuizPlayer studySetId={set.id} resumeSessionId={resumeSessionId} onManageSessions={onManageSessions} />
  </div>;
  return <>
    <div className="qd-toolbar">
      <div><button type="button" className="qd-edit-link" onClick={() => onEdit()}><StudyActionIcon name="edit" /> Chỉnh sửa</button><button type="button" className="secondary-button" onClick={onSave}>Lưu vào thư mục</button></div>
      <div><button type="button" className="secondary-button" disabled={loading || restricted || !!error || !questions.length} onClick={() => setPreviewOpen(true)}>Xem trước</button><button type="button" className="primary-button qd-start" onClick={() => setPlaying(true)}>▶ Bắt đầu ngay</button></div>
    </div>
    <div className="qd-layout">
      <aside className="qd-sidebar" aria-labelledby="qd-activity-title"><h2 id="qd-activity-title">Hoạt động với quiz</h2><div className="qd-side-card">
        <strong>Sẵn sàng thử sức?</strong><p>Luyện tập và xem đáp án sau mỗi câu, hoặc thi thử có tính giờ.</p>
        <button type="button" className="primary-button qd-start" onClick={() => setPlaying(true)}>Bắt đầu làm bài</button>
        {onManageSessions && <button type="button" className="secondary-button" onClick={() => onManageSessions(set.id)}>Quản lý phiên làm bài</button>}
      </div></aside>
      <section className="qd-preview" aria-label="Chi tiết câu hỏi">
        <header className="qd-preview-head"><span>{loading ? "Đang tải câu hỏi…" : `${questions.length || set.flashcardCount || 0} câu hỏi`}</span>
          {!restricted && <label className="qd-toggle">Hiển thị đáp án<input type="checkbox" role="switch" checked={showAnswers} onChange={event => setShowAnswers(event.target.checked)} /></label>}
        </header>
        {loading && <p className="qd-message" role="status">Đang tải chi tiết quiz…</p>}
        {restricted && <p className="qd-message">Chi tiết câu hỏi dành cho người có quyền chỉnh sửa. Nhấn “Bắt đầu ngay” để làm quiz.</p>}
        {error && <div className="qd-message" role="alert"><p>{error}</p><button type="button" className="secondary-button" onClick={() => setRetry(value => value + 1)}>Thử lại</button></div>}
        {!loading && !restricted && !error && <>
          {questions.slice(0, visible).map((question, index) => <QuestionPreview key={question.id ?? index} question={question} number={String(index + 1)} showAnswers={showAnswers} onEdit={question.id ? () => onEdit(question.id) : undefined} />)}
          {!questions.length && <p className="qd-message">Quiz này chưa có câu hỏi.</p>}
          {visible < questions.length && <button type="button" className="sd-load-more" onClick={() => setVisible(value => value + 50)}>Hiển thị thêm 50 câu hỏi</button>}
        </>}
      </section>
    </div>
    {previewOpen && <QuizPreviewDialog questions={questions} onClose={() => setPreviewOpen(false)} />}
  </>;
}
