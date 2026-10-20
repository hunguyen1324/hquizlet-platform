import type { ReactNode } from "react";

import { ViewerWatermark } from "../../../components/learning/ViewerWatermark";
import { contentProtection } from "../exam/contentProtection";

/** One frame for every quiz layout: same protection, same watermark, same
 * fixed-height column so the top/bottom bars never scroll away on a phone. */
export function QuizShell({ wide = false, children }: { wide?: boolean; children: ReactNode }) {
  return (
    <section {...contentProtection} className={`qz-shell${wide ? " qz-shell--wide" : ""}`}>
      <ViewerWatermark />
      {children}
    </section>
  );
}

/** Placeholder for a page that is still loading, sized like real content to
 * avoid a layout jump when the network answers. */
export function QuizPageSkeleton() {
  return (
    <div className="qz-skeleton" role="status" aria-live="polite" aria-label="Đang tải câu hỏi">
      <span className="qz-skeleton-eyebrow" />
      <span className="qz-skeleton-line" />
      <span className="qz-skeleton-line qz-skeleton-line--short" />
      {[0, 1, 2, 3].map((i) => (
        <span key={i} className="qz-skeleton-answer" />
      ))}
    </div>
  );
}
