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
	if h.graph == nil {
		WriteError(w, 503, "Neo4j chưa được cấu hình")
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
		WriteJSON(w, 200, map[string]any{"characters": []string{}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	related, err := h.graph.Related(ctx, chars)
	if err != nil {
		WriteError(w, 503, "Không kết nối được Neo4j. Vui lòng thử lại.")
		return
	}
	WriteJSON(w, 200, map[string]any{"characters": related})
}
