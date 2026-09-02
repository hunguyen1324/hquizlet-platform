// apps/web/src/features/live/JoinLiveSession.tsx
// Dev 5 - [P6-FE-JOIN-01] Player join code + display name flow

import React, { useState } from "react";

interface JoinLiveSessionProps {
  initialCode?: string;
  onJoin: (code: string, displayName: string) => void;
  error?: string | null;
  loading: boolean;
}

export function JoinLiveSession({ initialCode, onJoin, error, loading }: JoinLiveSessionProps) {
  const [code, setCode] = useState(initialCode?.toUpperCase() ?? "");
  const [displayName, setDisplayName] = useState("");

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (code.length === 6 && displayName.trim()) {
      onJoin(code.toUpperCase(), displayName.trim());
    }
  }

  return (
    <div className="join-session" style={{ maxWidth: 400, margin: "2rem auto", padding: "0 1rem" }}>
      <h1>Join Live Quiz</h1>

      {error && (
        <div style={{ padding: "0.75rem", background: "#fef2f2", color: "#991b1b", borderRadius: "4px", marginBottom: "1rem" }}>
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit}>
        <div style={{ marginBottom: "1rem" }}>
          <label style={{ display: "block", marginBottom: "0.25rem" }}>Join Code</label>
          <input
            type="text"
            maxLength={6}
            value={code}
            onChange={(e) => setCode(e.target.value.toUpperCase().replace(/[^A-Z2-9]/g, ""))}
            placeholder="XXXXXX"
            style={{
              width: "100%",
              fontSize: "1.5rem",
              textAlign: "center",
              letterSpacing: "0.5em",
              textTransform: "uppercase",
              padding: "0.75rem",
            }}
            autoFocus
          />
        </div>

        <div style={{ marginBottom: "1.5rem" }}>
          <label style={{ display: "block", marginBottom: "0.25rem" }}>Display Name</label>
          <input
            type="text"
            maxLength={40}
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Your name"
            style={{ width: "100%", padding: "0.75rem" }}
          />
        </div>

        <button
          type="submit"
          className="primary-button"
          disabled={code.length !== 6 || !displayName.trim() || loading}
          style={{ width: "100%", padding: "0.75rem" }}
        >
          {loading ? "Joining..." : "Join Game"}
        </button>
      </form>
    </div>
  );
}
