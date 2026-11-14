import { describe, expect, it } from "vitest";
import { videoPlaybackError } from "./videoPlaybackError";

describe("video playback error diagnosis", () => {
  it("does not describe a storage failure as expired access", () => {
    const message = videoPlaybackError(502, "networkError");
    expect(message).toContain("HTTP 502");
    expect(message).toContain("kho video");
    expect(message).not.toContain("hết hạn");
  });
  it("distinguishes expired authentication and revoked permission", () => {
    expect(videoPlaybackError(401, "networkError")).toContain("đăng nhập lại");
    expect(videoPlaybackError(403, "networkError")).toContain("không còn quyền");
  });
  it("distinguishes decoding and connectivity failures", () => {
    expect(videoPlaybackError(undefined, "mediaError")).toContain("giải mã");
    expect(videoPlaybackError(undefined, "networkError")).toContain("kết nối mạng");
  });
});
