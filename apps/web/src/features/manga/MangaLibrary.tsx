import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { mangaApi, type MangaVolume, type MangaPageResponse } from '../../lib/api/manga';
import { useAuth } from '../auth/AuthContext';
import { prepareMangaImport, type MangaImport } from './mokuro';
import './manga.css';
const message = (e: unknown) => e instanceof Error ? e.message : 'Không thực hiện được thao tác.';
export function MangaLibrary({ classId, canCreate = true }: { classId?: number; canCreate?: boolean }) {
    const { token, user } = useAuth();
    const [legacy,setLegacy]=useState<MangaVolume[]>([]);
    useEffect(()=>{if(classId && canCreate) mangaApi.list(token).then(items=>setLegacy(items.filter(v=>v.ownerId===user?.id))).catch(()=>{});},[token,classId,canCreate,user?.id]);
    const [volumes, setVolumes] = useState<MangaVolume[]>([]), [error, setError] = useState(''), [notice, setNotice] = useState(''), [loading, setLoading] = useState(true), [busy, setBusy] = useState(false), [imports, setImports] = useState<MangaImport[]>([]), [opened, setOpened] = useState<MangaVolume>(), [settings, setSettings] = useState<MangaVolume>();
    const input = useRef<HTMLInputElement>(null), folder = useRef<HTMLInputElement>(null), abort = useRef<AbortController>(), prepareID = useRef(0);
    const [sourceFiles, setSourceFiles] = useState<File[]>([]), [imageFiles, setImageFiles] = useState<File[]>([]);
    async function reload() {
        try {
            setVolumes(await mangaApi.list(token, classId));
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setLoading(false);
        }
    }
    useEffect(() => { void reload(); folder.current?.setAttribute('webkitdirectory', ''); return () => abort.current?.abort(); }, [token, classId]);
    async function prepare(sources: File[], images: File[]) {
        const id = ++prepareID.current;
        setImports([]);
        setError('');
        if (!sources.length || !images.length) {
            setNotice('Chọn file HTML và thư mục ảnh tương ứng để kiểm tra.');
            return;
        }
        setNotice('Đang kiểm tra HTML và ảnh…');
        try {
            const parsed = await prepareMangaImport([...sources, ...images.filter(f => /\.(?:jpe?g|png)$/i.test(f.name))], (source, name) => mangaApi.inspect(token, source, name));
            if (id !== prepareID.current)
                return;
            const pending = parsed.filter(p => !volumes.some(v => v.canEdit && v.status === 'ready' && v.sourceUuid === p.sourceUuid));
            setImports(pending);
            setNotice(pending.length ? 'Đã kiểm tra. Bấm Nhập để tải truyện lên; file gốc được giữ nguyên.' : 'Các tập đã có trong thư viện. Mở truyện để sửa OCR.');
        }
        catch (e) {
            if (id === prepareID.current)
                setError(message(e));
        }
    }
    async function upload() {
        if (busy || !imports.length)
            return;
        setBusy(true);
        setError('');
        const controller = new AbortController();
        abort.current = controller;
        try {
            for (const volume of imports) {
                if (controller.signal.aborted)
                    throw Error('Đã dừng nhập. Có thể chọn lại thư mục để tiếp tục phần đang nhập.');
                const created = await mangaApi.create(token, { title: volume.title, volume: volume.volume, sourceUuid: volume.sourceUuid, pageCount: volume.pages.length, htmlSource: volume.htmlSource, sourceName: volume.sourceName }, classId);
                const detail = await mangaApi.detail(token, created.id);
                const existing = new Set(detail.pages);
                for (let i = 0; i < volume.pages.length; i++) {
                    if (existing.has(i))
                        continue;
                    setNotice(`${volume.volume}: đang tải trang ${i + 1}/${volume.pages.length}`);
                    await mangaApi.upload(token, created.id, i, volume.pages[i].page, volume.pages[i].file, controller.signal);
                }
                await mangaApi.complete(token, created.id);
                await reload();
            }
            setImports([]);
            if (input.current)
                input.current.value = '';
            setNotice('Đã nhập truyện. Bạn có thể mở đọc và sửa chữ OCR.');
        }
        catch (e) {
            setError(controller.signal.aborted ? 'Đã dừng nhập. Chọn lại thư mục để tiếp tục.' : message(e));
            await reload();
        }
        finally {
            setBusy(false);
        }
    }
    return <section className="manga-library"><header className="manga-heading"><div><span>ĐỌC VÀ HỌC TIẾNG NHẬT</span><h1>{classId ? "Truyện của nhóm" : "Thư viện truyện"}</h1><p>{classId ? "Truyện trong nhóm mặc định chỉ dành cho thành viên nhóm." : "Truyện tại đây mặc định chia sẻ cho mọi người trên HQuizlet."} Chọn vùng chữ để xem OCR và sửa lỗi nhận dạng.</p></div>{canCreate && <div className="manga-import-controls"><label className="manga-import-button">① Chọn HTML<input ref={input} type="file" multiple accept=".html,.htm,.mokuro" disabled={busy} aria-label="Chọn file HTML truyện" onChange={e => { const files = Array.from(e.target.files || []); setSourceFiles(files); void prepare(files, imageFiles); }}/></label><label className="manga-import-button secondary">② Chọn thư mục ảnh<input ref={folder} type="file" multiple disabled={busy} aria-label="Chọn thư mục ảnh truyện" onChange={e => { const files = Array.from(e.target.files || []); setImageFiles(files); void prepare(sourceFiles, files); }}/></label></div>}</header>
 {classId && canCreate && legacy.length>0 && <label>Đưa truyện từ thư viện cũ vào lớp<select aria-label="Đưa truyện cũ vào lớp" value="" disabled={busy} onChange={async e=>{const id=Number(e.target.value);if(!id)return;setBusy(true);setError('');try{await mangaApi.move(token,id,classId);setLegacy(all=>all.filter(v=>v.id!==id));await reload();}catch(e){setError(message(e));}finally{setBusy(false);}}}><option value="">Chọn tập truyện…</option>{legacy.map(v=><option key={v.id} value={v.id}>{v.title} · {v.volume}</option>)}</select></label>}
 {error && <p role="alert" className="manga-error">{error}</p>}{notice && <p role="status" className="manga-notice">{notice}</p>}
 {!!imports.length && <div className="manga-import-review"><h2>Kiểm tra trước khi nhập</h2><p>{classId ? "Truyện mặc định chia sẻ cho thành viên lớp." : "Truyện mặc định chia sẻ cho mọi người trên HQuizlet."} File gốc trên máy được giữ nguyên.</p>{imports.map((v, i) => <div key={v.sourceUuid}><input aria-label={`Tên truyện ${i + 1}`} disabled={busy} maxLength={200} value={v.title} onChange={e => setImports(all => all.map((x, n) => n === i ? { ...x, title: e.target.value } : x))}/><strong>{v.volume}</strong><span>{v.pages.length} trang · {(v.pages.reduce((s, p) => s + p.file.size, 0) / 1048576).toFixed(1)} MiB</span></div>)}<button disabled={busy || imports.some(v => !v.title.trim())} onClick={() => void upload()}>{busy ? 'Đang nhập…' : `Nhập ${imports.length} tập`}</button>{busy ? <button onClick={() => abort.current?.abort()}>Dừng nhập</button> : <button onClick={() => setImports([])}>Hủy</button>}</div>}
 {loading ? <p role="status">Đang tải thư viện…</p> : !volumes.length && <div className="manga-empty"><h2>Thư viện truyện đang trống</h2><p>Chọn file HTML Mokuro và thư mục ảnh đi kèm để bắt đầu.</p><p>Hỗ trợ JPEG/PNG; chữ OCR được giữ ở từng khung thoại.</p></div>}
 <div className="manga-grid">{volumes.map(v => <article className="manga-card" key={v.id}><div className="manga-book-mark" aria-hidden="true">漫</div><div><small>{v.visibility === 'private' ? 'Riêng tư' : v.classId ? 'Thành viên nhóm' : 'Mọi người trên HQuizlet'}</small><h2>{v.title}</h2><p>{v.volume}</p><span>{v.uploaded}/{v.pageCount} trang{v.status === 'importing' ? ' · Chưa nhập xong' : ''}</span></div><footer><button disabled={v.status !== 'ready'} onClick={() => setOpened(v)}>Mở đọc →</button>{(v.canEdit || v.canDelete) && <button onClick={() => setSettings({ ...v })}>Cài đặt</button>}</footer></article>)}</div>
 {settings && <div className="manga-settings" role="region" aria-label="Cài đặt truyện"><h2>Cài đặt tập truyện</h2><label>Tên truyện<input value={settings.title} maxLength={200} onChange={e => setSettings({ ...settings, title: e.target.value })}/></label><label>Tên tập<input value={settings.volume} maxLength={200} onChange={e => setSettings({ ...settings, volume: e.target.value })}/></label><label>Quyền xem<select value={settings.visibility} onChange={e => setSettings({ ...settings, visibility: e.target.value as 'private' | 'shared' })}><option value="private">Riêng tư · chỉ chủ truyện</option><option value="shared">{classId ? 'Chia sẻ · thành viên nhóm được đọc' : 'Chia sẻ · mọi người trên HQuizlet được đọc'}</option></select></label><p>{classId ? "Quyền chỉnh sửa và xóa được quản lý bằng vai trò trong nhóm." : "Chủ truyện có quyền chỉnh sửa, đổi quyền xem và xóa truyện."}</p><button disabled={busy || !settings.canEdit || !settings.title.trim()} onClick={async () => {
                setBusy(true);
                setError('');
                try {
                    await mangaApi.update(token, settings);
                    setSettings(undefined);
                    await reload();
                }
                catch (e) {
                    setError(message(e));
                }
                finally {
                    setBusy(false);
                }
            }}>Lưu cài đặt</button><button onClick={() => setSettings(undefined)}>Đóng</button><button className="manga-danger" disabled={busy || !settings.canDelete} onClick={async () => {
                if (!confirm('Xóa tập truyện trên web? File gốc trên máy được giữ nguyên.'))
                    return;
                setBusy(true);
                try {
                    await mangaApi.remove(token, settings.id);
                    setSettings(undefined);
                    await reload();
                }
                catch (e) {
                    setError(message(e));
                }
                finally {
                    setBusy(false);
                }
            }}>Xóa tập</button></div>}
 {opened && <MangaReader token={token} volume={opened} onClose={() => setOpened(undefined)}/>}
 </section>;
}
function MangaImage({ token, volume, index, onSelect, version }: {
    token: string;
    volume: MangaVolume;
    index: number;
    onSelect: (index: number, data: MangaPageResponse, block: number) => void;
    version: number;
}) {
    const [data, setData] = useState<MangaPageResponse>(), [url, setURL] = useState(''), [error, setError] = useState('');
    useEffect(() => {
        const controller = new AbortController();
        let local = '';
        setData(undefined);
        setURL('');
        setError('');
        Promise.all([mangaApi.page(token, volume.id, index, controller.signal), mangaApi.image(token, volume.id, index, controller.signal)]).then(([page, blob]) => {
            if (controller.signal.aborted)
                return;
            local = URL.createObjectURL(blob);
            setData(page);
            setURL(local);
        }).catch(e => {
            if (!controller.signal.aborted)
                setError(message(e));
        });
        return () => {
            controller.abort();
            if (local)
                URL.revokeObjectURL(local);
        };
    }, [token, volume.id, index, version]);
    if (error)
        return <p role="alert">{error}</p>;
    if (!data || !url)
        return <p role="status" className="manga-image-loading">Đang tải trang {index + 1}…</p>;
    return <figure className="manga-page" style={{ aspectRatio: `${data.page.img_width}/${data.page.img_height}`, '--page-ratio': data.page.img_width / data.page.img_height } as import('react').CSSProperties}><img src={url} alt={`Trang ${index + 1}`} draggable={false}/>{data.page.blocks.map((b, i) => <button key={i} className="manga-region" aria-label={`Xem chữ OCR trang ${index + 1}, khối ${i + 1}`} onClick={() => onSelect(index, data, i)} style={{ left: `${b.box[0] / data.page.img_width * 100}%`, top: `${b.box[1] / data.page.img_height * 100}%`, width: `${(b.box[2] - b.box[0]) / data.page.img_width * 100}%`, height: `${(b.box[3] - b.box[1]) / data.page.img_height * 100}%` }}><span lang="ja">{b.lines.join('\n')}</span></button>)}<figcaption>Trang {index + 1}</figcaption></figure>;
}
export function MangaReader({ token, volume, onClose }: {
    token: string;
    volume: MangaVolume;
    onClose: () => void;
}) {
    const dialog = useRef<HTMLDialogElement>(null);
    const canvas = useRef<HTMLDivElement>(null);
    useEffect(() => { const el=canvas.current; if(!el)return; const observer=new ResizeObserver(() => { const style=getComputedStyle(el); el.style.setProperty("--canvas-height", `${el.clientHeight-parseFloat(style.paddingTop)-parseFloat(style.paddingBottom)}px`); });observer.observe(el);return ()=>observer.disconnect(); }, []);
    const [index, setIndex] = useState(() => { const n = Number(localStorage.getItem(`manga-page-${volume.id}`)); return Number.isInteger(n) && n >= 0 && n < volume.pageCount ? n : 0; }), [spread, setSpread] = useState(innerWidth >= 900 ? 2 : 1), [rtl, setRTL] = useState(true), [zoom, setZoom] = useState(0), [selection, setSelection] = useState<{
        index: number;
        data: MangaPageResponse;
        block: number;
    }>(), [text, setText] = useState(''), [dirty, setDirty] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState(''), [busy, setBusy] = useState(false), [version, setVersion] = useState(0);
    const safeLeave = () => !dirty || confirm('Chữ OCR chưa lưu. Bỏ thay đổi để tiếp tục?');
    function close() {
        if (!busy && safeLeave())
            onClose();
    }
    function navigate(next: number) {
        if (busy || next < 0 || next >= volume.pageCount || !safeLeave())
            return;
        setIndex(next);
        setSelection(undefined);
        setDirty(false);
        setNotice('');
        setError('');
        dialog.current?.querySelector('.manga-reader-canvas')?.scrollTo({ top: 0 });
    }
    useEffect(() => { const prev = document.activeElement as HTMLElement | null; const overflow = document.body.style.overflow; dialog.current?.showModal(); document.body.style.overflow = 'hidden'; return () => { dialog.current?.close(); document.body.style.overflow = overflow; prev?.focus(); }; }, []);
    useEffect(() => { localStorage.setItem(`manga-page-${volume.id}`, String(index)); }, [volume.id, index]);
    useEffect(() => {
        const unload = (e: BeforeUnloadEvent) => {
            if (dirty || busy) {
                e.preventDefault();
                e.returnValue = '';
            }
        };
        window.addEventListener('beforeunload', unload);
        return () => window.removeEventListener('beforeunload', unload);
    }, [dirty, busy]);
    async function save(restore = false) {
        if (!selection || busy)
            return;
        if (restore && !confirm('Khôi phục toàn bộ chữ OCR gốc của trang này?'))
            return;
        setBusy(true);
        setError('');
        try {
            const lines = selection.data.page.blocks.map((b, i) => i === selection.block ? text.split('\n') : b.lines);
            const result = await mangaApi.edit(token, volume.id, selection.index, selection.data.revision, lines, restore);
            setSelection({ ...selection, data: result });
            setText(result.page.blocks[selection.block]?.lines.join('\n') || '');
            setDirty(false);
            setVersion(v => v + 1);
            setNotice(restore ? 'Đã khôi phục OCR gốc của trang.' : 'Đã lưu chữ OCR.');
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setBusy(false);
        }
    }
    return createPortal(<dialog className="manga-reader" ref={dialog} aria-labelledby="manga-reader-title" onCancel={e => { e.preventDefault(); close(); }} onKeyDown={e => {
            if ((e.target as HTMLElement).closest('button,input,select,textarea'))
                return;
            if (e.key === 'ArrowRight') {
                e.preventDefault();
                navigate(index + (rtl ? -spread : spread));
            }
            if (e.key === 'ArrowLeft') {
                e.preventDefault();
                navigate(index + (rtl ? spread : -spread));
            }
        }}>
 <header><button aria-label="Đóng truyện" onClick={close}>✕</button><div><h2 id="manga-reader-title">{volume.title}</h2><small>{volume.volume}</small></div><nav aria-label="Điều khiển trang"><button disabled={busy || index === 0} onClick={() => navigate(0)}>«</button><button aria-label="Trang trước" disabled={busy || index === 0} onClick={() => navigate(Math.max(0, index - spread))}>‹</button><label>Trang<input aria-label="Số trang" type="number" min={1} max={volume.pageCount} value={index + 1} onChange={e => navigate(Number(e.target.value) - 1)}/></label><span>/ {volume.pageCount}</span><button aria-label="Trang tiếp theo" disabled={busy || index + spread >= volume.pageCount} onClick={() => navigate(index + spread)}>›</button><button disabled={busy || index === volume.pageCount - 1} onClick={() => navigate(volume.pageCount - 1)}>»</button></nav><label><span className="manga-control-label">Chế độ</span><select aria-label="Chế độ trang" value={spread} onChange={e => {
            if (safeLeave()) {
                setSpread(Number(e.target.value));
                setSelection(undefined);
                setDirty(false);
            }
        }}><option value={1}>Một trang</option><option value={2}>Hai trang</option></select></label><button aria-pressed={rtl} onClick={() => setRTL(!rtl)}>{rtl ? 'Phải → trái' : 'Trái → phải'}</button><button disabled={busy} onClick={async () => {
            if (dirty && !confirm('Chữ chưa lưu sẽ không có trong file xuất. Tiếp tục tải bản đã lưu?'))
                return;
            setBusy(true);
            setError('');
            try {
                const blob = await mangaApi.exportHTML(token, volume.id);
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = volume.volume + '-edited.html';
                a.click();
                setTimeout(() => URL.revokeObjectURL(url), 1000);
            }
            catch (e) {
                setError(message(e));
            }
            finally {
                setBusy(false);
            }
        }}>↓ HTML đã sửa</button><label><span className="manga-control-label">Thu phóng</span><select aria-label="Thu phóng" value={zoom} onChange={e => setZoom(Number(e.target.value))}><option value={0}>Vừa trang</option>{[75, 100, 125, 150, 200].map(n => <option key={n} value={n}>{n}%</option>)}</select></label></header>
 {error && !selection && <p role="alert" className="manga-error">{error}</p>}
 <div className={`manga-reader-workspace ${selection ? 'has-editor' : ''}`}><div className="manga-reader-canvas" ref={canvas}><div className={`manga-spread ${spread === 2 ? 'double' : ''} ${zoom === 0 ? 'fit-page' : ''}`} style={{ width: zoom === 0 ? '100%' : `${zoom}%`, direction: rtl ? 'rtl' : 'ltr' }}>{Array.from({ length: Math.min(spread, volume.pageCount - index) }, (_, i) => <MangaImage key={index + i} token={token} volume={volume} index={index + i} version={version} onSelect={(n, data, block) => {
                if (busy || !safeLeave())
                    return;
                setSelection({ index: n, data, block });
                setText(data.page.blocks[block].lines.join('\n'));
                setDirty(false);
                setError('');
                setNotice('');
            }}/>)}</div></div>
 {selection && <aside className="manga-ocr-editor"><header><h3>Chữ OCR · trang {selection.index + 1}</h3><button aria-label="Đóng bảng OCR" onClick={() => {
                if (!busy && safeLeave()) {
                    setSelection(undefined);
                    setDirty(false);
                }
            }}>×</button></header><label>Khung thoại<select disabled={busy} value={selection.block} onChange={e => {
                if (!safeLeave())
                    return;
                const n = Number(e.target.value);
                setSelection({ ...selection, block: n });
                setText(selection.data.page.blocks[n].lines.join('\n'));
                setDirty(false);
            }}>{selection.data.page.blocks.map((_, i) => <option value={i} key={i}>Khối {i + 1}</option>)}</select></label><label>Nội dung tiếng Nhật<textarea lang="ja" value={text} rows={10} readOnly={!volume.canEdit} disabled={busy} maxLength={100000} onChange={e => { setText(e.target.value); setDirty(true); setNotice(''); }}/></label><p>{volume.canEdit ? 'Chỉ sửa chữ OCR; ảnh gốc và vị trí khung thoại được giữ nguyên. Mỗi dòng là một dòng OCR.' : 'Bạn có thể chọn và sao chép chữ. Bạn cần quyền chỉnh sửa truyện để lưu thay đổi.'}</p>{volume.canEdit && <><button className="manga-save-ocr" disabled={busy || !dirty} onClick={() => void save()}>{busy ? 'Đang lưu…' : 'Lưu chữ OCR'}</button><button disabled={busy} onClick={() => void save(true)}>Khôi phục OCR gốc của trang</button></>}{error && <p role="alert" className="manga-error">{error}</p>}{notice && <p role="status" className="manga-notice">{notice}</p>}<small>Phiên bản trang: {selection.data.revision}{dirty ? ' · Chưa lưu' : ''}</small></aside>}
 </div></dialog>, document.body);
}
