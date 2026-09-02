// apps/web/src/features/live/LiveGame.tsx
// Dev 4/5 - Main Live Quiz orchestrator

import React, { useCallback, useEffect, useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { liveApi, type LiveSession } from "./liveApi";
import { LiveHome } from "./LiveHome";
import { JoinLiveSession } from "./JoinLiveSession";
import { HostLobby } from "./HostLobby";
import { HostGame } from "./HostGame";
import { PlayerLobby } from "./PlayerLobby";
import { PlayerGame } from "./PlayerGame";
import { LiveLeaderboard } from "./LiveLeaderboard";
import { useLiveHostSession, useLivePlayerSession } from "./useLiveSession";
import "./live.css";

type Flow = "home" | "creating" | "host-lobby" | "host-game" | "joining" | "player-lobby" | "player-game";

export function LiveGame() {
  const { user, token } = useAuth();
  const [flow, setFlow] = useState<Flow>("home");

  // Host state
  const [hostSessionId, setHostSessionId] = useState<number | null>(null);
  const [joinCode, setJoinCode] = useState("");

  // Player state
  const [playerSessionId, setPlayerSessionId] = useState<number | null>(null);
  const [participantToken, setParticipantToken] = useState<string | null>(null);
  const [playerDisplayName, setPlayerDisplayName] = useState("");
  const [joinError, setJoinError] = useState<string | null>(null);

  // Host hooks
  const host = useLiveHostSession(token, hostSessionId);

  // Player hooks
  const player = useLivePlayerSession(participantToken, playerSessionId);

  // Handle host creating session
  const handleHostCreate = useCallback(async (studySetId: number, questionCount: number, questionDurationMs: number) => {
    if (!token) return;
    setFlow("creating");
    try {
      const session = await liveApi.createSession(token, {
        studySetId,
        questionCount,
        questionDurationMs,
      });
      setHostSessionId(session.id);
      setJoinCode(session.code);
      setFlow("host-lobby");
    } catch (err) {
      alert((err as Error).message);
      setFlow("home");
    }
  }, [token]);

  // Handle player joining
  const handleJoin = useCallback(async (code: string, displayName: string) => {
    setFlow("joining");
    setJoinError(null);
    try {
      const resp = await liveApi.joinSession(code, displayName);
      setPlayerSessionId(resp.sessionId);
      setParticipantToken(resp.participantToken);
      setPlayerDisplayName(displayName);
      if (resp.status === "LOBBY") {
        setFlow("player-lobby");
      } else {
        setFlow("player-game");
      }
    } catch (err) {
      setJoinError((err as Error).message);
      setFlow("home");
    }
  }, []);

  // Host status transitions
  useEffect(() => {
    if (host.state.status === "QUESTION_OPEN" && flow === "host-lobby") {
      setFlow("host-game");
    }
    if (host.state.status === "QUESTION_CLOSED" && flow === "host-lobby") {
      setFlow("host-game");
    }
    if (host.state.status === "QUESTION_OPEN" && flow === "host-game") {
      // Already in game view
    }
    if (host.state.status === "QUESTION_CLOSED" && flow === "host-game") {
      // Show leaderboard
    }
    if (host.state.status === "ENDED" && flow !== "home") {
      // Stay in game, show final leaderboard
    }
  }, [host.state.status, flow]);

  // Player status transitions
  useEffect(() => {
    if (player.state.status === "QUESTION_OPEN" && flow === "player-lobby") {
      setFlow("player-game");
    }
    if (player.state.status === "ENDED" && flow !== "home") {
      // Stay to show final leaderboard
    }
  }, [player.state.status, flow]);

  // Calculate current question index
  const currentQuestionIndex = host.state.currentQuestion?.index ?? player.state.currentQuestion?.index ?? 0;

  // Fetch leaderboard when question closes
  useEffect(() => {
    if (host.state.status === "QUESTION_CLOSED" && hostSessionId && token) {
      liveApi.getLeaderboard(token, hostSessionId).then((lb) => {
        // Update leaderboard via event
      }).catch(() => {});
    }
  }, [host.state.status, hostSessionId, token]);

  return (
    <div className="live-game-container">
      {/* Connection status indicator */}
      {(flow.startsWith("host") || flow.startsWith("player")) && (
        <div
          className={host.state.connected || player.state.connected ? "sse-connected" : "sse-disconnected"}
          style={{ padding: "0.5rem 1rem", fontSize: "0.8rem", color: "#666", textAlign: "right" }}
        >
          {host.state.connected || player.state.connected ? "Connected" : "Disconnected"}
        </div>
      )}

      {/* Error display */}
      {(host.error || player.error) && (
        <div style={{ padding: "0.75rem", background: "#fef2f2", color: "#991b1b", margin: "0 1rem", borderRadius: "4px" }}>
          {host.error || player.error}
        </div>
      )}

      {/* Home */}
      {flow === "home" && (
        <>
          <LiveHome
            onHost={(scId, qc, qd) => void handleHostCreate(scId, qc, qd)}
            onJoin={(code) => { setJoinError(null); }}
          />
          <JoinLiveSession
            error={joinError}
            loading={false}
            onJoin={(code, name) => void handleJoin(code, name)}
          />
        </>
      )}

      {/* Creating */}
      {flow === "creating" && (
        <div style={{ textAlign: "center", padding: "4rem" }}>
          <p>Creating session...</p>
        </div>
      )}

      {/* Host Lobby */}
      {flow === "host-lobby" && (
        <HostLobby
          joinCode={joinCode}
          sessionId={hostSessionId!}
          participants={host.state.participants}
          onStart={() => void host.startSession()}
          onRefreshParticipants={() => void host.refreshParticipants()}
          loading={host.loading}
        />
      )}

      {/* Host Game */}
      {(flow === "host-game" || (flow === "host-lobby" && host.state.status !== "LOBBY")) && (
        <HostGame
          status={host.state.status}
          currentQuestion={host.state.currentQuestion}
          participants={host.state.participants}
          leaderboard={host.state.leaderboard}
          sessionId={hostSessionId!}
          questionCount={host.state.session?.questionCount ?? 10}
          questionDurationMs={host.state.session?.questionDurationMs ?? 20000}
          currentQuestionIndex={currentQuestionIndex}
          onStart={() => void host.startSession()}
          onClose={() => void host.closeQuestion()}
          onNext={() => void host.nextQuestion()}
          onEnd={() => void host.endSession()}
        />
      )}



      {/* Player Lobby */}
      {flow === "player-lobby" && (
        <PlayerLobby displayName={playerDisplayName} sessionId={playerSessionId!} />
      )}

      {/* Player Game */}
      {flow === "player-game" && (
        <PlayerGame
          currentQuestion={player.state.currentQuestion}
          questionDurationMs={player.state.session?.questionDurationMs ?? 20000}
          questionIndex={currentQuestionIndex}
          hasSubmitted={player.state.hasSubmitted}
          status={player.state.status}
          onSubmit={(qi, si) => void player.submitAnswer(qi, si)}
        />
      )}

      {/* Leaderboard (shown for both host and player after question close or end) */}
      {(host.state.status === "QUESTION_CLOSED" || host.state.status === "LEADERBOARD" ||
        player.state.status === "QUESTION_CLOSED" || host.state.status === "ENDED" || player.state.status === "ENDED") &&
        (host.state.leaderboard.length > 0 || player.state.leaderboard.length > 0) && (
        <div style={{ maxWidth: 600, margin: "1rem auto", padding: "0 1rem" }}>
          <LiveLeaderboard
            entries={host.state.leaderboard.length > 0 ? host.state.leaderboard : player.state.leaderboard}
            highlightParticipantId={player.state.status !== "LOBBY" ? undefined : undefined}
          />
        </div>
      )}
    </div>
  );
}
