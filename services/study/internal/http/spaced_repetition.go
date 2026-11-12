package http

import (
	"encoding/json"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"net/http"
)

func (h *Handler) spacedRepetition(w http.ResponseWriter, r *http.Request, setID int64) {
	uid := userIDFromHeader(r)
	if err := h.progress.Authorize(r.Context(), uid, setID); err != nil {
		WriteServiceError(w, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		var fresh, learning, almost, mastered, due int
		err := h.db.QueryRowContext(r.Context(), `SELECT
   count(*) FILTER (WHERE s.flashcard_id IS NULL),
   count(*) FILTER (WHERE s.repetitions < 2),
   count(*) FILTER (WHERE s.repetitions >= 2 AND s.repetitions < 4),
   count(*) FILTER (WHERE s.repetitions >= 4),
   count(*) FILTER (WHERE s.due_at <= now())
   FROM flashcards f LEFT JOIN spaced_repetition s ON s.flashcard_id=f.id AND s.user_id=$1
   WHERE f.study_set_id=$2`, uid, setID).Scan(&fresh, &learning, &almost, &mastered, &due)
		if err != nil {
			WriteServiceError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]int{"newCards": fresh, "learning": learning, "almostMastered": almost, "mastered": mastered, "due": due})
	case http.MethodPost:
		var in struct {
			FlashcardID int64  `json:"flashcardId"`
			Rating      string `json:"rating"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in) != nil || in.FlashcardID <= 0 || (in.Rating != "again" && in.Rating != "hard" && in.Rating != "good" && in.Rating != "easy") {
			WriteError(w, 400, "Đánh giá thẻ không hợp lệ")
			return
		}
		// UPSERT serializes concurrent reviews. Repeated taps before due time do not inflate mastery.
		result, err := h.db.ExecContext(r.Context(), `INSERT INTO spaced_repetition(user_id,flashcard_id,repetitions,interval_days,due_at)
   SELECT $1,id,CASE WHEN $3 IN ('good','easy') THEN 1 ELSE 0 END,
    CASE WHEN $3='easy' THEN 4 WHEN $3='good' THEN 1 ELSE 0 END,
    now()+CASE WHEN $3='easy' THEN interval '4 days' WHEN $3='good' THEN interval '1 day' WHEN $3='hard' THEN interval '10 minutes' ELSE interval '1 minute' END
   FROM flashcards WHERE id=$2 AND study_set_id=$4
   ON CONFLICT(user_id,flashcard_id) DO UPDATE SET
    repetitions=CASE WHEN $3='again' THEN 0 WHEN $3='hard' THEN spaced_repetition.repetitions ELSE spaced_repetition.repetitions+1 END,
    interval_days=CASE WHEN $3 IN ('again','hard') THEN 0 ELSE LEAST(365,GREATEST(1,spaced_repetition.interval_days)*CASE WHEN $3='easy' THEN 4 ELSE 2 END) END,
    due_at=now()+CASE WHEN $3='again' THEN interval '1 minute' WHEN $3='hard' THEN interval '10 minutes'
     ELSE LEAST(365,GREATEST(1,spaced_repetition.interval_days)*CASE WHEN $3='easy' THEN 4 ELSE 2 END)*interval '1 day' END,
    reviewed_at=now()
   WHERE spaced_repetition.due_at<=now()`, uid, in.FlashcardID, in.Rating, setID)
		if err != nil {
			WriteServiceError(w, err)
			return
		}
		n, _ := result.RowsAffected()
		if n == 0 { // distinguish a future review from a foreign card
			var exists bool
			if err = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM flashcards WHERE id=$1 AND study_set_id=$2)`, in.FlashcardID, setID).Scan(&exists); err != nil {
				WriteServiceError(w, err)
				return
			}
			if !exists {
				WriteServiceError(w, repository.ErrNotFound)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		_, err := h.db.ExecContext(r.Context(), `DELETE FROM spaced_repetition s USING flashcards f WHERE s.flashcard_id=f.id AND s.user_id=$1 AND f.study_set_id=$2`, uid, setID)
		if err != nil {
			WriteServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		w.WriteHeader(405)
	}
}
