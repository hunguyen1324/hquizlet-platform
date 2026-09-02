// apps/web/src/features/live/liveApi.ts
// Dev 4 - [P6-FE-API-01] Shared Live Quiz API client

const gatewayUrl = import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ?? "http://localhost:8080";

// ---- Types ----

export type LiveSessionStatus = "LOBBY" | "QUESTION_OPEN" | "QUESTION_CLOSED" | "LEADERBOARD" | "ENDED";

export interface LiveSession {
  id: number;
  code: string;
  hostUserId: number;
  studySetId: number;
  status: LiveSessionStatus;
  questionCount: number;
  questionDurationMs: number;
  currentQuestionIndex?: number | null;
  stateVersion: number;
  startedAt?: string | null;
  endedAt?: string | null;
  createdAt: string;
}

export interface LiveParticipant {
  id: string;
  displayName: string;
  totalScore: number;
  correctCount: number;
  totalResponseTimeMs: number;
  joinedAt: string;
}

export interface LiveQuestion {
  index: number;
  flashcardId: number;
  text: string;
  choices: string[];
  startsAt: string;
  closesAt: string;
}

export interface CreateLiveSessionRequest {
  studySetId: number;
  questionCount: number;
  questionDurationMs: number;
  seed?: number;
}

export interface JoinLiveSessionResponse {
  sessionId: number;
  participantId: string;
  participantToken: string;
  status: LiveSessionStatus;
}

export interface SubmitAnswerRequest {
  questionIndex: number;
  answer: { selectedIndex: number };
  idempotencyKey: string;
}

export interface AnswerAccepted {
  accepted: boolean;
  questionIndex: number;
  submittedAt: string;
}

export interface LeaderboardEntry {
  rank: number;
  participantId: string;
  displayName: string;
  totalScore: number;
  correctCount: number;
  totalResponseTimeMs: number;
}

export interface LiveLeaderboard {
  sessionId: number;
  stateVersion: number;
  rankings: LeaderboardEntry[];
}

export interface LiveHostState {
  session: LiveSession;
  participants: LiveParticipant[];
  answerCount: number;
}

export interface LiveSSEEvent {
  eventId: string;
  sessionId: number;
  stateVersion: number;
  event: string;
  data: Record<string, unknown>;
  serverTime: string;
}

// ---- API Client ----

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly code?: string,
    public readonly requestId?: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const url = `${gatewayUrl}${path}`;
  const res = await fetch(url, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init.headers as Record<string, string> | undefined),
    },
  });
  if (res.status === 204) return undefined as unknown as T;
  const body = (await res.json().catch(() => ({}))) as Record<string, unknown>;
  if (!res.ok) {
    throw new ApiError(
      res.status,
      (body.message as string) ?? `Request failed ${res.status}`,
      body.code as string,
      body.requestId as string
    );
  }
  return body as T;
}

async function apiFetchAuth<T>(path: string, token: string, init: RequestInit = {}): Promise<T> {
  return apiFetch<T>(path, {
    ...init,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(init.headers as Record<string, string> | undefined),
    },
  });
}

async function apiFetchParticipant<T>(path: string, participantToken: string, init: RequestInit = {}): Promise<T> {
  return apiFetch<T>(path, {
    ...init,
    headers: {
      Authorization: `Bearer ${participantToken}`,
      ...(init.headers as Record<string, string> | undefined),
    },
  });
}

export const liveApi = {
  // Host endpoints
  createSession: (token: string, req: CreateLiveSessionRequest): Promise<LiveSession> =>
    apiFetchAuth("/v1/live-sessions", token, {
      method: "POST",
      body: JSON.stringify(req),
    }),

  getSession: (token: string, sessionId: number): Promise<LiveHostState> =>
    apiFetchAuth(`/v1/live-sessions/${sessionId}`, token),

  startSession: (token: string, sessionId: number): Promise<LiveSession> =>
    apiFetchAuth(`/v1/live-sessions/${sessionId}/start`, token, { method: "POST" }),

  closeQuestion: (token: string, sessionId: number): Promise<LiveSession> =>
    apiFetchAuth(`/v1/live-sessions/${sessionId}/questions/current/close`, token, { method: "POST" }),

  nextQuestion: (token: string, sessionId: number): Promise<LiveSession> =>
    apiFetchAuth(`/v1/live-sessions/${sessionId}/questions/next`, token, { method: "POST" }),

  endSession: (token: string, sessionId: number): Promise<LiveSession> =>
    apiFetchAuth(`/v1/live-sessions/${sessionId}/end`, token, { method: "POST" }),

  getLeaderboard: (token: string, sessionId: number): Promise<LiveLeaderboard> =>
    apiFetchAuth(`/v1/live-sessions/${sessionId}/leaderboard`, token),

  // Join (public)
  joinSession: (code: string, displayName: string): Promise<JoinLiveSessionResponse> =>
    apiFetch(`/v1/live-sessions/${code}/join`, {
      method: "POST",
      body: JSON.stringify({ displayName }),
    }),

  // Player endpoints
  submitAnswer: (participantToken: string, sessionId: number, req: SubmitAnswerRequest): Promise<AnswerAccepted> =>
    apiFetchParticipant(`/v1/live-sessions/${sessionId}/answers`, participantToken, {
      method: "POST",
      body: JSON.stringify(req),
    }),

  getPlayerState: (participantToken: string, sessionId: number): Promise<Record<string, unknown>> =>
    apiFetchParticipant(`/v1/live-sessions/${sessionId}/player-state`, participantToken),

  // SSE - uses fetch streaming since EventSource doesn't support custom headers
  openSSE: (
    tokenOrParticipantToken: string,
    sessionId: number,
    lastEventId?: string
  ): ReadableStream<LiveSSEEvent> => {
    const url = `${gatewayUrl}/v1/live-sessions/${sessionId}/events`;
    const headers: Record<string, string> = {
      Authorization: `Bearer ${tokenOrParticipantToken}`,
    };
    if (lastEventId) {
      headers["Last-Event-ID"] = lastEventId;
    }

    const responsePromise = fetch(url, { headers });

    return new ReadableStream({
      async start(controller) {
        const res = await responsePromise;
        if (!res.ok || !res.body) {
          controller.close();
          return;
        }
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        let currentEventId = "";
        let currentEventName = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() ?? "";

          for (const line of lines) {
            if (line.startsWith("id: ")) {
              currentEventId = line.slice(4).trim();
            } else if (line.startsWith("event: ")) {
              currentEventName = line.slice(7).trim();
            } else if (line.startsWith("data: ")) {
              try {
                const data = JSON.parse(line.slice(6)) as LiveSSEEvent;
                data.eventId = data.eventId || currentEventId;
                data.event = data.event || currentEventName;
                controller.enqueue(data);
              } catch {
                // skip invalid JSON
              }
            } else if (line === "") {
              // Event boundary
              currentEventId = "";
              currentEventName = "";
            }
          }
        }
        controller.close();
      },
    });
  },
};
