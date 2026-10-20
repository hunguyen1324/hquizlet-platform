import type { ProtectedQuizView } from "../../../lib/api/protectedQuiz";

export function QuizNavigator({ view, answers, flags, onNavigate, onClose }: {
  view: ProtectedQuizView;
  answers: Record<number, string>;
  flags: boolean[];
  onNavigate: (index: number) => void;
  onClose: () => void;
}) {
  const graded = view.submitted || view.instant;
  return <div className="qz-overlay" onClick={onClose}>
    <aside className="qz-drawer" role="dialog" aria-modal="true" aria-label="Danh sách câu hỏi" onClick={(e) => e.stopPropagation()}>
      <header><h3>Danh sách câu hỏi</h3><button className="qz-btn" onClick={onClose}>Đóng</button></header>
      <div className="qz-grid">
        {view.manifest.map((item, i) => {
          const answered = answers[i] !== undefined;
          const state = graded && answered && view.results[i] !== undefined ? (view.results[i] ? " ok" : " bad") : answered ? " answered" : "";
          return <button key={i} className={`qz-cell${state}${flags[i] ? " flagged" : ""}${i === view.currentIndex ? " current" : ""}`}
            aria-label={`Câu ${item.number}${answered ? ", đã trả lời" : ", chưa trả lời"}${flags[i] ? ", đã đánh dấu" : ""}`}
            onClick={() => { onNavigate(i); onClose(); }}>{item.number}</button>;
        })}
      </div>
      <div className="qz-legend">
        <span><i className="a" />Đã trả lời</span>
        <span><i className="f" />Đã đánh dấu</span>
        {graded && <><span><i className="o" />Đúng</span><span><i className="b" />Sai</span></>}
      </div>
    </aside>
  </div>;
}
