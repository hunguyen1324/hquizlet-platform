import { afterEach, describe, expect, it, vi } from "vitest";
import { classVideoApi } from "./client";

afterEach(() => vi.unstubAllGlobals());
const reply = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
const input = { title: "lll", url: "", audience: "all" as const, viewerIds: [] };

describe("chunked video upload", () => {
  it("sends bounded chunks in order and reports progress", async () => {
    const fetcher = vi.fn().mockResolvedValueOnce(reply({ uploadId: "session", chunkSize: 4 }))
      .mockResolvedValueOnce(reply({ received: 4 })).mockResolvedValueOnce(reply({ received: 8 }))
      .mockResolvedValueOnce(reply({ received: 10 })).mockResolvedValueOnce(reply({ id: 7 }, 202));
    vi.stubGlobal("fetch", fetcher);
    const progress = vi.fn();
    expect(await classVideoApi.upload("token", 1, input, new File(["0123456789"], "test.mp4"), progress)).toEqual({ id: 7 });
    expect(fetcher.mock.calls.slice(1, 4).map(([url, init]) => [new URL(url).searchParams.get("offset"), init.body.size])).toEqual([["0", 4], ["4", 4], ["8", 2]]);
    expect(progress.mock.calls.map(([value]) => value)).toEqual([40, 80, 100]);
    expect(fetcher.mock.calls[4][0]).toContain("/session/complete");
  });

  it("does not resend a chunk whose response was lost after commit", async () => {
    vi.useFakeTimers();
    try {
      const fetcher = vi.fn().mockResolvedValueOnce(reply({ uploadId: "session", chunkSize: 4 }))
        .mockRejectedValueOnce(new TypeError("network disconnected")).mockResolvedValueOnce(reply({ received: 4 }))
        .mockResolvedValueOnce(reply({ id: 8 }, 202));
      vi.stubGlobal("fetch", fetcher);
      const result = classVideoApi.upload("token", 1, input, new File(["1234"], "test.mp4"));
      await vi.runAllTimersAsync();
      expect(await result).toEqual({ id: 8 });
      expect(fetcher.mock.calls.filter(([, init]) => init?.method === "PUT")).toHaveLength(1);
    } finally { vi.useRealTimers(); }
  });

  it("rejects videos over 1 GiB before making a network request", async () => {
    const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
    await expect(classVideoApi.upload("token", 1, input, { size: 1024 ** 3 + 1 } as File)).rejects.toThrow("1 GiB");
    expect(fetcher).not.toHaveBeenCalled();
  });
});
