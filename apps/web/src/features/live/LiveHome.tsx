// apps/web/src/features/live/LiveHome.tsx
// Dev 4 - [P6-FE-HOST-01] Live Quiz entry point

import React, { useState } from "react";

interface LiveHomeProps {
  onHost: (studySetId: number, questionCount: number, questionDurationMs: number) => void;
  onJoin: (code: string) => void;
}

export function LiveHome({ onHost, onJoin }: LiveHomeProps) {
  const [mode, setMode] = useState<"host" | "join">("join");
  const [joinCode, setJoinCode] = useState("");
  const [studySetId, setStudySetId] = useState(0);
  const [questionCount, setQuestionCount] = useState(10);
  const [questionDuration, setQuestionDuration] = useState(20);

  return (
    <div className="live-home" style={{ maxWidth: 480, margin: "2rem auto", padding: "0 1rem" }}>
      <h1>Live Quiz</h1>

      <div style={{ display: "flex", gap: "0.5rem", marginBottom: "1.5rem" }}>
        <button
          className={mode === "join" ? "primary-button" : "ghost-button"}
          onClick={() => setMode("join")}
        >
          Join Game
        </button>
        <button
          className={mode === "host" ? "primary-button" : "ghost-button"}
          onClick={() => setMode("host")}
        >
          Host Game
        </button>
      </div>

      {mode === "join" && (
        <div>
          <h2>Join a Game</h2>
          <div style={{ marginBottom: "1rem" }}>
            <label>Enter Code</label>
            <input
              type="text"
              maxLength={6}
              value={joinCode}
              onChange={(e) => setJoinCode(e.target.value.toUpperCase().trim())}
              placeholder="XXXXXX"
              style={{
                width: "100%",
                fontSize: "1.5rem",
                textAlign: "center",
                letterSpacing: "0.5em",
                textTransform: "uppercase",
                padding: "0.75rem",
                marginTop: "0.25rem",
              }}
            />
          </div>
          <button
            className="primary-button"
            disabled={joinCode.length !== 6}
            onClick={() => onJoin(joinCode)}
            style={{ width: "100%" }}
          >
            Join
          </button>
        </div>
      )}

      {mode === "host" && (
        <div>
          <h2>Host a Game</h2>
          <div style={{ marginBottom: "1rem" }}>
            <label>Study Set ID</label>
            <input
              type="number"
              min={1}
              value={studySetId || ""}
              onChange={(e) => setStudySetId(Number(e.target.value))}
              placeholder="Enter study set ID"
              style={{ width: "100%", padding: "0.5rem", marginTop: "0.25rem" }}
            />
          </div>
          <div style={{ marginBottom: "1rem" }}>
            <label>Number of Questions</label>
            <input
              type="number"
              min={1}
              max={100}
              value={questionCount}
              onChange={(e) => setQuestionCount(Number(e.target.value))}
              style={{ width: "100%", padding: "0.5rem", marginTop: "0.25rem" }}
            />
          </div>
          <div style={{ marginBottom: "1rem" }}>
            <label>Time per Question (seconds)</label>
            <input
              type="number"
              min={5}
              max={120}
              value={questionDuration}
              onChange={(e) => setQuestionDuration(Number(e.target.value))}
              style={{ width: "100%", padding: "0.5rem", marginTop: "0.25rem" }}
            />
          </div>
          <button
            className="primary-button"
            disabled={studySetId < 1}
            onClick={() => onHost(studySetId, questionCount, questionDuration * 1000)}
            style={{ width: "100%" }}
          >
            Create Session
          </button>
        </div>
      )}
    </div>
  );
}
