package http

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
)

// Handler holds all HTTP handlers for the study service.
type Handler struct {
	sets    *service.StudySetService
	cards   *service.FlashcardService
	folders *service.FolderService
	db      *sql.DB
}

// New creates a Handler wired with the given services.
func New(sets *service.StudySetService, cards *service.FlashcardService, folders *service.FolderService, db *sql.DB) *Handler {
	return &Handler{sets: sets, cards: cards, folders: folders, db: db}
}

// Register wires all routes onto mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.health)

	// Study sets
	mux.HandleFunc("GET /v1/study-sets", h.listStudySets)
	mux.HandleFunc("POST /v1/study-sets", h.createStudySet)
	mux.HandleFunc("/v1/study-sets/", h.studySetRouter)

	// Flashcards
	mux.HandleFunc("/v1/flashcards/", h.flashcardRouter)

	// Folders
	mux.HandleFunc("GET /v1/folders", h.listFolders)
	mux.HandleFunc("POST /v1/folders", h.createFolder)
	mux.HandleFunc("/v1/folders/", h.folderRouter)
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

// listStudySets handles GET /v1/study-sets with optional query params:
//
//	?search=<title>  filter by title substring
//	?sort=<updated|created|title>  sort order (default: updated)
//	?page=<n>        1-based page number (default: 1)
//	?per_page=<n>    items per page (default: 20, max: 100)
func (h *Handler) listStudySets(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromHeader(r)
	q := r.URL.Query()

	filter := model.StudySetFilter{
		Search:  strings.TrimSpace(q.Get("search")),
		SortBy:  q.Get("sort"),
		Page:    intQueryParam(q.Get("page"), 1),
		PerPage: intQueryParam(q.Get("per_page"), 20),
	}

	result, err := h.sets.ListWithFilter(r.Context(), userID, filter)
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, result)
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
	if len(parts) == 2 && parts[1] == "flashcards" {
		if r.Method == http.MethodPost {
			h.createFlashcard(w, r, setID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// POST /v1/study-sets/{id}/flashcards/bulk
	if len(parts) == 3 && parts[1] == "flashcards" && parts[2] == "bulk" {
		if r.Method == http.MethodPost {
			h.bulkSaveFlashcards(w, r, setID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
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

// bulkSaveFlashcards handles POST /v1/study-sets/{id}/flashcards/bulk
func (h *Handler) bulkSaveFlashcards(w http.ResponseWriter, r *http.Request, studySetID int64) {
	var in model.BulkSaveFlashcardsInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	userID := userIDFromHeader(r)
	result, err := h.cards.BulkSave(r.Context(), studySetID, userID, in)
	if err != nil {
		if isValidationError(err) {
			WriteError(w, http.StatusBadRequest, err.Error())
		} else {
			WriteServiceError(w, err)
		}
		return
	}
	WriteJSON(w, http.StatusOK, result)
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

// ---------- folders ----------

func (h *Handler) listFolders(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromHeader(r)
	folders, err := h.folders.List(r.Context(), userID)
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, folders)
}

func (h *Handler) createFolder(w http.ResponseWriter, r *http.Request) {
	var in model.CreateFolderInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	userID := userIDFromHeader(r)
	folder, err := h.folders.Create(r.Context(), userID, in)
	if err != nil {
		if isValidationError(err) {
			WriteError(w, http.StatusBadRequest, err.Error())
		} else {
			WriteServiceError(w, err)
		}
		return
	}
	WriteJSON(w, http.StatusCreated, folder)
}

func (h *Handler) folderRouter(w http.ResponseWriter, r *http.Request) {
	parts := PathParts(r.URL.Path, "/v1/folders/")
	if len(parts) == 0 {
		WriteError(w, http.StatusNotFound, "folder not found")
		return
	}
	folderID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid folder id")
		return
	}

	userID := userIDFromHeader(r)

	// POST /v1/folders/{id}/study-sets
	if len(parts) == 2 && parts[1] == "study-sets" && r.Method == http.MethodPost {
		var body struct {
			StudySetID int64 `json:"studySetId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := h.folders.AddStudySet(r.Context(), folderID, body.StudySetID, userID); err != nil {
			WriteServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	// DELETE /v1/folders/{id}/study-sets/{setId}
	if len(parts) == 3 && parts[1] == "study-sets" && r.Method == http.MethodDelete {
		setID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid study set id")
			return
		}
		if err := h.folders.RemoveStudySet(r.Context(), folderID, setID, userID); err != nil {
			WriteServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	switch r.Method {
	case http.MethodGet:
		folder, err := h.folders.GetWithStudySets(r.Context(), folderID, userID)
		if err != nil {
			WriteServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, folder)
	case http.MethodPut:
		var in model.UpdateFolderInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		folder, err := h.folders.Update(r.Context(), folderID, userID, in)
		if err != nil {
			if isValidationError(err) {
				WriteError(w, http.StatusBadRequest, err.Error())
			} else {
				WriteServiceError(w, err)
			}
			return
		}
		WriteJSON(w, http.StatusOK, folder)
	case http.MethodDelete:
		if err := h.folders.Delete(r.Context(), folderID, userID); err != nil {
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
// Returns 0 if the header is absent.
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
	return err != nil &&
		!errors.Is(err, service.ErrForbidden)
}

// intQueryParam parses an int query param with a fallback default.
func intQueryParam(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
