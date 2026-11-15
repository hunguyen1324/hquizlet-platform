import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  indexKanjiWords,
  queryKanjiWords,
  type GraphWord,
} from "./kanjiWordIndex";
import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../../lib/api/client";
import { useAuth } from "../auth/AuthContext";
import { kanjiCharacters } from "./KanjiTools";
export function KanjiGraph({
  term,
  cards,
  studySetId,
}: {
  term: string;
  cards: GraphWord[];
  studySetId: number;
}) {
  const index = useMemo(() => indexKanjiWords(cards), [cards]);
  const localGroups = useMemo(
    () => queryKanjiWords(index, term),
    [index, term],
  );
  const dialog = useRef<HTMLDialogElement>(null);
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState("");
  useEffect(() => {
    dialog.current?.close();
    setOpen(false);
    setSelected("");
  }, [term]);
  useEffect(() => {
    if (open) dialog.current?.showModal();
  }, [open]);
  const { token, user } = useAuth();
  const query = useQuery({
    queryKey: ["kanji-word-graph", user?.id, studySetId, term],
    enabled: open && !!token,
    staleTime: 30000,
    queryFn: ({ signal }) =>
      apiFetch<{ groups: typeof localGroups; truncated: boolean }>(
        `/v1/study-sets/${studySetId}/kanji-graph`,
        token,
        { signal },
        { term },
      ),
  });
  const groups = query.data?.groups ?? localGroups;
  if (!kanjiCharacters(term).length) return null;
  const group = groups.find((g) => g.char === selected) ?? groups[0];
  return (
    <div className="kanji-tools">
      <button type="button" onClick={() => setOpen(true)}>
        ♧ Kanji graph
      </button>
      {open &&
        createPortal(
          <dialog
            className="kanji-dialog"
            ref={dialog}
            onClose={() => setOpen(false)}
            aria-label={`Kanji graph: ${term}`}
          >
            <header>
              <h2>Kanji graph · {term}</h2>
              <button
                type="button"
                aria-label="Đóng đồ thị"
                onClick={() => dialog.current?.close()}
              >
                ×
              </button>
            </header>
            <p>
              Các từ cùng chữ Hán trong học phần hiện tại và các học phần của
              bạn. Chọn chữ để khám phá liên kết.
            </p>
            {query.isFetching && <p role="status">Đang tìm liên kết…</p>}
            {query.isError && (
              <p role="alert">
                Chưa tải được đồ thị thư viện; đang hiển thị thẻ đã tải trong
                học phần.{" "}
                <button type="button" onClick={() => void query.refetch()}>
                  Thử lại
                </button>
              </p>
            )}
            {query.data?.truncated && <p>Hiển thị tối đa 200 liên kết.</p>}
            <div className="kanji-controls">
              {groups.map((g) => (
                <button
                  type="button"
                  aria-pressed={group.char === g.char}
                  key={g.char}
                  onClick={() => setSelected(g.char)}
                >
                  {g.char} · {g.words.length}
                </button>
              ))}
            </div>
            <ul className="kanji-word-graph">
              {group.words.slice(0, 100).map((word) => (
                <li key={word.id}>
                  <strong>{group.char}</strong>
                  <span aria-hidden="true">→</span>
                  <div>
                    <b lang="ja">{word.term}</b>
                    <small>{word.definition}</small>
                  </div>
                </li>
              ))}
            </ul>
            {group.words.length > 100 && (
              <p>
                Hiển thị 100 / {group.words.length} từ. Dùng tìm kiếm thẻ để tìm
                thêm.
              </p>
            )}
            {!group.words.length && <p>Chưa có từ liên quan trong học phần.</p>}
          </dialog>,
          document.body,
        )}
    </div>
  );
}
