import React from "react";
import { useQuery } from "@tanstack/react-query";
import "./kanji.css";

type Component = { char: string | null; position: string | null; radical: boolean; phonetic: boolean; children: Component[] };
type KanjiData = { strokes: string[]; numbers: { x: number; y: number; n: string }[]; tree: Component };
const positions: Record<string, string> = { left: "trái", right: "phải", top: "trên", bottom: "dưới", nyo: "bao quanh", nyou: "bao quanh", tare: "bao phía trên", kamae: "bao quanh" };
export function kanjiCharacters(term: string) {
  return [...new Set(Array.from(term).filter(char => /\p{Unified_Ideograph}/u.test(char)))];
}

function ComponentTree({ node }: { node: Component }) {
  const children = <ul>{node.children.map((child, index) => <li key={index}><ComponentTree node={child} /></li>)}</ul>;
  const label = <><strong>{node.char}</strong>{node.radical && <span> · Bộ thủ</span>}{node.phonetic && <span> · Thành phần âm</span>}{node.position && <small> · {positions[node.position] ?? node.position}</small>}</>;
  if (!node.char) return children;
  return node.children.length ? <details open><summary>{label}</summary>{children}</details> : <div>{label}</div>;
}

function KanjiViewer({ char, mode }: { char: string; mode: "components" | "strokes" }) {
  const [step, setStep] = React.useState(0);
  const [playing, setPlaying] = React.useState(false);
  const [speed, setSpeed] = React.useState(1);
  const [numbers, setNumbers] = React.useState(true);
  const shard = char.codePointAt(0)!.toString(16).padStart(5, "0").slice(0, -2);
  const query = useQuery({ queryKey: ["kanji-data", shard], staleTime: Infinity, retry: false, queryFn: async ({ signal }) => {
    const response = await fetch(`${import.meta.env.BASE_URL}kanji/${shard}.json`, { signal });
    if (response.status === 404) return {} as Record<string, KanjiData>;
    if (!response.ok) throw new Error("Không tải được dữ liệu Kanji.");
    return await response.json() as Record<string, KanjiData>;
  } });
  const data = query.data?.[char];
  const total = data?.strokes.length ?? 0;
  React.useEffect(() => {
    if (!playing || !total) return;
    const timer = window.setTimeout(() => {
      if (step >= total) setPlaying(false);
      else setStep(step + 1);
    }, 800 / speed);
    return () => window.clearTimeout(timer);
  }, [playing, step, speed, total]);
  if (query.isPending) return <p role="status">Đang tải dữ liệu…</p>;
  if (query.isError) return <p role="alert">Không tải được dữ liệu. <button type="button" onClick={() => void query.refetch()}>Thử lại</button></p>;
  if (!data) return <p>Chưa có dữ liệu cho chữ {char}.</p>;
  if (mode === "components") return <div className="kanji-tree"><ComponentTree node={data.tree} /><p>Bấm vào thành phần để thu gọn hoặc mở rộng.</p></div>;
  return <>
    <svg className="kanji-canvas" viewBox="0 0 109 109" role="img" aria-label={`Cách viết ${char}, nét ${step} trên ${total}`}>
      <path d="M0 54.5H109M54.5 0V109M0 0L109 109M109 0L0 109" stroke="#dfe5ef" strokeWidth=".5" strokeDasharray="2 2" />
      <g fill="none" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
        {data.strokes.map((d, index) => <path key={`guide-${index}`} d={d} stroke="#e2e8f0" />)}
        {data.strokes.slice(0, step).map((d, index) => <path key={index === step - 1 ? `active-${step}-${playing}` : index} d={d} stroke={index === step - 1 ? "#6366f1" : "currentColor"} pathLength="1" className={playing && index === step - 1 ? "kanji-active-stroke" : undefined} style={{ animationDuration: `${700 / speed}ms` }} />)}
      </g>
      {numbers && data.numbers.slice(0, step).map((number, index) => <text key={index} x={number.x} y={number.y} fontSize="5" fill="currentColor">{number.n}</text>)}
    </svg>
    <p role="status">Nét {step} / {total}</p>
    <div className="kanji-controls">
      <button type="button" onClick={() => { setPlaying(false); setStep(0); }}>Viết lại</button>
      <button type="button" disabled={!step} onClick={() => { setPlaying(false); setStep(step - 1); }}>Nét trước</button>
      <button type="button" onClick={() => { if (!playing) { if (step >= total) setStep(1); else if (!step) setStep(1); } setPlaying(!playing); }}>{playing ? "Tạm dừng" : "Phát"}</button>
      <button type="button" disabled={step >= total} onClick={() => { setPlaying(false); setStep(step + 1); }}>Nét sau</button>
      <label>Tốc độ <select value={speed} onChange={event => setSpeed(Number(event.target.value))}>{[0.5, 1, 1.5, 2].map(value => <option key={value} value={value}>{value}×</option>)}</select></label>
      <label><input type="checkbox" checked={numbers} onChange={event => setNumbers(event.target.checked)} /> Số thứ tự nét</label>
    </div>
  </>;
}

export function KanjiTools({ term }: { term: string }) {
  const chars = kanjiCharacters(term);
  const [selected, setSelected] = React.useState<{ char: string; mode: "components" | "strokes" } | null>(null);
  const dialog = React.useRef<HTMLDialogElement>(null);
  React.useEffect(() => { if (selected) dialog.current?.showModal(); }, [selected]);
  React.useEffect(() => { dialog.current?.close(); setSelected(null); }, [term]);
  if (!chars.length) return null;
  return <div className="kanji-tools" onClick={event => event.stopPropagation()}>
    {chars.map(char => <div className="kanji-tool-pair" key={char}><button type="button" onClick={() => setSelected({ char, mode: "components" })}>Bộ thủ {char}</button><button type="button" onClick={() => setSelected({ char, mode: "strokes" })}>Cách viết {char}</button></div>)}
    <dialog ref={dialog} className="kanji-dialog" onClose={() => setSelected(null)} onKeyDown={event => event.stopPropagation()} onClick={event => { if (event.target === event.currentTarget) dialog.current?.close(); }}>
      {selected && <><header><h2>{selected.mode === "components" ? "Bộ thủ và thành phần" : "Thứ tự nét viết"}: {selected.char}</h2><button type="button" aria-label="Đóng" onClick={() => dialog.current?.close()}>×</button></header>
        <KanjiViewer key={`${selected.char}-${selected.mode}`} {...selected} />
        <footer>Dữ liệu: <a href="https://kanjivg.tagaini.net/" target="_blank" rel="noreferrer">KanjiVG</a> · Ulrich Apel và cộng tác viên · <a href={`${import.meta.env.BASE_URL}kanji/LICENSE.txt`}>CC BY-SA 3.0</a></footer></>}
    </dialog>
  </div>;
}
