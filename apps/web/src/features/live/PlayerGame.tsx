// apps/web/src/features/live/PlayerGame.tsx
// Dev 5 - [P6-FE-PLAYER-01] Player question/submit/wait/reveal screens

import React, { useCallback, useEffect, useRef, useState } from "react";
import type { LiveQuestion } from "./liveApi";

interface PlayerGameProps {
  currentQuestion: LiveQuestion | null;
  questionDurationMs: number;
  questionIndex: number;
  hasSubmitted: boolean;
  status: string;
  onSubmit: (questionIndex: number, selectedIndex: number) => void;
}

export function PlayerGame({ currentQuestion, questionDurationMs, questionIndex, hasSubmitted, status, onSubmit }: PlayerGameProps) {
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null);
  const [countdown, setCountdown] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const timerRef = useRef<number | null>(null);

  // Reset selection on new question
  useEffect(() => {
    setSelectedIndex(null);
    setSubmitting(false);
  }, [questionIndex]);

  // Countdown
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

  const handleSubmit = useCallback(() => {
    if (selectedIndex === null || submitting || hasSubmitted) return;
    setSubmitting(true);
    onSubmit(questionIndex, selectedIndex);
  }, [selectedIndex, submitting, hasSubmitted, onSubmit, questionIndex]);

  if (!currentQuestion) {
    return (
      <div style={{ textAlign: "center", padding: "4rem 1rem", color: "#666" }}>
        <p>Waiting for the next question...</p>
      </div>
    );
  }

  return (
    <div className="player-game" style={{ maxWidth: 600, margin: "2rem auto", padding: "0 1rem" }}>
      {/* Countdown */}
      {status === "QUESTION_OPEN" && (
        <div
          style={{
            textAlign: "center",
            fontSize: "2.5rem",
            fontWeight: "bold",
            color: countdown <= 5 ? "#ef4444" : "#333",
            marginBottom: "1rem",
          }}
          role="timer"
          aria-live="polite"
          aria-label={`${countdown} seconds remaining`}
        >
          {countdown}s
        </div>
      )}

      {/* Question text */}
      <div style={{ padding: "1.5rem", background: "#f8f9fa", borderRadius: "8px", marginBottom: "1.5rem" }}>
        <h2 style={{ margin: 0 }}>{currentQuestion.text}</h2>
      </div>

      {/* Choices */}
      <div style={{ display: "grid", gap: "0.75rem", marginBottom: "1.5rem" }}>
        {currentQuestion.choices?.map((choice, i) => (
          <button
            key={i}
            onClick={() => !hasSubmitted && setSelectedIndex(i)}
            disabled={hasSubmitted}
            style={{
              padding: "1rem",
              textAlign: "left",
              border: selectedIndex === i ? "2px solid #2563eb" : "2px solid #ddd",
              borderRadius: "8px",
              background: selectedIndex === i ? "#eff6ff" : "white",
              cursor: hasSubmitted ? "default" : "pointer",
              fontSize: "1rem",
              transition: "all 0.15s",
            }}
            aria-pressed={selectedIndex === i}
          >
            <strong style={{ marginRight: "0.5rem" }}>{String.fromCharCode(65 + i)}.</strong>
            {choice}
          </button>
        ))}
      </div>

      {/* Submit / Waiting / Ended */}
      {status === "QUESTION_OPEN" && !hasSubmitted && (
        <button
          className="primary-button"
          disabled={selectedIndex === null || submitting}
          onClick={handleSubmit}
          style={{ width: "100%", padding: "0.75rem", fontSize: "1.125rem" }}
        >
          {submitting ? "Submitting..." : "Submit Answer"}
        </button>
      )}

      {hasSubmitted && status === "QUESTION_OPEN" && (
        <div
          style={{
            textAlign: "center",
            padding: "1rem",
            background: "#f0fdf4",
            borderRadius: "8px",
            color: "#166534",
          }}
        >
          ✓ Answer submitted! Waiting for host...
        </div>
      )}

      {status === "QUESTION_CLOSED" && (
        <div
          style={{
            textAlign: "center",
            padding: "1rem",
            background: "#fff7ed",
            borderRadius: "8px",
            color: "#9a3412",
          }}
        >
          Question closed. Waiting for next question...
        </div>
      )}

      {status === "ENDED" && (
        <div
          style={{
            textAlign: "center",
            padding: "1rem",
            background: "#f5f5f5",
            borderRadius: "8px",
            color: "#666",
          }}
        >
          Game over! Check the leaderboard below.
        </div>
      )}
    </div>
  );
}
