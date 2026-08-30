package http

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
)

// Handler holds all HTTP handlers for the study service.
type Handler struct {
	sets  *service.StudySetService
	cards *service.FlashcardService
	db    *sql.DB
}

// New creates a Handler wired with the given services.
func New(sets *service.StudySetService, cards *service.FlashcardService, db *sql.DB) *Handler {
	return &Handler{sets: sets, cards: cards, db: db}
}

// Register wires all routes onto mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /v1/study-sets", h.listStudySets)
	mux.HandleFunc("POST /v1/study-sets", h.createStudySet)
	// Wildcard routes for sub-resources
	mux.HandleFunc("/v1/study-sets/", h.studySetRouter)
	mux.HandleFunc("/v1/flashcards/", h.flashcardRouter)
}

// ---------- health ----------

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	body := map[string]string{"service": "study", "status": "ok", "database": "ok"}
	status := http.StatusOK
	if err := h.db.PingContext(r.Context()); err != nil {
		status = http.StatusServiceUnavailable
		body["status"] = "degraded"
		body["database"] = "offline"
	}
	WriteJSON(w, status, body)
}

// ---------- study sets ----------

func (h *Handler) listStudySets(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromHeader(r)
	sets, err := h.sets.List(r.Context(), userID)
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, sets)
}

func (h *Handler) createStudySet(w http.ResponseWriter, r *http.Request) {
	var in model.CreateStudySetInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	userID := userIDFromHeader(r)
	set, err := h.sets.Create(r.Context(), userID, in)
	if err != nil {
		if isValidationError(err) {
			WriteError(w, http.StatusBadRequest, err.Error())
		} else {
			WriteServiceError(w, err)
		}
		return
	}
	WriteJSON(w, http.StatusCreated, set)
}

func (h *Handler) studySetRouter(w http.ResponseWriter, r *http.Request) {
	parts := PathParts(r.URL.Path, "/v1/study-sets/")
	if len(parts) == 0 {
		WriteError(w, http.StatusNotFound, "study set not found")
		return
	}
	setID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid study set id")
		return
	}
	// POST /v1/study-sets/{id}/flashcards
	if len(parts) == 2 && parts[1] == "flashcards" && r.Method == http.MethodPost {
		h.createFlashcard(w, r, setID)
		return
	}
	userID := userIDFromHeader(r)
	switch r.Method {
	case http.MethodGet:
		set, err := h.sets.GetWithCards(r.Context(), setID)
		if err != nil {
			WriteServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, set)
	case http.MethodPut:
		var in model.UpdateStudySetInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		set, err := h.sets.Update(r.Context(), setID, userID, in)
		if err != nil {
			if isValidationError(err) {
				WriteError(w, http.StatusBadRequest, err.Error())
			} else {
				WriteServiceError(w, err)
			}
			return
		}
		WriteJSON(w, http.StatusOK, set)
	case http.MethodDelete:
		if err := h.sets.Delete(r.Context(), setID, userID); err != nil {
			WriteServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ---------- flashcards ----------

func (h *Handler) createFlashcard(w http.ResponseWriter, r *http.Request, studySetID int64) {
	var in model.CreateFlashcardInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	userID := userIDFromHeader(r)
	card, err := h.cards.Create(r.Context(), studySetID, userID, in)
	if err != nil {
		if isValidationError(err) {
			WriteError(w, http.StatusBadRequest, err.Error())
		} else {
			WriteServiceError(w, err)
		}
		return
	}
	WriteJSON(w, http.StatusCreated, card)
}

func (h *Handler) flashcardRouter(w http.ResponseWriter, r *http.Request) {
	parts := PathParts(r.URL.Path, "/v1/flashcards/")
	if len(parts) == 0 {
		WriteError(w, http.StatusNotFound, "flashcard not found")
		return
	}
	cardID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid flashcard id")
		return
	}
	// POST /v1/flashcards/{id}/star
	if len(parts) == 2 && parts[1] == "star" && r.Method == http.MethodPost {
		userID := userIDFromHeader(r)
		card, err := h.cards.ToggleStar(r.Context(), cardID, userID)
		if err != nil {
			WriteServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, card)
		return
	}
	userID := userIDFromHeader(r)
	switch r.Method {
	case http.MethodPut:
		var in model.UpdateFlashcardInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		card, err := h.cards.Update(r.Context(), cardID, userID, in)
		if err != nil {
			if isValidationError(err) {
				WriteError(w, http.StatusBadRequest, err.Error())
			} else {
				WriteServiceError(w, err)
			}
			return
		}
		WriteJSON(w, http.StatusOK, card)
	case http.MethodDelete:
		if err := h.cards.Delete(r.Context(), cardID, userID); err != nil {
			WriteServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ---------- helpers ----------

// userIDFromHeader reads X-User-ID injected by the gateway/auth middleware.
// Returns 0 if the header is absent (auth not yet wired in Sprint 1).
func userIDFromHeader(r *http.Request) int64 {
	raw := r.Header.Get("X-User-ID")
	if raw == "" {
		return 0
	}
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}

// isValidationError returns true for user-facing validation errors.
func isValidationError(err error) bool {
	// All validation errors come from service layer as plain errors with descriptive text.
	// They are not repository/service sentinel errors, so we check via negative assertion.
	return err != nil &&
		!errors.Is(err, service.ErrForbidden)
}
