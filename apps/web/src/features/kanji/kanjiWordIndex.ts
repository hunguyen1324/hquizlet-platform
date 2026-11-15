import { kanjiCharacters } from "./KanjiTools";
export type GraphWord = { id: number; term: string; definition: string };
// Inverted index: each word is visited once when the deck changes.
export function indexKanjiWords(cards: GraphWord[]) {
  const index = new Map<string, GraphWord[]>();
  for (const card of cards)
    for (const char of kanjiCharacters(card.term)) {
      const bucket = index.get(char);
      if (bucket) bucket.push(card);
      else index.set(char, [card]);
    }
  return index;
}
export function queryKanjiWords(index: Map<string, GraphWord[]>, term: string) {
  return kanjiCharacters(term).map((char) => ({
    char,
    words: index.get(char) ?? [],
  }));
}
