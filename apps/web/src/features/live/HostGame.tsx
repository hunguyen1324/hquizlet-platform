// apps/web/src/features/live/HostGame.tsx
// Dev 4 - [P6-FE-HOST-03, P6-FE-HOST-04] Host game controls and view

import React, { useCallback, useEffect, useRef, useState } from "react";
import type { LiveQuestion, LiveSessionStatus, LeaderboardEntry, LiveParticipant } from "./liveApi";
import { LiveLeaderboard } from "./LiveLeaderboard";

interface HostGameProps {
  status: LiveSessionStatus;
  currentQuestion: LiveQuestion | null;
  participants: LiveParticipant[];
  leaderboard: LeaderboardEntry[];
  sessionId: number;
  questionCount: number;
  questionDurationMs: number;
  currentQuestionIndex: number;
  onStart: () => void;
  onClose: () => void;
  onNext: () => void;
  onEnd: () => void;
}

export function HostGame({
  status,
  currentQuestion,
  participants,
  leaderboard,
  sessionId,
  questionCount,
  questionDurationMs,
  currentQuestionIndex,
  onStart,
  onClose,
  onNext,
  onEnd,
}: HostGameProps) {
  const [countdown, setCountdown] = useState(0);
  const [confirmEnd, setConfirmEnd] = useState(false);
  const timerRef = useRef<number | null>(null);

  // Countdown based on server time
  useEffect(() => {
    if (status === "QUESTION_OPEN" && currentQuestion?.closesAt) {
      const closesAt = new Date(currentQuestion.closesAt).getTime();
      function tick() {
        const remaining = Math.max(0, Math.ceil((closesAt - Date.now()) / 1000));
        setCountdown(remaining);
        if (remaining > 0) {
          timerRef.current = window.setTimeout(tick, 250);
        }
      }
      tick();
      return () => { if (timerRef.current) clearTimeout(timerRef.current); };
    }
    setCountdown(0);
  }, [status, currentQuestion?.closesAt]);

  const handleEnd = useCallback(() => {
    if (confirmEnd) {
      onEnd();
      setConfirmEnd(false);
    } else {
      setConfirmEnd(true);
      setTimeout(() => setConfirmEnd(false), 3000);
    }
  }, [confirmEnd, onEnd]);

  return (
    <div className="host-game" style={{ maxWidth: 800, margin: "2rem auto", padding: "0 1rem" }}>
      {/* Status bar */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1rem" }}>
        <h2 style={{ margin: 0 }}>
          Question {(currentQuestionIndex ?? 0) + 1} / {questionCount}
        </h2>
        <span
          style={{
            padding: "0.25rem 0.75rem",
            borderRadius: "999px",
            fontSize: "0.875rem",
            fontWeight: "bold",
            background:
              status === "QUESTION_OPEN"
                ? "#22c55e"
                : status === "QUESTION_CLOSED"
                ? "#eab308"
                : "#6b7280",
            color: "white",
          }}
        >
          {status}
        </span>
      </div>

      {/* Question display */}
      {currentQuestion && (
        <div
          style={{
            padding: "1.5rem",
            background: "#f8f9fa",
            borderRadius: "8px",
            marginBottom: "1rem",
          }}
        >
          <h3 style={{ margin: "0 0 0.5rem" }}>{currentQuestion.text}</h3>
          {currentQuestion.choices?.length > 0 && (
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0.5rem" }}>
              {currentQuestion.choices.map((choice, i) => (
                <div
                  key={i}
                  style={{
                    padding: "0.75rem",
                    background: "white",
                    border: "1px solid #ddd",
                    borderRadius: "4px",
                  }}
                >
                  <strong>{String.fromCharCode(65 + i)}.</strong> {choice}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Countdown */}
      {status === "QUESTION_OPEN" && (
        <div
          style={{
            textAlign: "center",
            fontSize: "3rem",
            fontWeight: "bold",
            color: countdown <= 5 ? "#ef4444" : "#333",
            marginBottom: "1rem",
          }}
        >
          {countdown}s
        </div>
      )}

      {/* Controls */}
      <div style={{ display: "flex", gap: "0.5rem", marginBottom: "2rem", flexWrap: "wrap" }}>
        {status === "LOBBY" && (
          <button className="primary-button" onClick={onStart} disabled={participants.length === 0}>
            Start Game
          </button>
        )}
        {status === "QUESTION_OPEN" && (
          <button className="primary-button" onClick={onClose}>
            Close Question
          </button>
        )}
        {(status === "QUESTION_CLOSED" || status === "LEADERBOARD") && currentQuestionIndex < questionCount - 1 && (
          <button className="primary-button" onClick={onNext}>
            Next Question
          </button>
        )}
        {status !== "ENDED" && (
          <button
            className="ghost-button"
            onClick={handleEnd}
            style={{ color: confirmEnd ? "#ef4444" : undefined }}
          >
            {confirmEnd ? "Confirm End?" : "End Game"}
          </button>
        )}
      </div>

      {/* Leaderboard */}
      {(status === "QUESTION_CLOSED" || status === "LEADERBOARD" || status === "ENDED") && leaderboard.length > 0 && (
        <LiveLeaderboard entries={leaderboard} />
      )}

      {/* Participants */}
      <div style={{ marginTop: "1rem" }}>
        <h3>Players ({participants.length})</h3>
        <div style={{ display: "flex", flexWrap: "wrap", gap: "0.5rem" }}>
          {participants.map((p) => (
            <span
              key={p.id}
              style={{
                padding: "0.25rem 0.75rem",
                background: "#e5e7eb",
                borderRadius: "999px",
                fontSize: "0.875rem",
              }}
            >
              {p.displayName}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}
