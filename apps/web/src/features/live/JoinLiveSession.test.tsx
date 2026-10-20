import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { JoinLiveSession, describeJoinError, sanitizeJoinCode } from "./JoinLiveSession";

const noop = () => {};

describe("sanitizeJoinCode", () => {
  it("uppercases, drops separators and caps at six characters", () => {
    expect(sanitizeJoinCode("ab c-12")).toBe("ABC12");
    expect(sanitizeJoinCode("abc1239")).toBe("ABC123");
    expect(sanitizeJoinCode("")).toBe("");
  });
});

describe("describeJoinError", () => {
  it("points a wrong code at the six characters", () => {
    const hint = describeJoinError({ status: 404, code: "LIVE_SESSION_NOT_FOUND", message: "live session not found" });
    expect(hint).toContain("6 ký tự");
    expect(hint).toContain("Không tìm thấy phòng");
  });

  it("explains an already-started or finished room", () => {
    expect(describeJoinError({ status: 409, code: "LIVE_INVALID_STATE" })).toContain("đã bắt đầu");
    expect(describeJoinError({ status: 410, code: "LIVE_SESSION_ENDED" })).toContain("đã kết thúc");
  });

  it("asks the player to slow down when rate limited", () => {
    expect(describeJoinError({ status: 429, code: "RATE_LIMITED" })).toContain("quá nhanh");
  });

  it("falls back to the server message, then to a retry hint", () => {
    expect(describeJoinError(new Error("Tên này đã có người dùng"))).toContain("Tên này");
    expect(describeJoinError(null)).toContain("Không vào được phòng");
  });
});

describe("JoinLiveSession", () => {
  it("asks for the code first and prefills a deep link", () => {
    const markup = renderToStaticMarkup(
      <JoinLiveSession onJoin={noop} loading={false} initialCode="abc123" />,
    );
    expect(markup).toContain("Mã phòng");
    expect(markup).toContain('value="ABC123"');
    expect(markup).toContain("Tên của bạn");
    // Submit stays disabled until a name is provided.
    expect(markup).toContain("disabled");
  });

  it("announces failures with role=alert and marks the field invalid", () => {
    const markup = renderToStaticMarkup(
      <JoinLiveSession onJoin={noop} loading={false} error="Không tìm thấy phòng với mã này." />,
    );
    expect(markup).toContain('role="alert"');
    expect(markup).toContain("Không tìm thấy phòng với mã này.");
    expect(markup).toContain('aria-invalid="true"');
  });

  it("shows that the request is in flight", () => {
    const markup = renderToStaticMarkup(
      <JoinLiveSession onJoin={noop} loading initialCode="ABC123" />,
    );
    expect(markup).toContain("Đang vào phòng…");
  });
});
