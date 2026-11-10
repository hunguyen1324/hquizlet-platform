import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fetchHealth } from "./api";
import { startServiceHealthPolling } from "./useServiceHealth";

vi.mock("./api", () => ({ fetchHealth: vi.fn() }));
const request = vi.mocked(fetchHealth);
let hidden = false;
let visibility: (() => void) | undefined;
let stop: (() => void) | undefined;
beforeEach(() => {
  vi.useFakeTimers();
  hidden = false;
  vi.stubGlobal("window", globalThis);
  vi.stubGlobal("document", {
    get hidden() { return hidden; },
    addEventListener: (_: string, handler: () => void) => { visibility = handler; },
    removeEventListener: () => { visibility = undefined; },
  });
  request.mockReset().mockResolvedValue([]);
});
afterEach(() => { stop?.(); stop = undefined; vi.useRealTimers(); vi.unstubAllGlobals(); });

describe("service status polling", () => {
  it("polls once a minute and pauses for hidden tabs", async () => {
    stop = startServiceHealthPolling(vi.fn(), vi.fn());
    await vi.advanceTimersByTimeAsync(59_999);
    expect(request).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(request).toHaveBeenCalledTimes(2);
    hidden = true; visibility?.();
    await vi.advanceTimersByTimeAsync(180_000);
    expect(request).toHaveBeenCalledTimes(2);
    hidden = false; visibility?.();
    await vi.advanceTimersByTimeAsync(1);
    expect(request).toHaveBeenCalledTimes(3);
  });
  it("does not overlap slow requests and aborts on cleanup", async () => {
    request.mockImplementation(() => new Promise(() => {}));
    stop = startServiceHealthPolling(vi.fn(), vi.fn());
    await vi.advanceTimersByTimeAsync(180_000);
    expect(request).toHaveBeenCalledTimes(1);
    const signal = request.mock.calls[0][0]!;
    stop();
    expect(signal.aborted).toBe(true);
  });
  it("ignores a late response after the screen unmounts", async () => {
    let resolve!: (value: []) => void;
    request.mockImplementation(() => new Promise(done => { resolve = done; }));
    const update = vi.fn();
    stop = startServiceHealthPolling(update, vi.fn());
    stop(); resolve([]);
    await vi.advanceTimersByTimeAsync(0);
    expect(update).not.toHaveBeenCalled();
  });
});
