import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import type { ClassVideo } from "../../lib/api";
import { VideoProcessingProgress } from "./VideoProcessingProgress";

const video: ClassVideo = { id: 3, title: "Bài giảng", youtubeId: "", audience: "all", viewerIds: [], processingState: "processing" };

describe("video processing progress", () => {
  it("shows measured percentage and current phase", () => {
    const html = renderToStaticMarkup(<VideoProcessingProgress video={{ ...video, processingPercent: 47, processingStage: "encoding" }} />);
    expect(html).toContain("47%");
    expect(html).toContain('value="47"');
    expect(html).toContain("Đang mã hóa video");
  });
  it("keeps unknown progress indeterminate", () => {
    const html = renderToStaticMarkup(<VideoProcessingProgress video={{ ...video, processingPercent: null, processingStage: "encoding" }} />);
    expect(html).not.toContain("%");
    expect(html).not.toContain('value="');
  });
  it("does not show completion before the video becomes ready", () => {
    const html = renderToStaticMarkup(<VideoProcessingProgress video={{ ...video, processingPercent: 100, processingStage: "storing" }} />);
    expect(html).toContain("99%");
    expect(html).toContain("Đang lưu video");
  });
});
