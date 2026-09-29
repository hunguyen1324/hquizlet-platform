export function ExamTimer({ secondsLeft }: { secondsLeft: number }) {
  const h = Math.floor(secondsLeft / 3600);
  const m = Math.floor((secondsLeft % 3600) / 60);
  const s = secondsLeft % 60;
  const low = secondsLeft > 0 && secondsLeft <= 300;
  const text = `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;

  return (
    <span
      className={`font-mono text-sm font-bold tabular-nums ${low ? "text-red-400" : "text-[#ff9900]"}`}
    >
      {text}
    </span>
  );
}
