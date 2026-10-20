// apps/web/src/features/live/LiveHome.tsx
// Host panel: create a live room from a study set.
// The player join card lives in JoinLiveSession.tsx so this screen has a single job.

import React, { useState } from "react";

interface LiveHomeProps {
  onHost: (studySetId: number, questionCount: number, questionDurationMs: number) => void;
}

export function LiveHome({ onHost }: LiveHomeProps) {
  const [studySetId, setStudySetId] = useState(0);
  const [questionCount, setQuestionCount] = useState(10);
  const [questionDuration, setQuestionDuration] = useState(20);

  return (
    <form
      className="live-host-form"
      onSubmit={(e) => {
        e.preventDefault();
        if (studySetId < 1) return;
        onHost(studySetId, questionCount, questionDuration * 1000);
      }}
    >
      <div className="live-host-field">
        <label htmlFor="live-host-set" className="live-host-label">ID bộ đề</label>
        <input
          id="live-host-set"
          type="number"
          min={1}
          value={studySetId || ""}
          onChange={(e) => setStudySetId(Number(e.target.value))}
          placeholder="Ví dụ: 128"
          className="live-host-input"
        />
        <p className="live-host-hint">Mã bộ đề nằm trong đường dẫn trang bộ đề.</p>
      </div>

      <div className="live-host-row">
        <div className="live-host-field">
          <label htmlFor="live-host-count" className="live-host-label">Số câu hỏi</label>
          <input
            id="live-host-count"
            type="number"
            min={1}
            max={100}
            value={questionCount}
            onChange={(e) => setQuestionCount(Number(e.target.value))}
            className="live-host-input"
          />
        </div>
        <div className="live-host-field">
          <label htmlFor="live-host-time" className="live-host-label">Giây mỗi câu</label>
          <input
            id="live-host-time"
            type="number"
            min={5}
            max={120}
            value={questionDuration}
            onChange={(e) => setQuestionDuration(Number(e.target.value))}
            className="live-host-input"
          />
        </div>
      </div>

      <button type="submit" className="live-host-submit" disabled={studySetId < 1}>
        Tạo phòng và lấy mã
      </button>
    </form>
  );
}
