import type { MangaPage, HTMLInspection } from '../../lib/api/manga';
export type MangaImport = {
    sourceUuid: string;
    title: string;
    volume: string;
    htmlSource?: string;
    sourceName?: string;
    pages: {
        page: MangaPage;
        file: File;
    }[];
};
const safePath = (p: string) => {
    if (!p || p.startsWith('/') || p.includes('\\') || p.split('/').some(s => !s || s === '.' || s === '..') || p.includes(':'))
        throw Error('Đường dẫn ảnh trong Mokuro không hợp lệ.');
    return p;
};
export function parseMokuro(value: unknown): {
    sourceUuid: string;
    title: string;
    volume: string;
    pages: (MangaPage & {
        img_path: string;
    })[];
} {
    if (!value || typeof value !== 'object')
        throw Error('File Mokuro không hợp lệ.');
    const d = value as Record<string, unknown>;
    if (typeof d.volume_uuid !== 'string' || !d.volume_uuid.trim() || d.volume_uuid.length > 100 || !Array.isArray(d.pages) || !d.pages.length || d.pages.length > 1000)
        throw Error('Mokuro cần mã tập và từ 1 đến 1.000 trang.');
    const pages = d.pages.map((raw, index) => {
        if (!raw || typeof raw !== 'object')
            throw Error(`Trang ${index + 1} bị lỗi.`);
        const p = raw as MangaPage & {
            img_path: string;
        };
        safePath(p.img_path);
        if (!Number.isInteger(p.img_width) || !Number.isInteger(p.img_height) || p.img_width < 1 || p.img_height < 1 || p.img_width > 20000 || p.img_height > 20000 || p.img_width * p.img_height > 100000000 || !Array.isArray(p.blocks) || p.blocks.length > 1000)
            throw Error(`Kích thước/OCR trang ${index + 1} không hợp lệ.`);
        const blocks = p.blocks.map(b => {
            if (!Array.isArray(b.box) || b.box.length !== 4 || b.box.some(n => !Number.isFinite(n)) || !Array.isArray(b.lines) || b.lines.length > 100 || b.lines.some(l => typeof l !== 'string' || l.length > 2000 || l.includes('\0')))
                throw Error(`Khối OCR trang ${index + 1} bị lỗi.`);
            return { box: b.box, vertical: !!b.vertical, font_size: Number.isFinite(b.font_size) ? b.font_size : 0, lines: b.lines };
        });
        return { img_width: p.img_width, img_height: p.img_height, img_path: p.img_path, blocks };
    });
    return { sourceUuid: d.volume_uuid, title: String(d.title || 'Truyện').slice(0, 200), volume: String(d.volume || '').slice(0, 200), pages };
}
export async function prepareMangaImport(files: File[], inspectHTML?: (source: string, name: string) => Promise<HTMLInspection>): Promise<MangaImport[]> {
    const images = files.filter(f => /\.(?:jpe?g|png)$/i.test(f.name));
    const html = files.filter(f => /\.html?$/i.test(f.name));
    const metadata = (html.length ? html : files.filter(f => f.name.toLowerCase().endsWith('.mokuro'))).sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true }));
    if (!metadata.length)
        throw Error('Chọn file HTML Mokuro (hoặc .mokuro) và thư mục ảnh.');
    const result: MangaImport[] = [];
    for (const file of metadata) {
        if (file.size > 8 * 1024 * 1024)
            throw Error(file.name + ': file tối đa 8 MiB.');
        const source = (await file.text()).replace(/^\uFEFF/, '');
        const isHTML = /\.html?$/i.test(file.name);
        if (isHTML && !inspectHTML)
            throw Error('Cần kiểm tra cấu trúc HTML trước khi nhập.');
        const parsed = isHTML ? await inspectHTML!(source, file.name) : parseMokuro(JSON.parse(source));
        const filename = file.webkitRelativePath || file.name;
        const base = isHTML ? filename.slice(0, filename.lastIndexOf('/') + 1) : filename.slice(0, filename.lastIndexOf('/') + 1) + file.name.slice(0, -7) + '/';
        const pages = parsed.pages.map(p => {
            const relative = safePath(p.img_path);
            const full = base + relative;
            const exact = images.filter(f => (f.webkitRelativePath || f.name) === full);
            const matches = exact.length ? exact : images.filter(f => { const name = f.webkitRelativePath || f.name; return name === relative || name.endsWith('/' + relative); });
            if (matches.length !== 1)
                throw Error(file.name + ': ' + (matches.length ? 'trùng đường dẫn' : 'thiếu ảnh') + ' ' + relative);
            const image = matches[0];
            if (image.size > 12 * 1024 * 1024 || !image.size)
                throw Error(image.name + ': ảnh tối đa 12 MiB.');
            return { page: { img_width: p.img_width, img_height: p.img_height, blocks: p.blocks }, file: image };
        });
        result.push({ ...parsed, pages, ...(isHTML ? { htmlSource: source, sourceName: file.name } : {}) });
    }
    return result;
}
