// Package http contains HTTP handlers for the study service.
package http

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/templates"
)

type Handler struct {
	sets          *service.StudySetService
	cards         *service.FlashcardService
	folders       *service.FolderService
	progress      *service.ProgressService
	quizQuestions *service.QuizQuestionService
	importSvc     *service.ImportService
	db            *sql.DB
}

func New(sets *service.StudySetService, cards *service.FlashcardService, folders *service.FolderService, progress *service.ProgressService, quizQuestions *service.QuizQuestionService, importSvc *service.ImportService, db *sql.DB) *Handler {
	return &Handler{sets: sets, cards: cards, folders: folders, progress: progress, quizQuestions: quizQuestions, importSvc: importSvc, db: db}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /v1/study-sets", h.listStudySets)
	mux.HandleFunc("POST /v1/study-sets", h.createStudySet)
	mux.HandleFunc("/v1/study-sets/", h.studySetRouter)
	mux.HandleFunc("/v1/flashcards/", h.flashcardRouter)
	mux.HandleFunc("GET /v1/folders", h.listFolders)
	mux.HandleFunc("POST /v1/folders", h.createFolder)
	mux.HandleFunc("/v1/folders/", h.folderRouter)
	// Phase 10: Template download (no auth required)
	mux.HandleFunc("GET /v1/templates/", h.templateRouter)
	// Progress routes – handled within studySetRouter via path inspection.
	// Phase 4 internal API: Quiz service fetches flashcards by ownership.
	mux.HandleFunc("/internal/study-sets/", h.internalRouter)
}

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

