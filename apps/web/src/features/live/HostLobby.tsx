// apps/web/src/features/live/HostLobby.tsx
// Dev 4 - [P6-FE-HOST-02] Host lobby with join code and participant list

import React, { useEffect, useState } from "react";
import type { LiveParticipant } from "./liveApi";

interface HostLobbyProps {
  joinCode: string;
  sessionId: number;
  participants: LiveParticipant[];
  onStart: () => void;
  onRefreshParticipants: () => void;
  loading: boolean;
}

export function HostLobby({ joinCode, sessionId, participants, onStart, onRefreshParticipants, loading }: HostLobbyProps) {
  const [copied, setCopied] = useState(false);

  // Auto-refresh participants every 3s
  useEffect(() => {
    const id = setInterval(onRefreshParticipants, 3000);
    return () => clearInterval(id);
  }, [onRefreshParticipants]);

  function handleCopy() {
    navigator.clipboard.writeText(joinCode).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  return (
    <div className="host-lobby" style={{ maxWidth: 600, margin: "2rem auto", padding: "0 1rem" }}>
      <h1>Lobby</h1>

      <div style={{ textAlign: "center", marginBottom: "2rem" }}>
        <p style={{ fontSize: "0.875rem", color: "#666" }}>Share this code with players</p>
        <div
          style={{
            fontSize: "3rem",
            fontWeight: "bold",
            letterSpacing: "0.3em",
            fontFamily: "monospace",
            padding: "1rem",
            background: "#f0f0f0",
            borderRadius: "8px",
            cursor: "pointer",
          }}
          onClick={handleCopy}
          title="Click to copy"
        >
          {joinCode}
        </div>
        {copied && <span style={{ color: "green", fontSize: "0.875rem" }}>Copied!</span>}
      </div>

      <div style={{ marginBottom: "2rem" }}>
        <h2>
          Players ({participants.length})
        </h2>
        {participants.length === 0 ? (
          <p style={{ color: "#999" }}>Waiting for players to join...</p>
        ) : (
          <ul style={{ listStyle: "none", padding: 0 }}>
            {participants.map((p) => (
              <li
                key={p.id}
                style={{
                  padding: "0.75rem",
                  borderBottom: "1px solid #eee",
                  display: "flex",
                  justifyContent: "space-between",
                }}
              >
                <span>{p.displayName}</span>
                <span style={{ color: "#999", fontSize: "0.875rem" }}>
                  Joined {new Date(p.joinedAt).toLocaleTimeString()}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>

      <button
        className="primary-button"
        disabled={loading || participants.length === 0}
        onClick={onStart}
        style={{ width: "100%", padding: "0.75rem", fontSize: "1.125rem" }}
      >
        {loading ? "Starting..." : "Start Game"}
      </button>
    </div>
  );
}
