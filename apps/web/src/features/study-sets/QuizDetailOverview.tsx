import React from "react";
import type { QuizQuestion, StudySet } from "../../types";
import { ApiError, quizQuestionApi } from "../../lib/api";
import { useAuth } from "../auth/AuthContext";
import { ProtectedQuizPlayer } from "./ProtectedQuizPlayer";
import { RichTextContent } from "./exam/RichTextContent";

const TYPE_LABELS: Record<string, string> = {
  multiple_choice: "Nhiều lựa chọn", true_false: "Đúng / Sai", written: "Tự luận",
  paragraph: "Đọc hiểu", sorting: "Sắp xếp",
};

function QuestionPreview({ question, number, showAnswers }: { question: QuizQuestion; number: string; showAnswers: boolean }) {
  const children = Array.isArray(question.subQuestions) ? question.subQuestions as QuizQuestion[] : [];
  const options = question.questionType === "true_false"
    ? [{ text: "true", label: "Đúng", isCorrect: question.correctAnswer === "true" }, { text: "false", label: "Sai", isCorrect: question.correctAnswer === "false" }]
    : (question.options ?? []).map(option => ({ ...option, label: option.text }));
  return <article className="qd-question">
    <p className="qd-question-meta">{number}. {TYPE_LABELS[question.questionType] ?? "Câu hỏi"}{question.timeInSeconds ? ` · ${question.timeInSeconds} giây` : ""}</p>
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

export function QuizDetailOverview({ set, onEdit, onSave, resumeSessionId, onManageSessions }: {
  set: StudySet; onEdit: () => void; onSave: () => void; resumeSessionId?: string;
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
  const previewRef = React.useRef<HTMLElement>(null);
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
      <div><button type="button" className="secondary-button" onClick={onEdit}>✎ Chỉnh sửa</button><button type="button" className="secondary-button" onClick={onSave}>Lưu vào thư mục</button></div>
      <div><button type="button" className="secondary-button" onClick={() => previewRef.current?.scrollIntoView({ behavior: "smooth" })}>Xem trước</button><button type="button" className="primary-button qd-start" onClick={() => setPlaying(true)}>▶ Bắt đầu ngay</button></div>
    </div>
    <div className="qd-layout">
      <aside className="qd-sidebar"><h2>Hoạt động với quiz</h2><div className="qd-side-card">
        <strong>Sẵn sàng thử sức?</strong><p>Luyện tập và xem đáp án sau mỗi câu, hoặc thi thử có tính giờ.</p>
        <button type="button" className="primary-button qd-start" onClick={() => setPlaying(true)}>Bắt đầu làm bài</button>
        {onManageSessions && <button type="button" className="secondary-button" onClick={() => onManageSessions(set.id)}>Quản lý phiên làm bài</button>}
      </div></aside>
      <section className="qd-preview" ref={previewRef} aria-label="Chi tiết câu hỏi">
        <header className="qd-preview-head"><span>{loading ? "Đang tải câu hỏi…" : `${questions.length || set.flashcardCount || 0} câu hỏi`}</span>
          {!restricted && <label className="qd-toggle">Hiển thị đáp án<input type="checkbox" role="switch" checked={showAnswers} onChange={event => setShowAnswers(event.target.checked)} /></label>}
        </header>
        {loading && <p className="qd-message" role="status">Đang tải chi tiết quiz…</p>}
        {restricted && <p className="qd-message">Chi tiết câu hỏi dành cho người có quyền chỉnh sửa. Nhấn “Bắt đầu ngay” để làm quiz.</p>}
        {error && <div className="qd-message" role="alert"><p>{error}</p><button type="button" className="secondary-button" onClick={() => setRetry(value => value + 1)}>Thử lại</button></div>}
        {!loading && !restricted && !error && <>
          {questions.slice(0, visible).map((question, index) => <QuestionPreview key={question.id ?? index} question={question} number={String(index + 1)} showAnswers={showAnswers} />)}
          {!questions.length && <p className="qd-message">Quiz này chưa có câu hỏi.</p>}
          {visible < questions.length && <button type="button" className="sd-load-more" onClick={() => setVisible(value => value + 50)}>Hiển thị thêm 50 câu hỏi</button>}
        </>}
      </section>
    </div>
  </>;
}
