// ViewerWatermark — deterrent/traceability overlay, NOT an access control.
// The mark is a pseudonymous code issued by the backend for the authenticated
// viewer. The overlay never captures pointer events and is hidden from
// assistive tech. If the backend has watermarking disabled, nothing renders.

import React from "react";
import { useAuth } from "../../features/auth/AuthContext";
import { apiFetch } from "../../lib/api/client";

let cached: { token: string; mark: string } | null = null;

export function useViewerMark(): string | null {
  const { token } = useAuth();
  const [mark, setMark] = React.useState<string | null>(cached && cached.token === token ? cached.mark : null);
  React.useEffect(() => {
    if (!token) { setMark(null); return; }
    if (cached && cached.token === token) { setMark(cached.mark); return; }
    let live = true;
    apiFetch<{ mark: string }>("/v1/study-sets/viewer-mark", token)
      .then((r) => { cached = { token, mark: r.mark }; if (live) setMark(r.mark); })
      .catch(() => { if (live) setMark(null); });
    return () => { live = false; };
  }, [token]);
  return mark;
}

export function ViewerWatermark() {
  const mark = useViewerMark();
  if (!mark) return null;
  const label = `HQuizlet · ${mark}`;
  return (
    <div className="viewer-watermark" aria-hidden="true">
      {Array.from({ length: 12 }, (_, i) => (
        <span key={i}>{label}</span>
      ))}
    </div>
  );
}
