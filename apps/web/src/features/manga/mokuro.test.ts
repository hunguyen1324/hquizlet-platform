import { describe, it, expect } from 'vitest';
import { parseMokuro } from './mokuro';
const sample = () => ({ volume_uuid: 'test-volume', title: 'Manga', volume: '1', pages: [{ img_path: '1.jpg', img_width: 100, img_height: 200, blocks: [{ box: [1, 2, 30, 40] as [number, number, number, number], vertical: true, font_size: 12, lines: ['こんにちは'], lines_coords: [[]] }] }] });
describe('Mokuro import', () => {
    it('preserves OCR text, bounds and explicit page order, without HTML', () => { const d = sample(); d.pages.push({ ...d.pages[0], img_path: '10.jpg' }); const p = parseMokuro(d); expect(p.pages.map(p => p.img_path)).toEqual(['1.jpg', '10.jpg']); expect(p.pages[0].blocks[0].lines).toEqual(['こんにちは']); expect(p.pages[0].blocks[0]).not.toHaveProperty('lines_coords'); });
    it('rejects missing volume ID, traversal, giant images and invalid blocks', () => {
        for (const change of [(d: ReturnType<typeof sample>) => d.volume_uuid = '', (d: ReturnType<typeof sample>) => d.pages[0].img_path = '../outside.jpg', (d: ReturnType<typeof sample>) => d.pages[0].img_width = 30000, (d: ReturnType<typeof sample>) => d.pages[0].blocks[0].lines = ['bad\0text']]) {
            const d = sample();
            change(d);
            expect(() => parseMokuro(d)).toThrow();
        }
    });
});
import { prepareMangaImport } from './mokuro';
it('imports HTML text as source and resolves pictures from a selected parent folder', async () => {
    const html = new File(['<html>Existing reader</html>'], 'chapter.html', { type: 'text/html' });
    const image = new File(['image bytes'], '1.jpg', { type: 'image/jpeg' });
    Object.defineProperty(image, 'webkitRelativePath', { value: 'manga/manga_images2/1.jpg' });
    const parsed = { sourceUuid: 'html-hash', title: 'HTML title', volume: 'chapter', pages: [{ ...sample().pages[0], img_path: 'manga_images2/1.jpg' }] };
    let called = false;
    const result = await prepareMangaImport([html, image], async (source, name) => { called = source.includes('Existing reader') && name === 'chapter.html'; return parsed; });
    expect(called).toBe(true);
    expect(result[0].htmlSource).toContain('Existing reader');
    expect(result[0].sourceName).toBe('chapter.html');
    expect(result[0].pages[0].file).toBe(image);
    await expect(prepareMangaImport([html], async () => parsed)).rejects.toThrow('thiếu ảnh');
});
it('rejects ambiguous image folder matches', async () => {
    const html = new File(['html'], 'chapter.html');
    const a = new File(['a'], '1.jpg'), b = new File(['b'], '1.jpg');
    Object.defineProperty(a, 'webkitRelativePath', { value: 'folderA/manga_images2/1.jpg' });
    Object.defineProperty(b, 'webkitRelativePath', { value: 'folderB/manga_images2/1.jpg' });
    await expect(prepareMangaImport([html, a, b], async () => ({ sourceUuid: 'id', title: 'Manga', volume: '1', pages: [{ ...sample().pages[0], img_path: 'manga_images2/1.jpg' }] }))).rejects.toThrow('trùng đường dẫn');
});
