package http

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode"
)

func (h *Handler) relatedKanji(w http.ResponseWriter, r *http.Request, setID int64) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		w.WriteHeader(405)
		return
	}
	if err := h.progress.Authorize(r.Context(), userIDFromHeader(r), setID); err != nil {
		WriteServiceError(w, err)
		return
	}
	term := strings.TrimSpace(r.URL.Query().Get("term"))
	if len([]rune(term)) > 40 {
		WriteError(w, 400, "Thuật ngữ quá dài")
		return
	}
	chars := []string{}
	seen := map[rune]bool{}
	for _, c := range term {
		if unicode.Is(unicode.Han, c) && !seen[c] {
			chars = append(chars, string(c))
			seen[c] = true
		}
	}
	if len(chars) == 0 {
		WriteJSON(w, 200, map[string]any{"characters": []string{}, "groups": []any{}, "truncated": false})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.db.QueryContext(ctx, `SELECT DISTINCT k.char,f.id,f.term,f.definition,f.study_set_id FROM kanji_word_index k JOIN flashcards f ON f.id=k.flashcard_id JOIN study_sets s ON s.id=f.study_set_id WHERE k.char=ANY($1::text[]) AND (f.study_set_id=$2 OR s.user_id=$3) AND NOT s.moderation_hidden ORDER BY k.char,f.id LIMIT 201`, "{"+strings.Join(chars, ",")+"}", setID, userIDFromHeader(r))
	if err != nil {
		WriteError(w, 503, "Không tải được Kanji graph")
		return
	}
	defer rows.Close()
	type word struct {
		ID         int64  `json:"id"`
		Term       string `json:"term"`
		Definition string `json:"definition"`
		StudySetID int64  `json:"studySetId"`
	}
	type relation struct {
		Char  string `json:"char"`
		Words []word `json:"words"`
	}
	groups := []relation{}
	byChar := map[string]int{}
	for _, char := range chars {
		byChar[char] = len(groups)
		groups = append(groups, relation{char, []word{}})
	}
	count := 0
	truncated := false
	for rows.Next() {
		var char string
		var entry word
		if err = rows.Scan(&char, &entry.ID, &entry.Term, &entry.Definition, &entry.StudySetID); err != nil {
			WriteError(w, 500, "Dữ liệu đồ thị lỗi")
			return
		}
		count++
		if count > 200 {
			truncated = true
			break
		}
		i := byChar[char]
		groups[i].Words = append(groups[i].Words, entry)
	}
	if rows.Err() != nil {
		WriteError(w, 503, "Không tải được đồ thị")
		return
	}
	if strings.HasSuffix(r.URL.Path, "/kanji-graph") {
		WriteJSON(w, 200, map[string]any{"groups": groups, "truncated": truncated})
		return
	}
	related := []string{}
	for _, g := range groups {
		for _, w := range g.Words {
			for _, c := range w.Term {
				if unicode.Is(unicode.Han, c) && !seen[c] {
					seen[c] = true
					related = append(related, string(c))
				}
			}
		}
	}
	WriteJSON(w, 200, map[string]any{"characters": related})
}
