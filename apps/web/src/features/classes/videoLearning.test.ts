import { describe, it, expect } from 'vitest';
import { parseLearningSubtitles, cueAtTime, translationForCue, exportLearningSRT, makeCloze, normalizePracticeAnswer } from './videoLearning';
describe('YouTube learning subtitles', () => {
    it('imports SRT and WebVTT with IDs, settings, BOM and safe text', () => {
        const srt = '\uFEFF1\r\n00:00:01,000 --> 00:00:03,500\r\n<b>Hello &amp; world</b>\r\n';
        expect(parseLearningSubtitles(srt)).toEqual([{ start: 1, end: 3.5, text: 'Hello & world' }]);
        const vtt = 'WEBVTT\n\nNOTE author note\nignored\n\ncue-id\n00:01.000 --> 00:03.500 align:start position:0%\nこんにちは\n';
        expect(parseLearningSubtitles(vtt)).toEqual([{ start: 1, end: 3.5, text: 'こんにちは' }]);
    });
    it('rejects malformed timestamps, reversed ranges, empty and oversized files', () => { for (const raw of ['plain text', '1\n00:00:03,000 --> 00:00:01,000\ntext', '1\n00:66:01,000 --> 00:67:02,000\ntext', 'WEBVTT\n\n', 'x'.repeat(1024 * 1024 + 1)])
        expect(() => parseLearningSubtitles(raw)).toThrow(); });
    it('seeks and highlights exact boundaries and preserves subtitle gaps', () => { const cues = [{ start: 1, end: 3, text: 'One' }, { start: 4, end: 6, text: 'Two' }]; expect(cueAtTime(cues, 1)).toBe(0); expect(cueAtTime(cues, 3)).toBe(-1); expect(cueAtTime(cues, 4.5)).toBe(1); expect(cueAtTime(cues, 7)).toBe(-1); });
    it('aligns real translation cues by overlap instead of inventing a translation', () => { const original = { start: 1, end: 4, text: 'Hello' }; const translated = [{ start: 0, end: 1, text: 'Before' }, { start: 1, end: 2, text: 'Xin' }, { start: 2, end: 4, text: 'chào' }, { start: 4, end: 5, text: 'After' }]; expect(translationForCue(original, translated)).toBe('Xin chào'); expect(translationForCue(original, [])).toBe(''); });
    it('roundtrips exported SRT and builds cloze questions for Latin and Japanese text', () => { const cues = [{ start: 1.001, end: 3.999, text: 'Daily routines' }]; expect(parseLearningSubtitles(exportLearningSRT(cues))).toEqual(cues); for (const text of ['I study every morning.', '毎日日本語を勉強します。']) {
        const cloze = makeCloze(text, text.startsWith('I') ? 'en' : 'ja');
        expect(cloze).not.toBeNull();
        expect(cloze!.before + cloze!.answer + cloze!.after).toBe(text);
    } expect(normalizePracticeAnswer('Ｈｅｌｌｏ, world!')).toBe(normalizePracticeAnswer('hello world')); });
});
