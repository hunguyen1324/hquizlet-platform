// apps/web/src/features/live/PlayerLobby.tsx
// Dev 5 - [P6-FE-PLAYER-01] Player lobby screen

import React from "react";

interface PlayerLobbyProps {
  displayName: string;
  sessionId: number;
}

export function PlayerLobby({ displayName, sessionId }: PlayerLobbyProps) {
  return (
    <div className="player-lobby" style={{ maxWidth: 400, margin: "4rem auto", padding: "0 1rem", textAlign: "center" }}>
      <h1>Waiting for Host</h1>
      <div style={{ fontSize: "1.125rem", color: "#666", marginBottom: "2rem" }}>
        <p>
          Welcome, <strong>{displayName}</strong>!
        </p>
        <p>The host will start the game soon.</p>
      </div>

      <div
        style={{
          animation: "pulse 2s infinite",
          padding: "1rem",
          background: "#f0f9ff",
          borderRadius: "8px",
        }}
      >
        <p style={{ margin: 0, color: "#2563eb" }}>
          Session #{sessionId} — Waiting...
        </p>
      </div>
    </div>
  );
}
