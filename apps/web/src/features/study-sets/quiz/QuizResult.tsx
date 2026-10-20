import type { CSSProperties } from "react";
import type { ProtectedQuizView } from "../../../lib/api/protectedQuiz";
import { ViewerWatermark } from "../../../components/learning/ViewerWatermark";
import { PART_LABELS } from "../exam/toeic";
import { formatDuration } from "./quizHelpers";

export function QuizResult({ view, onReview, onRestart }: { view: ProtectedQuizView; onReview: () => void; onRestart: () => void }) {
  const total = view.manifest.length;
  const score = view.score ?? 0;
  const percent = total ? Math.round((score / total) * 100) : 0;
  const answered = Object.keys(view.answers).length;
  const parts = [...new Set(view.manifest.map((m) => m.part))].sort((a, b) => a - b).map((part) => {
    const indices = view.manifest.flatMap((m, i) => (m.part === part ? [i] : []));
    return { part, total: indices.length, correct: indices.filter((i) => view.results[i]).length };
  });
  const verdict = percent >= 80 ? "Rất tốt!" : percent >= 50 ? "Khá ổn, còn vài câu cần xem lại." : "Cần luyện thêm — xem lại các câu sai để nhớ lâu hơn.";
  return <div className="qz-shell"><ViewerWatermark /><div className="qz-scroll"><div className="qz-result">
    <div className="qz-result-hero">
      <div className="qz-ring" style={{ "--p": percent } as CSSProperties} role="img" aria-label={`Đạt ${percent}%`}><span>{percent}%</span></div>
      <div>
        <h2>Đúng {score}/{total} câu</h2>
        <p>{verdict}</p>
        <p>Đã trả lời {answered}/{total} câu · Thời gian {formatDuration(view.timeUsed)}</p>
      </div>
    </div>
    {view.layout === "toeic" && parts.length > 0 && <section className="qz-parts" aria-label="Kết quả theo Part">
      <h3 style={{ margin: 0 }}>Kết quả theo Part</h3>
      {parts.map((p) => <div className="qz-part-row" key={p.part}>
        <span>{(PART_LABELS[p.part] ?? `Part ${p.part}`).split(":")[0]}</span>
        <div className="qz-bar"><span style={{ width: `${p.total ? (p.correct / p.total) * 100 : 0}%` }} /></div>
        <b>{p.correct}/{p.total}</b>
      </div>)}
    </section>}
    <div className="qz-result-actions">
      <button className="qz-btn primary" onClick={onReview}>Xem lại từng câu</button>
      <button className="qz-btn" onClick={onRestart}>Làm bài khác</button>
    </div>
  </div></div></div>;
}
