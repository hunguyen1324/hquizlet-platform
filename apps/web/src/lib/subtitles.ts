export type SubtitleCue = { start: number; end: number; text: string };
export const MAX_SRT_BYTES = 1024 * 1024;

export function parseSubtitleText(raw: string): SubtitleCue[] {
  const text = raw.replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n").trim();
  if (!text) return [];
  if (new TextEncoder().encode(text).length > MAX_SRT_BYTES || text.includes("\0")) throw new Error("Phụ đề không được quá 1 MiB.");
  const blocks = text.split(/\n\s*\n/);
  if (blocks.length > 10000) throw new Error("Phụ đề có quá nhiều đoạn.");
  return blocks.map((block, index) => {
    const lines = block.split("\n");
    const match = /^(\d{2,}):([0-5]\d):([0-5]\d)[,.](\d{3})\s+-->\s+(\d{2,}):([0-5]\d):([0-5]\d)[,.](\d{3})$/.exec(lines[1]?.trim() ?? "");
    if (!/^\d+$/.test(lines[0]?.trim() ?? "") || Number(lines[0]) < 1 || !match || lines.length < 3) throw new Error(`Đoạn ${index + 1}: cần số thứ tự, thời gian SRT và nội dung.`);
    const seconds = (offset: number) => Number(match[offset]) * 3600 + Number(match[offset + 1]) * 60 + Number(match[offset + 2]) + Number(match[offset + 3]) / 1000;
    const start = seconds(1), end = seconds(5), content = lines.slice(2).join("\n").trim();
    if (!Number.isFinite(end) || end <= start || !content) throw new Error(`Đoạn ${index + 1}: thời gian hoặc nội dung không hợp lệ.`);
    return { start, end, text: content.replace(/<[^>]*>/g, "") };
  });
}

export function normalizeSubtitleText(raw: string): string {
  parseSubtitleText(raw);
  const value = raw.replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n").trim();
  return value ? value + "\n" : "";
}