func (h *Handler) listStudySets(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromHeader(r)
	q := r.URL.Query()
	filter := model.StudySetFilter{
		Search: strings.TrimSpace(q.Get("search")), SortBy: q.Get("sort"),
		Page: intQueryParam(q.Get("page"), 1), PerPage: intQueryParam(q.Get("per_page"), 20),
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
	set, err := h.sets.Create(r.Context(), userIDFromHeader(r), in)
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

	if len(parts) == 2 && parts[1] == "flashcards" {
		if r.Method == http.MethodPost {
			h.createFlashcard(w, r, setID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	if len(parts) == 3 && parts[1] == "flashcards" && parts[2] == "bulk" {
		if r.Method == http.MethodPost {
			h.bulkSaveFlashcards(w, r, setID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// Phase 10: Quiz question routes
	if len(parts) == 2 && parts[1] == "quiz-questions" {
		if r.Method == http.MethodGet {
			h.listQuizQuestions(w, r, setID)
		} else if r.Method == http.MethodPost {
			h.bulkSaveQuizQuestions(w, r, setID)
		} else if r.Method == http.MethodDelete {
			h.deleteQuizQuestions(w, r, setID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// Phase 10: Import routes
	if len(parts) == 3 && parts[1] == "import" && parts[2] == "flashcards" {
		if r.Method == http.MethodPost {
			h.importFlashcards(w, r, setID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	if len(parts) == 3 && parts[1] == "import" && parts[2] == "quiz" {
		if r.Method == http.MethodPost {
			h.importQuiz(w, r, setID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// Progress sub-routes:
	//   POST /v1/study-sets/{id}/progress
	//   GET  /v1/study-sets/{id}/progress
	//   GET  /v1/study-sets/{id}/progress/latest
	if len(parts) == 2 && parts[1] == "progress" {
		switch r.Method {
		case http.MethodPost:
			h.saveProgress(w, r, setID)
		case http.MethodGet:
			h.getProgressSummary(w, r, setID)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	if len(parts) == 3 && parts[1] == "progress" && parts[2] == "latest" {
		if r.Method == http.MethodGet {
			h.getLatestProgress(w, r, setID)
		} else {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	userID := userIDFromHeader(r)
	switch r.Method {
	case http.MethodGet:
		// Ownership is mandatory: the authenticated user ID flows into the service.
		set, err := h.sets.GetWithCards(r.Context(), setID, userID)
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

func (h *Handler) createFlashcard(w http.ResponseWriter, r *http.Request, studySetID int64) {
	var in model.CreateFlashcardInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	card, err := h.cards.Create(r.Context(), studySetID, userIDFromHeader(r), in)
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

func (h *Handler) bulkSaveFlashcards(w http.ResponseWriter, r *http.Request, studySetID int64) {
	var in model.BulkSaveFlashcardsInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := h.cards.BulkSave(r.Context(), studySetID, userIDFromHeader(r), in)
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
	if len(parts) == 2 && parts[1] == "star" && r.Method == http.MethodPost {
		card, err := h.cards.ToggleStar(r.Context(), cardID, userIDFromHeader(r))
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

func (h *Handler) listFolders(w http.ResponseWriter, r *http.Request) {
	folders, err := h.folders.List(r.Context(), userIDFromHeader(r))
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, folders)
}

func (h *Handler) createFolder(w http.ResponseWriter, r *http.Request) {
	var in model.CreateFolderInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		WriteRequestError(w, r, http.StatusUnprocessableEntity, "invalid request", map[string]any{"field": "body"})
		return
	}
	folder, err := h.folders.Create(r.Context(), userIDFromHeader(r), in)
	if err != nil {
		if errors.Is(err, service.ErrValidation) {
			WriteRequestError(w, r, http.StatusUnprocessableEntity, "invalid request", map[string]any{"field": "title"})
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
	if err != nil || folderID <= 0 {
		WriteRequestError(w, r, http.StatusUnprocessableEntity, "invalid request", map[string]any{"field": "folderId"})
		return
	}
	userID := userIDFromHeader(r)
	if len(parts) == 2 && parts[1] == "study-sets" && r.Method == http.MethodPost {
		var body struct {
			StudySetID int64 `json:"studySetId"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil || body.StudySetID <= 0 {
			WriteRequestError(w, r, http.StatusUnprocessableEntity, "invalid request", map[string]any{"field": "studySetId"})
			return
		}
		if err := h.folders.AddStudySet(r.Context(), folderID, body.StudySetID, userID); err != nil {
			WriteServiceError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if len(parts) == 3 && parts[1] == "study-sets" && r.Method == http.MethodDelete {
		setID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || setID <= 0 {
			WriteRequestError(w, r, http.StatusUnprocessableEntity, "invalid request", map[string]any{"field": "studySetId"})
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
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
			WriteRequestError(w, r, http.StatusUnprocessableEntity, "invalid request", map[string]any{"field": "body"})
			return
		}
		folder, err := h.folders.Update(r.Context(), folderID, userID, in)
		if err != nil {
			if errors.Is(err, service.ErrValidation) {
				WriteRequestError(w, r, http.StatusUnprocessableEntity, "invalid request", map[string]any{"field": "title"})
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

// ---------------------------------------------------------------------------
// Progress handlers
// ---------------------------------------------------------------------------

// saveProgress handles POST /v1/study-sets/{id}/progress.
// userID is read from the X-User-ID header set by the gateway (never trusted from body).
func (h *Handler) saveProgress(w http.ResponseWriter, r *http.Request, studySetID int64) {
	var in model.SaveProgressInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	session, err := h.progress.Save(r.Context(), userIDFromHeader(r), studySetID, in)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrConflict):
			WriteError(w, http.StatusConflict, "duplicate idempotency key: this session was already saved")
		case errors.Is(err, service.ErrUnauthorized):
			WriteError(w, http.StatusUnauthorized, "authentication required")
		case errors.Is(err, service.ErrForbidden):
			WriteError(w, http.StatusForbidden, "you do not have permission to perform this action")
		case errors.Is(err, repository.ErrNotFound):
			WriteError(w, http.StatusNotFound, "resource not found")
		default:
			if service.IsProgressValidationError(err) {
				WriteError(w, http.StatusUnprocessableEntity, err.Error())
			} else {
				WriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}
		return
	}
	WriteJSON(w, http.StatusCreated, session)
}

// getProgressSummary handles GET /v1/study-sets/{id}/progress.
func (h *Handler) getProgressSummary(w http.ResponseWriter, r *http.Request, studySetID int64) {
	q := r.URL.Query()
	filter := model.ProgressFilter{
		Page:    intQueryParam(q.Get("page"), 1),
		PerPage: intQueryParam(q.Get("per_page"), 20),
	}
	summary, err := h.progress.GetSummary(r.Context(), userIDFromHeader(r), studySetID, filter)
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, summary)
}

// getLatestProgress handles GET /v1/study-sets/{id}/progress/latest.
func (h *Handler) getLatestProgress(w http.ResponseWriter, r *http.Request, studySetID int64) {
	sessions, err := h.progress.GetLatest(r.Context(), userIDFromHeader(r), studySetID)
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, sessions)
}

// ---------------------------------------------------------------------------
// Phase 10: Quiz Question handlers

func (h *Handler) listQuizQuestions(w http.ResponseWriter, r *http.Request, studySetID int64) {
	questions, err := h.quizQuestions.ListByStudySet(r.Context(), studySetID, userIDFromHeader(r))
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, questions)
}

func (h *Handler) bulkSaveQuizQuestions(w http.ResponseWriter, r *http.Request, studySetID int64) {
	var in model.BulkSaveQuizQuestionsInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.quizQuestions.BulkSave(r.Context(), studySetID, userIDFromHeader(r), in); err != nil {
		if isValidationError(err) {
			WriteError(w, http.StatusBadRequest, err.Error())
		} else {
			WriteServiceError(w, err)
		}
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) deleteQuizQuestions(w http.ResponseWriter, r *http.Request, studySetID int64) {
	if err := h.quizQuestions.DeleteByStudySet(r.Context(), studySetID, userIDFromHeader(r)); err != nil {
		WriteServiceError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---------------------------------------------------------------------------
// Phase 10: Import handlers

func (h *Handler) importFlashcards(w http.ResponseWriter, r *http.Request, studySetID int64) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	result, err := h.importSvc.ImportFlashcards(r.Context(), studySetID, userIDFromHeader(r), file)
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	if len(result.Errors) > 0 {
		WriteJSON(w, http.StatusUnprocessableEntity, result)
		return
	}
	WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) importQuiz(w http.ResponseWriter, r *http.Request, studySetID int64) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	result, err := h.importSvc.ImportQuiz(r.Context(), studySetID, userIDFromHeader(r), file)
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	if len(result.Errors) > 0 {
		WriteJSON(w, http.StatusUnprocessableEntity, result)
		return
	}
	WriteJSON(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Phase 4: Internal API for Quiz service

// internalRouter handles /internal/study-sets/{id}/* routes.
func (h *Handler) internalRouter(w http.ResponseWriter, r *http.Request) {
	parts := PathParts(r.URL.Path, "/internal/study-sets/")
	if len(parts) == 0 {
		WriteError(w, http.StatusNotFound, "study set not found")
		return
	}
	setID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid study set id")
		return
	}
	if len(parts) == 2 && parts[1] == "flashcards" && r.Method == http.MethodGet {
		h.getFlashcardsInternal(w, r, setID)
		return
	}
	WriteError(w, http.StatusNotFound, "endpoint not found")
}

// getFlashcardsInternal returns all flashcards for a study set.
// Used by Quiz service to fetch cards by ownership via X-User-ID header.
func (h *Handler) getFlashcardsInternal(w http.ResponseWriter, r *http.Request, studySetID int64) {
	userID := userIDFromHeader(r)
	set, err := h.sets.GetWithCards(r.Context(), studySetID, userID)
	if err != nil {
		WriteServiceError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, set)
}

// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Phase 10: Template download handler

func (h *Handler) templateRouter(w http.ResponseWriter, r *http.Request) {
	parts := PathParts(r.URL.Path, "/v1/templates/")
	if len(parts) == 0 {
		WriteError(w, http.StatusNotFound, "template not found")
		return
	}
	name := parts[0]
	if name != "flashcard_template.xlsx" && name != "quiz_template.xlsx" {
		WriteError(w, http.StatusNotFound, "template not found")
		return
	}
	data, err := templates.GetTemplate(name)
	if err != nil {
		WriteError(w, http.StatusNotFound, "template not found")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, data)
}

func userIDFromHeader(r *http.Request) int64 {
	raw := r.Header.Get("X-User-ID")
	if raw == "" {
		return 0
	}
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}

func isValidationError(err error) bool {
	return err != nil && !errors.Is(err, service.ErrForbidden) && !errors.Is(err, service.ErrUnauthorized)
}

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
