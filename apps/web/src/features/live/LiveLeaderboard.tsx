// apps/web/src/features/live/LiveLeaderboard.tsx
// Dev 5 - [P6-FE-LB-01] Realtime/final leaderboard component

import React from "react";
import type { LeaderboardEntry } from "./liveApi";

interface LiveLeaderboardProps {
  entries: LeaderboardEntry[];
  highlightParticipantId?: string;
  title?: string;
}

export function LiveLeaderboard({ entries, highlightParticipantId, title }: LiveLeaderboardProps) {
  if (!entries || entries.length === 0) {
    return null;
  }

  return (
    <div className="live-leaderboard">
      <h3 style={{ margin: "0 0 0.75rem" }}>{title ?? "Leaderboard"}</h3>
      <table
        style={{
          width: "100%",
          borderCollapse: "collapse",
          fontSize: "0.9375rem",
        }}
      >
        <thead>
          <tr style={{ borderBottom: "2px solid #ddd", textAlign: "left" }}>
            <th style={{ padding: "0.5rem" }}>#</th>
            <th style={{ padding: "0.5rem" }}>Player</th>
            <th style={{ padding: "0.5rem", textAlign: "right" }}>Score</th>
            <th style={{ padding: "0.5rem", textAlign: "right" }}>Correct</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry) => (
            <tr
              key={entry.participantId}
              style={{
                borderBottom: "1px solid #eee",
                background: entry.participantId === highlightParticipantId ? "#eff6ff" : "transparent",
                fontWeight: entry.participantId === highlightParticipantId ? "bold" : "normal",
              }}
            >
              <td style={{ padding: "0.5rem" }}>
                {entry.rank <= 3 ? (
                  <span style={{ fontSize: "1.125rem" }}>
                    {entry.rank === 1 ? "🥇" : entry.rank === 2 ? "🥈" : "🥉"}
                  </span>
                ) : (
                  entry.rank
                )}
              </td>
              <td style={{ padding: "0.5rem" }}>
                {entry.displayName}
                {entry.participantId === highlightParticipantId && (
                  <span style={{ marginLeft: "0.5rem", color: "#2563eb", fontSize: "0.8rem" }}>(You)</span>
                )}
              </td>
              <td style={{ padding: "0.5rem", textAlign: "right", fontFamily: "monospace" }}>
                {entry.totalScore.toLocaleString()}
              </td>
              <td style={{ padding: "0.5rem", textAlign: "right" }}>
                {entry.correctCount}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
