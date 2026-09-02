// apps/web/src/features/live/useLiveSession.ts
// Dev 4/5 - [P6-FE-API-01] Shared live session state hook

import { useCallback, useEffect, useRef, useState } from "react";
import { liveApi, type LiveSession, type LiveParticipant, type LiveQuestion, type LiveSessionStatus, type LeaderboardEntry } from "./liveApi";

export interface LiveSessionState {
  status: LiveSessionStatus;
  session: LiveSession | null;
  participants: LiveParticipant[];
  currentQuestion: LiveQuestion | null;
  hasSubmitted: boolean;
  leaderboard: LeaderboardEntry[];
  connected: boolean;
}

export function useLiveHostSession(token: string | null, sessionId: number | null) {
  const [state, setState] = useState<LiveSessionState>({
    status: "LOBBY",
    session: null,
    participants: [],
    currentQuestion: null,
    hasSubmitted: false,
    leaderboard: [],
    connected: false,
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const eventRef = useRef<ReadableStreamDefaultReader | null>(null);
  const lastEventIdRef = useRef<string>("");

  // Fetch initial state
  useEffect(() => {
    if (!token || !sessionId) return;
    setLoading(true);
    liveApi.getSession(token, sessionId)
      .then((data) => {
        setState((s) => ({
          ...s,
          session: data.session,
          participants: data.participants ?? [],
          status: data.session.status,
        }));
        setLoading(false);
      })
      .catch((err) => {
        setError(err.message);
        setLoading(false);
      });
  }, [token, sessionId]);

  // SSE stream
  useEffect(() => {
    if (!token || !sessionId) return;
    let cancelled = false;
    let reader: ReadableStreamDefaultReader | null = null;

    void (async () => {
      while (!cancelled) {
        try {
          const stream = liveApi.openSSE(token, sessionId, lastEventIdRef.current);
          reader = stream.getReader();
          eventRef.current = reader;
          setState((s) => ({ ...s, connected: true }));
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          if (value) {
            lastEventIdRef.current = value.eventId;
            handleHostEvent(value.event, value.data);
          }
        }
        } catch {
          // Reconnect below with Last-Event-ID.
        }
        if (cancelled) break;
        setState((s) => ({ ...s, connected: false }));
        await new Promise((resolve) => window.setTimeout(resolve, 1000));
      }
    })();

    return () => {
      cancelled = true;
      void reader?.cancel();
    };
  }, [token, sessionId]);

  function handleHostEvent(eventName: string, data: Record<string, unknown>) {
    switch (eventName) {
      case "session.snapshot": {
        const session = data.session as LiveSession | undefined;
        if (session) setState((s) => ({ ...s, session, status: session.status }));
        break;
      }
      case "session.started":
        setState((s) => ({ ...s, status: "QUESTION_OPEN" }));
        break;
      case "question.opened":
        setState((s) => ({
          ...s,
          status: "QUESTION_OPEN",
          currentQuestion: data as unknown as LiveQuestion,
          hasSubmitted: false,
        }));
        break;
      case "question.closed":
        setState((s) => ({ ...s, status: "QUESTION_CLOSED", hasSubmitted: false }));
        break;
      case "participant.joined":
        setState((s) => ({
          ...s,
          participants: [...s.participants, data as unknown as LiveParticipant],
        }));
        break;
      case "session.ended":
        setState((s) => ({ ...s, status: "ENDED" }));
        break;
      case "leaderboard.updated":
        if (data.rankings) {
          setState((s) => ({ ...s, leaderboard: data.rankings as LeaderboardEntry[] }));
        }
        break;
    }
  }

  const startSession = useCallback(async () => {
    if (!token || !sessionId) return;
    try {
      await liveApi.startSession(token, sessionId);
    } catch (err: unknown) {
      setError((err as Error).message);
    }
  }, [token, sessionId]);

  const closeQuestion = useCallback(async () => {
    if (!token || !sessionId) return;
    try {
      await liveApi.closeQuestion(token, sessionId);
    } catch (err: unknown) {
      setError((err as Error).message);
    }
  }, [token, sessionId]);

  const nextQuestion = useCallback(async () => {
    if (!token || !sessionId) return;
    try {
      await liveApi.nextQuestion(token, sessionId);
    } catch (err: unknown) {
      setError((err as Error).message);
    }
  }, [token, sessionId]);

  const endSession = useCallback(async () => {
    if (!token || !sessionId) return;
    try {
      await liveApi.endSession(token, sessionId);
      // Fetch leaderboard
      const lb = await liveApi.getLeaderboard(token, sessionId);
      setState((s) => ({ ...s, leaderboard: lb.rankings, status: "ENDED" }));
    } catch (err: unknown) {
      setError((err as Error).message);
    }
  }, [token, sessionId]);

  const refreshParticipants = useCallback(async () => {
    if (!token || !sessionId) return;
    try {
      const data = await liveApi.getSession(token, sessionId);
      setState((s) => ({
        ...s,
        participants: data.participants ?? [],
      }));
    } catch { /* ignore */ }
  }, [token, sessionId]);

  return { state, error, loading, startSession, closeQuestion, nextQuestion, endSession, refreshParticipants };
}

export function useLivePlayerSession(participantToken: string | null, sessionId: number | null) {
  const [state, setState] = useState<LiveSessionState>({
    status: "LOBBY",
    session: null,
    participants: [],
    currentQuestion: null,
    hasSubmitted: false,
    leaderboard: [],
    connected: false,
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const lastEventIdRef = useRef<string>("");
  const answerKeysRef = useRef<Map<number, string>>(new Map());

  // SSE stream
  useEffect(() => {
    if (!participantToken || !sessionId) return;
    let cancelled = false;
    let reader: ReadableStreamDefaultReader | null = null;

    void (async () => {
      while (!cancelled) {
        try {
          const stream = liveApi.openSSE(participantToken, sessionId, lastEventIdRef.current);
          reader = stream.getReader();
          setState((s) => ({ ...s, connected: true }));
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          if (value) {
            lastEventIdRef.current = value.eventId;
            handlePlayerEvent(value.event, value.data);
          }
        }
        } catch {
          // Reconnect below with Last-Event-ID.
        }
        if (cancelled) break;
        setState((s) => ({ ...s, connected: false }));
        await new Promise((resolve) => window.setTimeout(resolve, 1000));
      }
    })();

    return () => {
      cancelled = true;
      void reader?.cancel();
    };
  }, [participantToken, sessionId]);

  function handlePlayerEvent(eventName: string, data: Record<string, unknown>) {
    switch (eventName) {
      case "session.snapshot":
        {
          const session = data.session as LiveSession | undefined;
          if (session) setState((s) => ({ ...s, session, status: session.status }));
        }
        break;
      case "session.started":
        setState((s) => ({ ...s, status: "QUESTION_OPEN" }));
        break;
      case "question.opened":
        setState((s) => ({
          ...s,
          status: "QUESTION_OPEN",
          currentQuestion: data as unknown as LiveQuestion,
          hasSubmitted: false,
        }));
        break;
      case "answer.accepted":
        setState((s) => ({ ...s, hasSubmitted: true }));
        break;
      case "question.closed":
        setState((s) => ({ ...s, status: "QUESTION_CLOSED" }));
        break;
      case "leaderboard.updated":
        if (data.rankings) {
          setState((s) => ({ ...s, leaderboard: data.rankings as LeaderboardEntry[] }));
        }
        break;
      case "session.ended":
        setState((s) => ({ ...s, status: "ENDED" }));
        break;
    }
  }

  const submitAnswer = useCallback(async (questionIndex: number, selectedIndex: number) => {
    if (!participantToken || !sessionId) return;
    let idempotencyKey = answerKeysRef.current.get(questionIndex);
    if (!idempotencyKey) {
      idempotencyKey = globalThis.crypto?.randomUUID?.() ?? `${sessionId}-${questionIndex}-${Date.now()}`;
      answerKeysRef.current.set(questionIndex, idempotencyKey);
    }
    try {
      await liveApi.submitAnswer(participantToken, sessionId, {
        questionIndex,
        answer: { selectedIndex },
        idempotencyKey,
      });
      setState((s) => ({ ...s, hasSubmitted: true }));
    } catch (err: unknown) {
      setError((err as Error).message);
    }
  }, [participantToken, sessionId]);

  const fetchLeaderboard = useCallback(async () => {
    if (!participantToken || !sessionId) return;
    try {
      // Player doesn't have leaderboard endpoint auth, but try
      const res = await fetch(`${import.meta.env.VITE_GATEWAY_URL ?? "http://localhost:8080"}/v1/live-sessions/${sessionId}/leaderboard`, {
        headers: { Authorization: `Bearer ${participantToken}` },
      });
      if (res.ok) {
        const lb = await res.json();
        setState((s) => ({ ...s, leaderboard: lb.rankings ?? [] }));
      }
    } catch { /* ignore */ }
  }, [participantToken, sessionId]);

  return { state, error, loading, submitAnswer, fetchLeaderboard };
}
