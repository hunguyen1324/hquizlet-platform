import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchProgressSummary, makeIdempotencyKey, saveProgress } from "./progressContract";

afterEach(() => vi.unstubAllGlobals());

describe("progress API contract", () => {
  it("sends flashcardId and never emits the obsolete cardId", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      id: 1, mode: "learn", score: 1, total: 1,
      completedAt: "2026-09-01T00:01:00Z", createdAt: "2026-09-01T00:01:00Z",
    }), { status: 201, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    const result = await saveProgress("token", 7, {
      mode: "learn", score: 1, total: 1,
      startedAt: "2026-09-01T00:00:00Z", completedAt: "2026-09-01T00:01:00Z",
      idempotencyKey: "7:learn:2026-09-01T00:00:00Z",
      cardResults: [{ flashcardId: 42, correct: true, attempts: 2 }],
    });

    expect(result.ok).toBe(true);
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body.cardResults[0]).toMatchObject({ flashcardId: 42, attempts: 2 });
    expect(body.cardResults[0]).not.toHaveProperty("cardId");
  });

  it("reads backend history contract and sends per_page", async () => {
    const response = { studySetId: 7, totalSessions: 1, bestScore: 8, lastMode: "test",
      history: [], page: 2, perPage: 10, totalPages: 1 };
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(response), {
      status: 200, headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(fetchProgressSummary("token", 7, 2, 10)).resolves.toEqual(response);
    expect(fetchMock.mock.calls[0][0]).toContain("page=2&per_page=10");
  });

  it("changes idempotency key when a restarted session has a new start time", () => {
    expect(makeIdempotencyKey(7, "match", "2026-09-01T00:00:00Z"))
      .not.toBe(makeIdempotencyKey(7, "match", "2026-09-01T00:01:00Z"));
  });
});
