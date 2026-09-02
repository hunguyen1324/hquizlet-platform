// PlaySection — 3 fixed cards: Flashcards, Học, Ghép thẻ
import React from "react";
import { Card } from "../ui";

type Props = {
  onNavigate: (view: string) => void;
};

const modes = [
  {
    id: "flashcards",
    title: "Flashcards",
    description: "Ôn tập thẻ ghi nhớ",
    button: "Ôn tập",
    iconBg: "from-sky-400 to-blue-500",
    icon: "🃏",
  },
  {
    id: "learn",
    title: "Học",
    description: "Học qua câu hỏi trắc nghiệm",
    button: "Học ngay",
    iconBg: "from-violet-400 to-purple-500",
    icon: "🎓",
  },
  {
    id: "match",
    title: "Ghép thẻ",
    description: "Thử thách ghép thẻ nhanh",
    button: "Chơi",
    iconBg: "from-emerald-400 to-teal-500",
    icon: "🎯",
  },
];

export function PlaySection({ onNavigate }: Props) {
  return (
    <section className="mb-8">
      <h2 className="text-lg font-bold mb-3">Chơi và ôn tập</h2>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {modes.map((mode) => (
          <Card key={mode.id} className="p-5 flex flex-col gap-3">
            <div
              className={`w-12 h-12 rounded-xl bg-gradient-to-br ${mode.iconBg} flex items-center justify-center text-2xl text-white`}
            >
              {mode.icon}
            </div>
            <h3 className="text-sm font-bold">{mode.title}</h3>
            <p className="text-xs text-[var(--muted-foreground)]">{mode.description}</p>
            <button
              onClick={() => onNavigate(mode.id)}
              className="mt-auto px-4 py-2 rounded-xl bg-[var(--primary)] text-white text-sm font-bold hover:opacity-90 transition-opacity"
            >
              {mode.button}
            </button>
          </Card>
        ))}
      </div>
    </section>
  );
}
