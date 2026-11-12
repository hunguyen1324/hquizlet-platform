import React from "react";
import { useAuth } from "../auth/AuthContext";
import { protectedQuizApi } from "../../lib/api/protectedQuiz";
import { listeningApi, type TranscriptionJob } from "../../lib/api/listening";
import { MAX_SRT_BYTES, normalizeSubtitleText, parseSubtitleText } from "../../lib/subtitles";

type Props = {
  audioUrl: string;
  srtText: string;
  studySetId?: number;
  ensureStudySet: () => Promise<number>;
  onChange: (patch: { audioUrl?: string; srtText?: string }) => void;
  onBusyChange: (busy: boolean) => void;
  disabled: boolean;
};

export function ListeningMediaEditor({ audioUrl, srtText, studySetId, ensureStudySet, onChange, onBusyChange, disabled }: Props) {
  const { token } = useAuth();
  const [language, setLanguage] = React.useState("auto");
  const [busy, setBusy] = React.useState(false);
  const [preview, setPreview] = React.useState("");
  const [error, setError] = React.useState("");
  const [jobError, setJobError] = React.useState("");
  const [job, setJob] = React.useState<TranscriptionJob | null>(null);
  const [generated, setGenerated] = React.useState("");
  const [checking, setChecking] = React.useState(false);
  const active = React.useRef(true);
  const source = React.useRef(audioUrl);
  const currentSrt = React.useRef(srtText);
  const change = React.useRef(onChange);
  const baseline = React.useRef<string | null>(null);
  const previewAbort = React.useRef<AbortController | null>(null);
  const operation = React.useRef(0);
  source.current = audioUrl; currentSrt.current = srtText; change.current = onChange;
  React.useEffect(() => { active.current = true; return () => { active.current = false; previewAbort.current?.abort(); }; }, []);
  React.useEffect(() => { onBusyChange(busy); return () => onBusyChange(false); }, [busy, onBusyChange]);
  React.useEffect(() => () => { if (preview) URL.revokeObjectURL(preview); }, [preview]);
  React.useEffect(() => {
    operation.current++;
    previewAbort.current?.abort(); setPreview(""); setJob(null); setGenerated(""); setJobError(""); baseline.current = null;
    if (!studySetId || !audioUrl.startsWith("private-audio:")) return;
    let mounted = true;
    const controller = new AbortController();
    void listeningApi.latest(token, studySetId, audioUrl, controller.signal).then(({ job: value }) => {
      if (!mounted) return;
      setJob(value); if (value?.srtText) setGenerated(value.srtText);
    }).catch(() => { /* Manual subtitle upload remains available when STT is offline. */ });
    return () => { mounted = false; controller.abort(); };
  }, [audioUrl, studySetId, token]);
  React.useEffect(() => {
    if (!job || !studySetId || job.status === "completed" || job.status === "failed") return;
    let mounted = true;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      try {
        const { job: value } = await listeningApi.job(token, studySetId, job.id, controller.signal);
        if (!mounted) return;
        setJob(value); setJobError("");
        if (value.status === "completed" && value.srtText) {
          setGenerated(value.srtText);
          if (baseline.current !== null && currentSrt.current === baseline.current) change.current({ srtText: value.srtText });
          return;
        }
        if (value.status === "failed") return;
      } catch (value) { if (mounted) setJobError(value instanceof Error ? value.message : "Chưa lấy được trạng thái. Đang thử lại…"); }
      if (mounted) timer = setTimeout(() => void poll(), 4000);
    };
    void poll();
    return () => { mounted = false; controller.abort(); clearTimeout(timer); };
  }, [job?.id, job?.status, studySetId, token]);

  async function upload(file?: File) {
    if (!file) return;
    if (file.size === 0 || file.size > 30 * 1024 * 1024) { setError("File nghe phải từ 1 byte đến 30 MiB."); return; }
    setBusy(true); setError("");
    try {
      const id = await ensureStudySet();
      const result = await protectedQuizApi.uploadAudio(token, id, file);
      if (active.current) change.current({ audioUrl: result.audioUrl, srtText: "" });
    } catch (value) { if (active.current) setError(value instanceof Error ? value.message : "Không tải được file nghe."); }
    finally { if (active.current) setBusy(false); }
  }
  async function importSrt(file?: File) {
    if (!file) return;
    setError("");
    try {
      if (!/\.srt$/i.test(file.name) || file.size > MAX_SRT_BYTES) throw new Error("Chọn file .srt UTF-8 không quá 1 MiB.");
      const raw = new TextDecoder("utf-8", { fatal: true }).decode(await file.arrayBuffer());
      const normalized = normalizeSubtitleText(raw);
      if (!normalized) throw new Error("File SRT không có nội dung.");
      if (active.current) change.current({ srtText: normalized });
    } catch (value) { if (active.current) setError(value instanceof Error ? value.message : "SRT không hợp lệ."); }
  }
  async function listen() {
    if (!studySetId) return;
    const expected = audioUrl;
    previewAbort.current?.abort(); const controller = new AbortController(); previewAbort.current = controller;
    setChecking(true); setError("");
    try {
      const blob = await listeningApi.preview(token, studySetId, audioUrl, controller.signal);
      if (active.current && source.current === expected) setPreview(URL.createObjectURL(blob));
    } catch (value) { if (!controller.signal.aborted && active.current) setError(value instanceof Error ? value.message : "Không nghe thử được."); }
    finally { if (active.current) setChecking(false); }
  }
  async function start() {
    if (!studySetId) return;
    const expected = audioUrl, version = operation.current;
    setBusy(true); setError(""); setJobError(""); baseline.current = currentSrt.current;
    try {
      const result = await listeningApi.start(token, studySetId, audioUrl, language);
      if (active.current && source.current === expected && operation.current === version) setJob(result);
    } catch (value) { if (active.current) setError(value instanceof Error ? value.message : "Không tạo được phụ đề."); }
    finally { if (active.current) setBusy(false); }
  }
  let subtitleError = "", cues = 0;
  try { cues = parseSubtitleText(srtText).length; } catch (value) { subtitleError = value instanceof Error ? value.message : "SRT không hợp lệ."; }
  const running = job?.status === "queued" || job?.status === "running";
  return <section className="listening-editor" aria-label="File nghe và phụ đề">
    <header><strong>File nghe &amp; phụ đề</strong><span>Upload SRT hoặc tạo phụ đề bằng Whisper</span></header>
    <label>File nghe (tối đa 30 MiB)
      <input type="file" accept="audio/*,.mp3,.wav,.m4a,.ogg,.flac,.aac,.webm" disabled={disabled || busy} onChange={event => { void upload(event.target.files?.[0]); event.target.value = ""; }} />
    </label>
    {audioUrl && <div className="listening-editor__audio">
      <span>Đã gắn file nghe</span>
      {audioUrl.startsWith("private-audio:") && <button type="button" className="secondary-button" disabled={disabled || checking || busy} onClick={() => void listen()}>{checking ? "Đang tải…" : "Nghe thử"}</button>}
      <button type="button" className="ghost-button" disabled={disabled || busy} onClick={() => change.current({ audioUrl: "", srtText: "" })}>Gỡ file nghe</button>
      {preview && <audio controls preload="metadata" src={preview} />}
    </div>}
    <small>Đổi hoặc gỡ file nghe sẽ xóa phụ đề đang gắn để tránh lệch nội dung.</small>
    <div className="listening-editor__stt">
      <label>Ngôn ngữ file nghe<select value={language} disabled={disabled || busy || running} onChange={event => setLanguage(event.target.value)}><option value="auto">Tự phát hiện</option><option value="vi">Tiếng Việt</option><option value="ja">Tiếng Nhật</option><option value="en">Tiếng Anh</option></select></label>
      <button type="button" className="secondary-button" disabled={disabled || busy || running || !studySetId || !audioUrl.startsWith("private-audio:")} onClick={() => void start()}>{running ? "Đang tạo phụ đề…" : "Tạo phụ đề bằng STT"}</button>
    </div>
    <small>Audio gửi tới một trong hai máy STT. Tối đa 20 phút; xử lý có thể mất vài phút hoặc lâu hơn.</small>
    {busy && <p role="status">Đang tải / gửi dữ liệu…</p>}
    {job && <p role="status">{job.status === "queued" ? "Đang chờ máy STT nhận việc." : job.status === "running" ? "Đang nhận dạng giọng nói… Bạn có thể lưu bài và quay lại sau." : job.status === "completed" ? "Đã tạo xong phụ đề. Hãy kiểm tra rồi lưu câu hỏi." : "Tạo phụ đề thất bại. Kiểm tra audio và thử lại."}</p>}
    {jobError && <p role="status">{jobError}</p>}
    {generated && generated !== srtText && <button type="button" disabled={disabled || busy} className="secondary-button" onClick={() => change.current({ srtText: generated })}>Dùng phụ đề STT đã tạo</button>}
    <label>Tải phụ đề có sẵn (.srt, UTF-8)
      <input type="file" accept=".srt" disabled={disabled || busy} onChange={event => { void importSrt(event.target.files?.[0]); event.target.value = ""; }} />
    </label>
    <label>Nội dung SRT — có thể dán hoặc chỉnh sửa trực tiếp
      <textarea rows={7} spellCheck={false} value={srtText} disabled={disabled || busy} onChange={event => change.current({ srtText: event.target.value })} placeholder={'1\n00:00:00,000 --> 00:00:03,000\nNội dung lời thoại…'} />
    </label>
    <div className="listening-editor__footer"><span>{cues} đoạn phụ đề</span>{srtText && <button type="button" className="ghost-button" disabled={disabled || busy} onClick={() => { const blob = new Blob([srtText], { type: "application/x-subrip;charset=utf-8" }); const link = document.createElement("a"); link.href = URL.createObjectURL(blob); link.download = "subtitle.srt"; link.click(); setTimeout(() => URL.revokeObjectURL(link.href), 1000); }}>Tải SRT</button>}{srtText && <button type="button" className="ghost-button" disabled={disabled || busy} onClick={() => change.current({ srtText: "" })}>Xóa phụ đề</button>}</div>
    {subtitleError && <p role="alert">{subtitleError}</p>}
    {error && <p role="alert">{error}</p>}
    <details><summary>Nguồn âm thanh hiện tại</summary><input aria-label="Nguồn âm thanh" value={audioUrl} disabled={disabled || busy} onChange={event => change.current({ audioUrl: event.target.value, srtText: "" })} /></details>
  </section>;
}
