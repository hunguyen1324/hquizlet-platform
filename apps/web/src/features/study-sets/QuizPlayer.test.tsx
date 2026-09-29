import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { flattenQuestions } from "./QuizPlayer";
import { CustomAudioPlayer } from "./exam/ExamQuestionLayout";

describe("practice audio", () => {
  it("keeps standalone audio and inherits paragraph audio unless the child overrides it", () => {
    const result = flattenQuestions([
      { id: 1, questionType: "multiple_choice", questionText: "one", position: 0, options: [], audioUrl: "/one.mp3" },
      { id: 2, questionType: "paragraph", questionText: "passage", position: 1, options: [], audioUrl: "/parent.mp3", subQuestions: JSON.stringify([
        { questionText: "child one", options: [] },
        { questionText: "child two", options: [], audioUrl: "/child.mp3" },
      ]) },
    ]);
    expect(result.map((q) => q.audioUrl)).toEqual(["/one.mp3", "/parent.mp3", "/child.mp3"]);
  });
  it("renders a playable audio element and a play button without a download control", () => {
    const markup = renderToStaticMarkup(<CustomAudioPlayer src="/listen.mp3" muted={false} />);
    expect(markup).toContain('src="/listen.mp3"');
    expect(markup).toContain('title="Phát"');
    expect(markup).toContain('controlsList="nodownload"');
    expect(markup).not.toContain('download=');
  });
});
