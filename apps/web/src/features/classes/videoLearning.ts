import type { VideoCue } from '../../lib/api/videoSubtitles';
const LIMIT = 1024 * 1024;
function plainText(value: string): string {
    return value.replace(/<[^>]*>/g, '').replace(/&(?:amp|lt|gt|quot|apos|nbsp|#\d+|#x[0-9a-f]+);/gi, entity => {
        const name = entity.slice(1, -1).toLowerCase();
        const named: Record<string, string> = { amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ' };
        if (name in named)
            return named[name];
        const n = name.startsWith('#x') ? parseInt(name.slice(2), 16) : parseInt(name.slice(1), 10);
        return Number.isFinite(n) && n > 0 && n <= 0x10ffff ? String.fromCodePoint(n) : '';
    }).trim();
}
function timestamp(value: string): number {
    const match = /^(?:(\d{2,}):)?([0-5]\d):([0-5]\d)[.,](\d{3})$/.exec(value);
    if (!match)
        throw Error('Thời gian phụ đề phải có dạng 00:00:01,000 hoặc 00:01.000.');
    return Number(match[1] || 0) * 3600 + Number(match[2]) * 60 + Number(match[3]) + Number(match[4]) / 1000;
}
export function parseLearningSubtitles(raw: string): VideoCue[] {
    if (new TextEncoder().encode(raw).length > LIMIT || raw.includes('\0'))
        throw Error('File phụ đề tối đa 1 MiB.');
    const text = raw.replace(/^\uFEFF/, '').replace(/\r\n?/g, '\n').trim();
    const blocks = text.split(/\n\s*\n/);
    const cues: VideoCue[] = [];
    for (const block of blocks) {
        const lines = block.split('\n');
        if (/^(WEBVTT(?:\s|$)|NOTE(?:\s|$)|STYLE$|REGION$)/.test(lines[0]))
            continue;
        const timing = lines.findIndex(line => line.includes('-->'));
        if (timing < 0 || timing > 1)
            throw Error('File phải là SRT hoặc WebVTT có mốc thời gian cho từng câu.');
        const match = /^(\S+)\s+-->\s+(\S+)(?:\s+.*)?$/.exec(lines[timing].trim());
        if (!match)
            throw Error(`Mốc thời gian câu ${cues.length + 1} không hợp lệ.`);
        const start = timestamp(match[1]), end = timestamp(match[2]), content = plainText(lines.slice(timing + 1).join('\n'));
        if (end <= start || end > 43200 || !content || Array.from(content).length > 3000)
            throw Error(`Câu ${cues.length + 1} có thời gian hoặc nội dung không hợp lệ.`);
        cues.push({ start, end, text: content });
        if (cues.length > 10000)
            throw Error('Tối đa 10000 câu phụ đề.');
    }
    if (!cues.length)
        throw Error('Không tìm thấy câu phụ đề trong file.');
    return cues.sort((a, b) => a.start - b.start);
}
export function cueAtTime(cues: VideoCue[], time: number): number {
    let low = 0, high = cues.length;
    while (low < high) {
        const mid = (low + high) >>> 1;
        if (cues[mid].start <= time)
            low = mid + 1;
        else
            high = mid;
    }
    for (let i = low - 1; i >= 0; i--) {
        if (time < cues[i].end)
            return i;
    }
    return -1;
}
export function translationForCue(cue: VideoCue | undefined, cues: VideoCue[]): string {
    if (!cue || !cues.length)
        return '';
    const found: string[] = [];
    for (const translation of cues) {
        if (translation.start >= cue.end)
            break;
        if (translation.end > cue.start && !found.includes(translation.text))
            found.push(translation.text);
    }
    return found.join(' ');
}
export function formatCueTime(value: number, ms = false): string {
    const total = Math.max(0, Math.round(value * 1000));
    const hours = Math.floor(total / 3600000), minutes = Math.floor(total / 60000) % 60, seconds = Math.floor(total / 1000) % 60, millis = total % 1000;
    return `${hours || ms ? String(hours).padStart(2, '0') + ':' : ''}${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}${ms ? ',' + String(millis).padStart(3, '0') : ''}`;
}
export const exportLearningSRT = (cues: VideoCue[]) => cues.map((cue, i) => `${i + 1}\n${formatCueTime(cue.start, true)} --> ${formatCueTime(cue.end, true)}\n${cue.text}\n`).join('\n');
export const normalizePracticeAnswer = (text: string) => text.normalize('NFKC').toLocaleLowerCase().replace(/[\p{P}\p{Z}\p{S}\s]/gu, '');
export function makeCloze(text: string, language = 'en'): {
    before: string;
    answer: string;
    after: string;
} | null {
    let words: {
        segment: string;
        index: number;
        isWordLike?: boolean;
    }[] = [];
    const Segmenter = (Intl as unknown as {
        Segmenter?: new (locale: string, options: {
            granularity: string;
        }) => {
            segment: (text: string) => Iterable<{
                segment: string;
                index: number;
                isWordLike?: boolean;
            }>;
        };
    }).Segmenter;
    if (Segmenter) {
        let locale = language || 'en';
        try {
            Intl.getCanonicalLocales(locale);
        }
        catch {
            locale = 'en';
        }
        ;
        words = [...new Segmenter(locale, { granularity: 'word' }).segment(text)].filter(part => part.isWordLike);
    }
    else {
        words = [...text.matchAll(/[\p{L}\p{N}]+/gu)].map(m => ({ segment: m[0], index: m.index!, isWordLike: true }));
    }
    if (!words.length)
        return null;
    const chosen = words[Math.floor(words.length / 2)];
    return { before: text.slice(0, chosen.index), answer: chosen.segment, after: text.slice(chosen.index + chosen.segment.length) };
}
