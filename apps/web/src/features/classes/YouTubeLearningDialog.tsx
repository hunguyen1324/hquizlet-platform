import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { videoSubtitleApi, type ClassVideo, type VideoSubtitleResult, type VideoSubtitleTrack } from '../../lib/api';
import { cueAtTime, exportLearningSRT, formatCueTime, makeCloze, normalizePracticeAnswer, parseLearningSubtitles, translationForCue } from './videoLearning';
import { describeYouTubeError, loadYouTubeAPI, type YouTubePlayer } from './youtubePlayer';
import './YouTubeLearningDialog.css';
type Mode = 'shadowing' | 'pronunciation' | 'listening' | 'exercise';
const modes: {
    key: Mode;
    name: string;
    icon: string;
}[] = [{ key: 'shadowing', name: 'Shadowing', icon: '▣' }, { key: 'pronunciation', name: 'Phát âm', icon: '♩' }, { key: 'listening', name: 'Luyện nghe', icon: '♬' }, { key: 'exercise', name: 'Bài tập', icon: '▤' }];
const blank = (): VideoSubtitleResult => ({ original: null, translation: null, availableTracks: [] });
const languages = [['ja', '日本語'], ['en', 'English'], ['vi', 'Tiếng Việt'], ['ko', '한국어'], ['zh', '中文'], ['fr', 'Français'], ['de', 'Deutsch'], ['es', 'Español']];
export function YouTubeLearningDialog({ video, classId, token, canEditSubtitles, onClose }: {
    video: ClassVideo;
    classId: number;
    token: string;
    canEditSubtitles: boolean;
    onClose: () => void;
}) {
    const dialog = useRef<HTMLDialogElement>(null), playerMount = useRef<HTMLDivElement>(null), player = useRef<YouTubePlayer>();
    const [ready, setReady] = useState(false), [playing, setPlaying] = useState(false), [playerError, setPlayerError] = useState('');
    const [data, setData] = useState<VideoSubtitleResult>(blank), [loading, setLoading] = useState(true), [error, setError] = useState(''), [busy, setBusy] = useState(false);
    const [mode, setMode] = useState<Mode>('shadowing'), [focus, setFocus] = useState(0), [time, setTime] = useState(0), [repeat, setRepeat] = useState(false), [rate, setRate] = useState(1), [rates, setRates] = useState<number[]>([1]);
    const [panel, setPanel] = useState(true), [bilingual, setBilingual] = useState(true), [follow, setFollow] = useState(true), [large, setLarge] = useState(false), [search, setSearch] = useState('');
    const [answer, setAnswer] = useState(''), [checked, setChecked] = useState(false), [revealed, setRevealed] = useState(false);
    const [importKind, setImportKind] = useState<'original' | 'translation'>('original'), [importLanguage, setImportLanguage] = useState('ja'), [shareImport, setShareImport] = useState(canEditSubtitles), [notice, setNotice] = useState('');
    const [recording, setRecording] = useState(false), [audioURL, setAudioURL] = useState(''), [micError, setMicError] = useState('');
    const micPending = useRef(false), micTimer = useRef<number>();
    const request = useRef<AbortController>(), requestID = useRef(0), recorder = useRef<MediaRecorder>(), stream = useRef<MediaStream>(), audioURLRef = useRef(''), alive = useRef(true), list = useRef<HTMLDivElement>(null);
    const cues = data.original?.cues || [], cue = cues[focus];
    const cuesRef = useRef(cues), focusRef = useRef(focus), repeatRef = useRef(repeat), modeRef = useRef(mode);
    cuesRef.current = cues;
    focusRef.current = focus;
    repeatRef.current = repeat;
    modeRef.current = mode;
    const translations = useMemo(() => cues.map(c => translationForCue(c, data.translation?.cues || [])), [data.original, data.translation]);
    const cloze = useMemo(() => makeCloze(cue?.text || '', data.original?.language || 'en'), [cue, data.original?.language]);
    const hideAnswers = (mode === 'listening' || mode === 'exercise') && !revealed && !checked;
    const filtered = useMemo(() => cues.map((cue, index) => ({ cue, index })).filter(({ cue, index }) => `${cue.text} ${translations[index]}`.toLocaleLowerCase().includes(search.toLocaleLowerCase())), [cues, translations, search]);
    function stopMic() { if (micTimer.current !== undefined)
        clearTimeout(micTimer.current); if (recorder.current?.state === 'recording')
        recorder.current.stop(); stream.current?.getTracks().forEach(t => t.stop()); stream.current = undefined; setRecording(false); }
    async function loadSubtitles(options: {
        language?: string;
        translation?: string;
        youtube?: number;
    } = {}) {
        const id = ++requestID.current;
        request.current?.abort();
        const controller = new AbortController();
        request.current = controller;
        setLoading(true);
        setError('');
        setNotice('');
        try {
            const result = await videoSubtitleApi.get(token, classId, video.id, options, controller.signal);
            if (id !== requestID.current || !alive.current)
                return;
            setData(result);
            setFocus(0);
            setSearch('');
            setRepeat(false);
            setNotice(result.message || '');
        }
        catch (e) {
            if (!controller.signal.aborted && alive.current)
                setError(e instanceof Error ? e.message : 'Không tải được phụ đề. Bạn có thể nhập SRT/VTT.');
        }
        finally {
            if (id === requestID.current && alive.current)
                setLoading(false);
        }
    }
    useEffect(() => {
        alive.current = true;
        const element = dialog.current;
        const previous = document.activeElement as HTMLElement | null;
        const overflow = document.body.style.overflow;
        element?.showModal();
        document.body.style.overflow = 'hidden';
        void loadSubtitles();
        return () => { alive.current = false; if (micTimer.current !== undefined)
            clearTimeout(micTimer.current); request.current?.abort(); if (recorder.current?.state === 'recording')
            recorder.current.stop(); stream.current?.getTracks().forEach(t => t.stop()); if (audioURLRef.current)
            URL.revokeObjectURL(audioURLRef.current); element?.close(); document.body.style.overflow = overflow; previous?.focus(); };
    }, [classId, video.id, token]);
    useEffect(() => {
        let disposed = false;
        let clock: number | undefined;
        let instance: YouTubePlayer | undefined;
        setReady(false);
        loadYouTubeAPI().then(api => {
            if (disposed || !playerMount.current)
                return;
            const mount = document.createElement('div');
            playerMount.current.replaceChildren(mount);
            instance = new api.Player(mount, { host: 'https://www.youtube-nocookie.com', videoId: video.youtubeId, playerVars: { origin: location.origin, controls: 1, rel: 0, playsinline: 1, cc_load_policy: 1, autoplay: 0 }, events: {
                    onReady: event => { if (disposed)
                        return; player.current = event.target; setReady(true); event.target.getIframe().title = video.title; event.target.getIframe().referrerPolicy = 'strict-origin-when-cross-origin'; const available = event.target.getAvailablePlaybackRates().filter(r => r > 0 && r <= 2); setRates(available.length ? available : [1]); },
                    onStateChange: event => { if (disposed)
                        return; setPlaying(event.data === 1); if (event.data === 0 && repeatRef.current) {
                        const target = cuesRef.current[focusRef.current];
                        if (target) {
                            event.target.seekTo(target.start, true);
                            event.target.playVideo();
                        }
                    } },
                    onError: event => { if (!disposed)
                        setPlayerError(describeYouTubeError(event.data)); }
                } });
            clock = window.setInterval(() => {
                if (disposed || !player.current)
                    return;
                const p = player.current, current = p.getCurrentTime();
                if (!Number.isFinite(current))
                    return;
                setTime(current);
                const target = cuesRef.current[focusRef.current];
                const practice = modeRef.current !== 'shadowing';
                if (target && (repeatRef.current || practice) && p.getPlayerState() === 1 && current >= target.end - .04) {
                    if (repeatRef.current) {
                        p.seekTo(target.start, true);
                        p.playVideo();
                    }
                    else
                        p.pauseVideo();
                    return;
                }
                if (!repeatRef.current && !practice) {
                    const active = cueAtTime(cuesRef.current, current);
                    if (active >= 0)
                        setFocus(active);
                }
            }, 150);
        }).catch(e => { if (!disposed)
            setPlayerError(e.message); });
        return () => { disposed = true; if (clock !== undefined)
            clearInterval(clock); instance?.destroy(); player.current = undefined; };
    }, [video.youtubeId, video.title]);
    useEffect(() => { setAnswer(''); setChecked(false); setRevealed(false); setMicError(''); if (audioURLRef.current) {
        URL.revokeObjectURL(audioURLRef.current);
        audioURLRef.current = '';
        setAudioURL('');
    } stopMic(); }, [focus, mode, data.original]);
    useEffect(() => {
        const container = list.current;
        if (!follow || !container || loading) return;
        const row = container.querySelector<HTMLElement>(`[data-cue="${focus}"]`);
        if (!row) return;
        const scrollToCurrent = () => {
            const viewport = container.getBoundingClientRect();
            const current = row.getBoundingClientRect();
            // Both rectangles use viewport coordinates; offsetTop may use different parents.
            if (current.top < viewport.top + 12 || current.bottom > viewport.bottom - 12) {
                container.scrollTo({
                    top: Math.max(0, container.scrollTop + current.top - viewport.top - container.clientTop - Math.max(12, (container.clientHeight - current.height) / 2)),
                    behavior: 'auto',
                });
            }
        };
        const frame = requestAnimationFrame(scrollToCurrent);
        const observer = new ResizeObserver(scrollToCurrent);
        observer.observe(container);
        observer.observe(row);
        return () => { cancelAnimationFrame(frame); observer.disconnect(); };
    }, [focus, follow, panel, loading, search, large, bilingual, data.original, data.translation]);
    function seek(index: number, play = true) { if (index < 0 || index >= cues.length)
        return; setFocus(index); setTime(cues[index].start); setAnswer(''); setChecked(false); setRevealed(false); if (player.current && ready) {
        player.current.seekTo(cues[index].start, true);
        if (play)
            player.current.playVideo();
    } }
    function togglePlay() { if (!ready || !player.current)
        return; if (playing)
        player.current.pauseVideo();
    else
        player.current.playVideo(); }
    async function importFile(file: File) {
        if (busy)
            return;
        setBusy(true);
        setError('');
        setNotice('');
        try {
            if (file.size > 1024 * 1024)
                throw Error('File phụ đề tối đa 1 MiB.');
            const cues = parseLearningSubtitles(await file.text());
            const track: VideoSubtitleTrack = { language: importLanguage, label: file.name.slice(0, 100), source: 'local', cues };
            const saved = canEditSubtitles && shareImport ? await videoSubtitleApi.save(token, classId, video.id, importKind, track) : track;
            if (!alive.current)
                return;
            request.current?.abort();
            requestID.current++;
            setLoading(false);
            setData(previous => ({ ...previous, [importKind]: saved }));
            if (importKind === 'original') {
                setFocus(0);
                setRepeat(false);
                setSearch('');
            }
            setNotice(saved.source === 'local' ? 'Đã mở phụ đề cho phiên học này. File chưa được chia sẻ với lớp.' : 'Đã lưu phụ đề cho các thành viên được xem video.');
        }
        catch (e) {
            if (alive.current)
                setError(e instanceof Error ? e.message : 'Không đọc được phụ đề.');
        }
        finally {
            if (alive.current)
                setBusy(false);
        }
    }
    function download(track: VideoSubtitleTrack | null) { if (!track)
        return; const url = URL.createObjectURL(new Blob([exportLearningSRT(track.cues)], { type: 'application/x-subrip;charset=utf-8' })); const link = document.createElement('a'); link.href = url; link.download = `${video.youtubeId}-${track.language || 'subtitles'}.srt`; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); }
    async function record() {
        if (recording) {
            stopMic();
            return;
        }
        if (micPending.current)
            return;
        setMicError('');
        if (!navigator.mediaDevices?.getUserMedia || !window.MediaRecorder) {
            setMicError('Trình duyệt chưa hỗ trợ thu âm. Hãy dùng Chrome/Edge trên HTTPS hoặc localhost.');
            return;
        }
        micPending.current = true;
        try {
            player.current?.pauseVideo();
            const selected = focus;
            const media = await navigator.mediaDevices.getUserMedia({ audio: true });
            if (!alive.current || modeRef.current !== 'pronunciation' || selected !== focusRef.current) {
                media.getTracks().forEach(t => t.stop());
                return;
            }
            stream.current = media;
            const recording = new MediaRecorder(media);
            recorder.current = recording;
            const chunks: BlobPart[] = [];
            recording.ondataavailable = e => { if (e.data.size)
                chunks.push(e.data); };
            recording.onstop = () => { if (micTimer.current !== undefined)
                clearTimeout(micTimer.current); media.getTracks().forEach(t => t.stop()); if (!alive.current || selected !== focusRef.current || modeRef.current !== 'pronunciation')
                return; const blob = new Blob(chunks, { type: recording.mimeType || 'audio/webm' }); if (!blob.size) {
                setMicError('Bản thu trống. Kiểm tra microphone rồi thử lại.');
                return;
            } if (audioURLRef.current)
                URL.revokeObjectURL(audioURLRef.current); audioURLRef.current = URL.createObjectURL(blob); setAudioURL(audioURLRef.current); setRecording(false); };
            recording.onerror = () => { if (alive.current) {
                setMicError('Không thu được âm thanh. Kiểm tra microphone.');
                stopMic();
            } };
            recording.start();
            setRecording(true);
            micTimer.current = window.setTimeout(() => { if (alive.current)
                stopMic(); }, 60000);
        }
        catch {
            stream.current?.getTracks().forEach(t => t.stop());
            if (alive.current) {
                setRecording(false);
                setMicError('Chưa truy cập được microphone. Kiểm tra quyền microphone của trình duyệt.');
            }
        }
        finally {
            micPending.current = false;
        }
    }
    function changeMode(next: Mode) { stopMic(); setMode(next); setRepeat(false); player.current?.pauseVideo(); if (cue)
        player.current?.seekTo(cue.start, true); }
    return createPortal(<dialog ref={dialog} className={`youtube-learning-dialog ${large ? 'large-text' : ''}`} aria-labelledby="youtube-learning-title" onCancel={e => { e.preventDefault(); onClose(); }} onKeyDown={e => {
            if ((e.target as HTMLElement).closest('input,textarea,select,button,audio') || (e.target as HTMLElement).isContentEditable)
                return;
            if (e.key === ' ') {
                e.preventDefault();
                togglePlay();
            }
            else if (e.key === 'ArrowRight') {
                e.preventDefault();
                seek(focus + 1);
            }
            else if (e.key === 'ArrowLeft') {
                e.preventDefault();
                seek(focus - 1);
            }
        }}>
  <header className="yl-header"><button className="yl-back" onClick={onClose} aria-label="Quay lại thư viện video" autoFocus>←</button><div className="yl-title"><small>{video.category || 'KHÔNG GIAN LUYỆN NGÔN NGỮ'}</small><h2 id="youtube-learning-title">{video.title}</h2></div><nav className="yl-modes" aria-label="Chế độ học">{modes.map(item => <button key={item.key} className={mode === item.key ? 'active' : ''} aria-pressed={mode === item.key} onClick={() => changeMode(item.key)}><span aria-hidden="true">{item.icon}</span>{item.name}</button>)}</nav></header>
  <div className={`yl-workspace ${!panel ? 'panel-hidden' : ''}`}>
   <main className="yl-main">
    <div className="yl-player"><div ref={playerMount} className="yl-player-mount"/>{!ready && !playerError && <div className="yl-player-loading" role="status">Đang mở trình phát YouTube…</div>}</div>
    {playerError && <div className="yl-error" role="alert">{playerError} <a href={`https://www.youtube.com/watch?v=${video.youtubeId}`} target="_blank" rel="noopener noreferrer">Mở trên YouTube ↗</a></div>}
    <div className="yl-playback-controls"><div><button disabled={!ready || !cue || focus === 0} onClick={() => seek(focus - 1)} aria-label="Câu trước">❮</button><button className="yl-play-sentence" disabled={!ready || !cue} onClick={() => seek(focus)}><span aria-hidden="true">▶</span> Nghe câu</button><button disabled={!ready || !cue || focus >= cues.length - 1} onClick={() => seek(focus + 1)} aria-label="Câu tiếp theo">❯</button><button aria-pressed={repeat} className={repeat ? 'selected' : ''} disabled={!ready || !cue} onClick={() => { setRepeat(!repeat); if (!repeat)
        seek(focus); }} title="Lặp lại câu đang chọn">↻ Lặp câu</button></div><div><span className="yl-time">{formatCueTime(time)}</span><label>Tốc độ<select aria-label="Tốc độ phát video" disabled={!ready} value={rate} onChange={e => { const value = Number(e.target.value); setRate(value); player.current?.setPlaybackRate(value); }}>{rates.map(value => <option key={value} value={value}>{value}×</option>)}</select></label>{!panel && <button onClick={() => setPanel(true)}>Mở phụ đề ☷</button>}</div></div>
    <section className="yl-current-section" aria-label="Câu đang học">
     <div className="yl-sentence-meta"><span>{cue ? `Câu ${focus + 1} / ${cues.length}` : 'Phụ đề đồng bộ'}</span><span>{cue ? `${formatCueTime(cue.start)} – ${formatCueTime(cue.end)}` : ''}</span></div>
     <div className={`yl-current-cue ${hideAnswers ? 'hidden-answer' : ''}`}><p lang={data.original?.language || undefined}>{loading ? 'Đang tải phụ đề…' : !cue ? 'Chọn phụ đề hoặc nhập file SRT/VTT để học theo từng câu.' : hideAnswers ? mode === 'exercise' && cloze ? <>{cloze.before}<span className="yl-blank">________</span>{cloze.after}</> : 'Nghe và viết lại câu bạn vừa nghe.' : cue.text}</p></div>
     {bilingual && translations[focus] && !hideAnswers && <p className="yl-current-translation" lang={data.translation?.language || undefined}>{translations[focus]}</p>}
    </section>
    {mode === 'shadowing' && <div className="yl-mode-help"><span className="yl-help-icon" aria-hidden="true">↻</span><div><strong>Nghe, bắt chước và nói cùng người bản ngữ</strong><p>Chọn một câu trong phụ đề, giảm tốc độ nếu cần và bật lặp câu để luyện nhiều lần.</p></div></div>}
    {mode === 'pronunciation' && <section className="yl-practice-card"><h3>Luyện phát âm theo câu</h3><p>Nghe câu mẫu, thu âm giọng của bạn rồi nghe lại để tự so sánh. Bản thu chỉ nằm trong phiên trình duyệt này.</p><div className="yl-practice-actions"><button disabled={!cue || !ready} onClick={() => seek(focus)}>▶ Nghe mẫu</button><button className={recording ? 'yl-record recording' : 'yl-record'} disabled={!cue} onClick={() => void record()}>{recording ? '■ Dừng thu âm' : '● Thu âm giọng của bạn'}</button></div>{micError && <p role="alert" className="yl-error">{micError}</p>}{audioURL && <audio controls src={audioURL} aria-label="Nghe lại bản thu của bạn"/>}</section>}
    {(mode === 'listening' || mode === 'exercise') && <form className="yl-practice-card" onSubmit={e => { e.preventDefault(); setChecked(true); player.current?.pauseVideo(); }}><h3>{mode === 'listening' ? 'Nghe và viết lại' : 'Điền từ còn thiếu'}</h3><p>{mode === 'listening' ? 'Phụ đề được ẩn trong lúc luyện nghe. Nghe câu mẫu rồi nhập những gì bạn nghe được.' : 'Bài tập được tạo từ câu phụ đề đang chọn. Nghe video để điền phần bị ẩn.'}</p><label>{mode === 'listening' ? 'Câu trả lời của bạn' : 'Từ / cụm từ còn thiếu'}<textarea rows={mode === 'listening' ? 3 : 2} value={answer} disabled={!cue} maxLength={3000} onChange={e => { setAnswer(e.target.value); setChecked(false); }} placeholder={mode === 'listening' ? 'Nhập câu vừa nghe…' : 'Nhập phần còn thiếu…'}/></label><div className="yl-practice-actions"><button className="yl-primary" disabled={!cue || !answer.trim() || (mode === 'exercise' && !cloze)}>Kiểm tra</button><button type="button" disabled={!cue} onClick={() => setRevealed(!revealed)}>{revealed ? 'Ẩn đáp án' : 'Hiện đáp án'}</button></div>{checked && cue && <div role="status" className={`yl-answer ${normalizePracticeAnswer(answer) === normalizePracticeAnswer(mode === 'listening' ? cue.text : cloze?.answer || '') ? 'correct' : ''}`}><strong>{normalizePracticeAnswer(answer) === normalizePracticeAnswer(mode === 'listening' ? cue.text : cloze?.answer || '') ? 'Đã khớp với phụ đề!' : 'Chưa khớp hoàn toàn. So sánh với đáp án:'}</strong><p>{mode === 'listening' ? cue.text : cloze?.answer}</p><button type="button" disabled={focus >= cues.length - 1} onClick={() => seek(focus + 1)}>Luyện câu tiếp theo →</button></div>}</form>}
    <details className="yl-import"><summary>Thêm phụ đề SRT / VTT {canEditSubtitles ? '· quản lý cho lớp' : '· dùng trong phiên học'}</summary><p>Nếu YouTube không trả phụ đề, bạn có thể mở file phụ đề có sẵn. Bản dịch cần một file riêng có mốc thời gian tương ứng.</p><div className="yl-import-fields"><label>Loại phụ đề<select value={importKind} onChange={e => { const kind = e.target.value as 'original' | 'translation'; setImportKind(kind); setImportLanguage(kind === 'translation' ? 'vi' : data.original?.language || 'ja'); }}><option value="original">Nguyên ngữ</option><option value="translation">Bản dịch</option></select></label><label>Ngôn ngữ<select value={importLanguage} onChange={e => setImportLanguage(e.target.value)}>{languages.map(([key, name]) => <option key={key} value={key}>{name}</option>)}</select></label><label className="yl-file-label">Chọn file (tối đa 1 MiB)<input type="file" accept=".srt,.vtt,text/vtt,application/x-subrip" disabled={busy} onChange={e => { const file = e.target.files?.[0]; if (file)
        void importFile(file); e.target.value = ''; }}/></label></div>{canEditSubtitles && <label className="yl-checkbox"><input type="checkbox" checked={shareImport} disabled={busy} onChange={e => setShareImport(e.target.checked)}/>Lưu phụ đề cho lớp (thay thế bản cùng loại đang lưu)</label>}{busy && <p role="status">Đang lưu phụ đề…</p>}
     {canEditSubtitles && (['original', 'translation'] as const).map(kind => data[kind]?.source === 'uploaded' && <button key={kind} className="yl-remove-subtitles" disabled={busy} onClick={async () => { if (!confirm(`Xóa phụ đề ${kind === 'original' ? 'nguyên ngữ' : 'bản dịch'} đã lưu của lớp?`))
        return; setBusy(true); setError(''); try {
        await videoSubtitleApi.remove(token, classId, video.id, kind);
        setData(previous => ({ ...previous, [kind]: null }));
        setNotice('Đã xóa phụ đề đã lưu.');
    }
    catch (e) {
        setError(e instanceof Error ? e.message : 'Không xóa được phụ đề');
    }
    finally {
        setBusy(false);
    } }}>Xóa {kind === 'original' ? 'nguyên ngữ' : 'bản dịch'} đã lưu</button>)}
    </details>
    {error && <p className="yl-error" role="alert">{error}</p>}{notice && <p className="yl-notice" role="status">{notice}</p>}
   </main>
   {panel && <aside className="yl-transcript" aria-label="Danh sách phụ đề"><div className="yl-transcript-heading"><div><h3>Phụ đề</h3><small>{data.original?.source === 'youtube' ? 'Từ YouTube' : data.original?.source === 'uploaded' ? 'Phụ đề của lớp' : data.original?.source === 'local' ? 'Phụ đề phiên học' : 'Theo dõi từng câu'}</small></div><div><button aria-pressed={bilingual} title="Bật / tắt bản dịch" aria-label="Bật hoặc tắt bản dịch" onClick={() => setBilingual(!bilingual)}>AB</button><button aria-pressed={large} title="Đổi cỡ chữ" onClick={() => setLarge(!large)}>A+</button><button disabled={!cues.length} aria-label="Tải phụ đề nguyên ngữ SRT" title="Tải phụ đề SRT" onClick={() => download(data.original)}>↓</button><button aria-label="Ẩn danh sách phụ đề" className="yl-panel-close" onClick={() => setPanel(false)}>×</button></div></div>
    <div className="yl-transcript-tools"><label className="yl-search"><span aria-hidden="true">⌕</span><input type="search" aria-label="Tìm trong phụ đề" value={search} onChange={e => setSearch(e.target.value)} placeholder="Tìm câu, từ vựng…"/></label><div><label className="yl-checkbox"><input type="checkbox" checked={follow} onChange={e => setFollow(e.target.checked)}/>Theo câu đang phát</label><span>{cues.length} câu</span></div>{!!data.availableTracks.length && <label className="yl-language-label">Ngôn ngữ YouTube<select disabled={loading} value={data.original?.source === 'youtube' ? data.original.language : ''} onChange={e => void loadSubtitles({ language: e.target.value, translation: data.translation?.language || 'vi' })}><option value="" disabled>Chọn ngôn ngữ</option>{data.availableTracks.map((track, index) => <option key={`${track.language}-${index}`} value={track.language}>{track.label}{track.automatic ? ' · tự động' : ''}</option>)}</select></label>}</div>
    <div ref={list} className="yl-cue-list">{loading ? <div className="yl-transcript-empty" role="status"><span className="yl-loading-dot"/>Đang tìm phụ đề…</div> : !cues.length ? <div className="yl-transcript-empty"><span aria-hidden="true">☷</span><h4>Chưa tải được phụ đề</h4><p>Video vẫn phát bình thường. Bạn có thể xem CC của YouTube hoặc nhập SRT/VTT ở bên dưới video để học theo câu.</p><button onClick={() => void loadSubtitles({ youtube: 1, translation: 'vi' })}>Thử lấy từ YouTube</button></div> : filtered.length === 0 ? <p className="yl-transcript-empty">Không tìm thấy câu phù hợp.</p> : filtered.map(({ cue: line, index }) => <button key={index} data-cue={index} className={`yl-cue ${index === focus ? 'active' : ''}`} aria-current={index === focus ? 'true' : undefined} disabled={!ready} onClick={() => seek(index)} aria-label={`Nghe câu ${index + 1}, ${formatCueTime(line.start)}`}><span className="yl-cue-play" aria-hidden="true">▶</span><span className="yl-cue-copy"><small>{formatCueTime(line.start)}</small><span className="yl-original" lang={data.original?.language || undefined}>{hideAnswers ? `Câu ${index + 1} · bấm để nghe` : line.text}</span>{bilingual && translations[index] && !hideAnswers && <span className="yl-translation" lang={data.translation?.language || undefined}>{translations[index]}</span>}</span></button>)}</div>
    <footer className="yl-transcript-footer"><button disabled={loading} onClick={() => void loadSubtitles({ youtube: 1, translation: 'vi' })}>{loading ? 'Đang tải…' : '↻ Lấy phụ đề YouTube'}</button>{data.translation && <button onClick={() => download(data.translation)}>↓ Bản dịch</button>}</footer>
   </aside>}
  </div>
 </dialog>, document.body);
}
