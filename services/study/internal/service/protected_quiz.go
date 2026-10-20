package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

type ProtectedQuizService struct {
	sets       repository.StudySets
	questions  repository.QuizQuestions
	sessions   repository.QuizSessions
	snapshots  repository.QuizSnapshots
	limiter    QuizLimiter
	paymentURL string
	now        func() time.Time
}

// NewProtectedQuizService wires the protected quiz flow. snapshots may be nil,
// in which case sessions keep their questions in state (the pre-Phase-2
// layout); only legacy-session tests and the migration window use that path.
func NewProtectedQuizService(sets repository.StudySets, questions repository.QuizQuestions, sessions repository.QuizSessions, snapshots repository.QuizSnapshots, paymentURL string) *ProtectedQuizService {
	return &ProtectedQuizService{sets: sets, questions: questions, sessions: sessions, snapshots: snapshots, limiter: NewLocalLimiter(), paymentURL: strings.TrimRight(paymentURL, "/"), now: time.Now}
}

// WithLimiter replaces the limiter used for hot buckets. Pass a shared limiter
// (Redis) to keep the cap exact across replicas; nil falls back to the database
// counter, which is correct but costs one write per request.
func (s *ProtectedQuizService) WithLimiter(l QuizLimiter) *ProtectedQuizService {
	s.limiter = l
	return s
}
func (s *ProtectedQuizService) access(ctx context.Context, setID, uid int64) error {
	if err := requireUserID(uid); err != nil {
		return err
	}
	set, err := s.sets.Get(ctx, setID)
	if err != nil {
		return err
	}
	if set.ContentType != "quiz" {
		return ErrValidation
	}
	if set.UserID == uid {
		return nil
	}
	if set.Visibility != "public" {
		return ErrForbidden
	}
	// Fail closed if paid-content access cannot be checked.
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/internal/payment/entitlements/check?study_set_id=%d", s.paymentURL, setID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-User-ID", fmt.Sprint(uid))
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("access service unavailable")
	}
	var info struct {
		HasAccess bool `json:"hasAccess"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&info); err != nil {
		return err
	}
	if !info.HasAccess {
		return ErrForbidden
	}
	return nil
}
func (s *ProtectedQuizService) Allow(ctx context.Context, uid int64, bucket string, max int, window time.Duration) error {
	if err := requireUserID(uid); err != nil {
		return err
	}
	// Answer/page/audio fire on every tap; charging them in the database was a
	// write per request. Rare buckets stay on the shared counter.
	if s.limiter != nil && !sharedLimitBuckets[bucket] {
		return s.limiter.Allow(ctx, uid, bucket, max, window)
	}
	return s.sessions.Allow(ctx, uid, bucket, max, window)
}
func (s *ProtectedQuizService) Summary(ctx context.Context, setID, uid int64) (map[string]any, error) {
	if err := s.access(ctx, setID, uid); err != nil {
		return nil, err
	}
	if err := s.Allow(ctx, uid, "summary", 30, time.Minute); err != nil {
		return nil, err
	}
	qs, err := s.questions.ListByStudySet(ctx, setID)
	if err != nil {
		return nil, err
	}
	items, err := sessionItems(qs, false)
	if err != nil {
		return nil, err
	}
	counts := map[int]int{}
	for _, it := range items {
		counts[it.Part]++
	}
	ids, err := s.sessions.Recent(ctx, uid, setID)
	if err != nil {
		return nil, err
	}
	activeCount, err := s.sessions.CountActive(ctx, uid, setID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"total":       len(items),
		"parts":       counts,
		"sessions":    ids,
		"activeCount": activeCount,
		"limit":       repository.MaxSessionsPerQuiz,
		"studySetId":  setID,
	}, nil
}

// ListSessions returns paginated session metadata for the given user.
// When studySetID > 0, only sessions of that quiz are returned.
// No access check against the quiz itself — the caller is the session owner.
func (s *ProtectedQuizService) ListSessions(ctx context.Context, uid, studySetID int64, page, perPage int) ([]repository.QuizSessionMeta, int, error) {
	if err := requireUserID(uid); err != nil {
		return nil, 0, err
	}
	return s.sessions.List(ctx, uid, studySetID, page, perPage)
}

// DeleteSession soft-deletes a session owned by uid.
// Returns ErrForbidden if the session belongs to another user,
// ErrNotFound if the session does not exist.
func (s *ProtectedQuizService) DeleteSession(ctx context.Context, id string, uid int64) error {
	if err := requireUserID(uid); err != nil {
		return err
	}
	if err := s.sessions.Delete(ctx, id, uid); err != nil {
		return err
	}
	return nil
}
func (s *ProtectedQuizService) Start(ctx context.Context, setID, uid int64, in model.StartQuizInput) (model.QuizSessionView, error) {
	var empty model.QuizSessionView
	if err := s.access(ctx, setID, uid); err != nil {
		return empty, err
	}
	if in.Mode != "practice" && in.Mode != "exam" {
		return empty, ErrValidation
	}
	if in.Layout != "default" && in.Layout != "toeic" {
		return empty, ErrValidation
	}
	if in.Part < 0 || in.Part > 7 || in.DurationSeconds < 0 || in.DurationSeconds > 36000 || (in.Mode == "exam" && in.DurationSeconds < 60) {
		return empty, ErrValidation
	}
	if err := s.Allow(ctx, uid, "start", 20, time.Hour); err != nil {
		return empty, err
	}
	qs, err := s.questions.ListByStudySet(ctx, setID)
	if err != nil {
		return empty, err
	}
	// Sanitize once, into the snapshot, instead of on every request.
	all, err := sessionItems(qs, true)
	if err != nil {
		return empty, err
	}
	items := []model.SessionItem{}
	for _, it := range all {
		if in.Part == 0 || it.Part == in.Part {
			items = append(items, it)
		}
	}
	if len(items) == 0 || len(items) > 5000 {
		return empty, ErrValidation
	}
	var token [24]byte
	if _, err = rand.Read(token[:]); err != nil {
		return empty, err
	}
	now := s.now().UTC()
	session := model.QuizSession{ID: hex.EncodeToString(token[:]), UserID: uid, StudySetID: setID, Mode: in.Mode, Instant: in.Mode == "practice" && in.Instant, Layout: in.Layout, StartedAt: now, ExpiresAt: now.Add(24 * time.Hour), Answers: map[int]string{}, Seen: map[int]bool{}, Total: len(items), Sanitized: true}
	if in.DurationSeconds > 0 {
		deadline := now.Add(time.Duration(in.DurationSeconds) * time.Second)
		session.Deadline = &deadline
	}
	var content quizContent
	if s.snapshots == nil {
		session.Items = items
		content = legacyContent(&session)
	} else {
		snap, err := newQuizSnapshot(setID, in.Part, items, qs, now)
		if err != nil {
			return empty, err
		}
		if err = s.snapshots.Put(ctx, snap); err != nil {
			return empty, err
		}
		session.SnapshotID = snap.ID
		content = contentFromSnapshot(snap)
	}
	view := s.buildView(&session, content, 0, now, true)
	if err = s.sessions.Create(ctx, session); err != nil {
		return empty, err
	}
	return view, nil
}

// ActOptions controls how much of the session a mutation response carries.
// Only a resume needs the manifest: the player keeps it from start.
type ActOptions struct{ IncludeManifest bool }

// Act serializes navigation, answers and finalization under a row lock.
// The deadline is enforced here, not trusted from the browser.
func (s *ProtectedQuizService) Act(ctx context.Context, setID, uid int64, id, action string, index int, answer string) (model.QuizSessionView, error) {
	return s.ActWith(ctx, setID, uid, id, action, index, answer, ActOptions{})
}

func (s *ProtectedQuizService) ActWith(ctx context.Context, setID, uid int64, id, action string, index int, answer string, opts ActOptions) (model.QuizSessionView, error) {
	var view model.QuizSessionView
	if err := s.access(ctx, setID, uid); err != nil {
		return view, err
	}
	if err := s.Allow(ctx, uid, "play", 120, time.Minute); err != nil {
		return view, err
	}
	err := s.sessions.Update(ctx, id, uid, setID, func(session *model.QuizSession) error {
		content, err := s.content(ctx, session)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		if index == -1 {
			index = session.CurrentIndex
		}
		if index < 0 || index >= content.total {
			return ErrValidation
		}
		if session.SubmittedAt == nil && session.Deadline != nil && !now.Before(*session.Deadline) {
			t := *session.Deadline
			session.SubmittedAt = &t
		}
		switch action {
		case "page":
		case "answer":
			if session.SubmittedAt == nil {
				if !session.Seen[index] || len(answer) > 8000 {
					return ErrValidation
				}
				if _, exists := session.Answers[index]; session.Instant && exists {
					break
				}
				q := content.items[index].Question
				if !validQuizAnswer(q, answer) {
					return ErrValidation
				}
				session.Answers[index] = answer
			}
		case "submit":
			if session.SubmittedAt == nil {
				session.SubmittedAt = &now
			}
		default:
			return ErrValidation
		}
		view = s.buildView(session, content, index, now, opts.IncludeManifest)
		return nil
	})
	return view, err
}

// Audio must belong to a delivered question in this exact user's unexpired session.
func (s *ProtectedQuizService) Audio(ctx context.Context, setID, uid int64, id string, index int) (string, error) {
	if err := s.access(ctx, setID, uid); err != nil {
		return "", err
	}
	if err := s.Allow(ctx, uid, "audio", 300, time.Minute); err != nil {
		return "", err
	}
	var source string
	// Audio is a read: the answer path already marked the page as delivered.
	err := s.sessions.View(ctx, id, uid, setID, func(session *model.QuizSession) error {
		content, err := s.content(ctx, session)
		if err != nil {
			return err
		}
		if index < 0 || index >= content.total || !session.Seen[index] {
			return ErrForbidden
		}
		if u := content.items[index].Question.AudioURL; u != nil {
			source = *u
		}
		if source == "" {
			return repository.ErrNotFound
		}
		return nil
	})
	return source, err
}

// quizContent is the immutable question set behind a session: a cached snapshot
// for new sessions, or the items a legacy session still carries in its state.
type quizContent struct {
	items    []model.SessionItem
	pages    [][]int
	manifest []model.QuizManifestItem
	sanitize bool
	total    int
}

// page returns the item indexes served for the page anchored at index.
func (c quizContent) page(layout string, index int) []int {
	if index < 0 || index >= c.total {
		return nil
	}
	if layout == "toeic" {
		return c.pages[index]
	}
	return []int{index}
}

func contentFromSnapshot(snap model.QuizSnapshot) quizContent {
	return quizContent{items: snap.Items, pages: snap.Pages, manifest: snap.Manifest, total: len(snap.Items)}
}

func legacyContent(session *model.QuizSession) quizContent {
	pages, manifest := snapshotIndex(session.Items)
	return quizContent{items: session.Items, pages: pages, manifest: manifest, sanitize: !session.Sanitized, total: len(session.Items)}
}

// content resolves the questions for a session. Snapshots normally come from
// the in-process cache, so this costs no database round trip on the hot path.
func (s *ProtectedQuizService) content(ctx context.Context, session *model.QuizSession) (quizContent, error) {
	if session.SnapshotID == "" {
		return legacyContent(session), nil
	}
	if s.snapshots == nil {
		return quizContent{}, repository.ErrNotFound
	}
	snap, err := s.snapshots.Get(ctx, session.SnapshotID)
	if err != nil {
		return quizContent{}, err
	}
	return contentFromSnapshot(snap), nil
}

// buildView assembles one response. Page content is always page-scoped; the
// manifest, answers and results are complete only for start, a submitted
// session and an explicit resume. Everything else stays small so a tap on a
// 200-question quiz costs O(page) instead of O(quiz).
func (s *ProtectedQuizService) buildView(session *model.QuizSession, content quizContent, index int, now time.Time, includeManifest bool) model.QuizSessionView {
	session.CurrentIndex = index
	submitted := session.SubmittedAt != nil
	view := model.QuizSessionView{
		ID: session.ID, Mode: session.Mode, Instant: session.Instant, Layout: session.Layout,
		ServerTime: now, Deadline: session.Deadline, Submitted: submitted,
		CurrentIndex: index, Total: content.total, Answered: len(session.Answers),
		Page: []model.QuizPageItem{}, Answers: map[int]string{}, Results: map[int]bool{},
	}
	if includeManifest {
		view.Manifest = content.manifest
	}
	if submitted {
		// The result screen needs every question, and it runs once per session.
		for i := 0; i < content.total; i++ {
			answer, answered := session.Answers[i]
			if !answered {
				continue
			}
			view.Answers[i] = answer
			view.Results[i] = quizCorrect(content.items[i].Question, answer)
		}
	}
	served := content.page(session.Layout, index)
	for _, i := range served {
		// Served questions are the only ones a learner may answer or hear.
		session.Seen[i] = true
		answer, answered := session.Answers[i]
		if !submitted && answered {
			view.Answers[i] = answer
			if session.Instant {
				view.Results[i] = quizCorrect(content.items[i].Question, answer)
			}
		}
		revealed := answered && (submitted || session.Instant)
		view.Page = append(view.Page, quizPageItem(content.items[i], i, answered, revealed, content.sanitize))
	}
	if submitted {
		score := 0
		for i := range content.items {
			if view.Results[i] {
				score++
			}
		}
		session.CompletedScore = &score
		view.Score = &score
		view.TimeUsed = int(session.SubmittedAt.Sub(session.StartedAt).Seconds())
	}
	return view
}

// newQuizSnapshot builds the immutable snapshot for one quiz version. The id is
// a content hash, so two learners (or two replicas) starting the same version
// share one row and one cache entry.
func newQuizSnapshot(setID int64, part int, items []model.SessionItem, questions []model.QuizQuestion, now time.Time) (model.QuizSnapshot, error) {
	raw, err := json.Marshal(struct {
		SetID     int64                `json:"setId"`
		Part      int                  `json:"part"`
		Questions []model.QuizQuestion `json:"questions"`
	}{setID, part, questions})
	if err != nil {
		return model.QuizSnapshot{}, err
	}
	sum := sha256.Sum256(raw)
	pages, manifest := snapshotIndex(items)
	return model.QuizSnapshot{
		ID: hex.EncodeToString(sum[:]), StudySetID: setID, Part: part, CreatedAt: now,
		Items: items, Manifest: manifest, Pages: pages,
	}, nil
}

// snapshotIndex precomputes what a page request needs: the manifest and, for
// grouped layouts, which items share a passage.
func snapshotIndex(items []model.SessionItem) ([][]int, []model.QuizManifestItem) {
	pages := make([][]int, len(items))
	manifest := make([]model.QuizManifestItem, len(items))
	for i, it := range items {
		manifest[i] = model.QuizManifestItem{Number: it.Number, Part: it.Part, Group: it.Group}
	}
	for start := 0; start < len(items); {
		end := start + 1
		for end < len(items) && items[end].Group == items[start].Group {
			end++
		}
		group := make([]int, end-start)
		for i := start; i < end; i++ {
			group[i-start] = i
			pages[i] = group
		}
		start = end
	}
	return pages, manifest
}

// Peek serves only the window the player prefetches. Without this bound a caller
// could walk every page and rebuild the whole quiz, which would defeat the
// page-by-page delivery the protected flow depends on.
const (
	peekLookBehind = 1
	peekLookAhead  = 3
)

// Peek returns one page without touching the stored session. It is read-only on
// purpose: prefetching must not hold the row lock, and it must not mark the page
// as seen, so it grants neither answer rights nor audio access.
func (s *ProtectedQuizService) Peek(ctx context.Context, setID, uid int64, id string, index int) (model.QuizPeekView, error) {
	var view model.QuizPeekView
	if err := s.access(ctx, setID, uid); err != nil {
		return view, err
	}
	if err := s.Allow(ctx, uid, "peek", 240, time.Minute); err != nil {
		return view, err
	}
	err := s.sessions.View(ctx, id, uid, setID, func(session *model.QuizSession) error {
		content, err := s.content(ctx, session)
		if err != nil {
			return err
		}
		if index < 0 {
			index = session.CurrentIndex
		}
		if index < 0 || index < session.CurrentIndex-peekLookBehind || index > session.CurrentIndex+peekLookAhead {
			return ErrForbidden
		}
		if index >= content.total {
			return ErrValidation
		}
		now := s.now().UTC()
		view = model.QuizPeekView{ID: session.ID, ServerTime: now, CurrentIndex: session.CurrentIndex, Page: []model.QuizPageItem{}}
		for _, i := range content.page(session.Layout, index) {
			_, answered := session.Answers[i]
			revealed := answered && (session.SubmittedAt != nil || session.Instant)
			view.Page = append(view.Page, quizPageItem(content.items[i], i, answered, revealed, content.sanitize))
		}
		return nil
	})
	return view, err
}

// quizPageItem strips every solution field the learner has not earned yet.
// The returned question is a copy; neither the session nor the snapshot is
// mutated here. Only reveal solutions for attempted questions: a blank
// submission cannot dump an answer bank.
func quizPageItem(it model.SessionItem, i int, answered, revealed, sanitize bool) model.QuizPageItem {
	q := it.Question
	q.SubQuestions = nil
	if sanitize {
		q.QuestionText = safeQuizHTML(q.QuestionText)
		if q.ParagraphText != nil {
			v := safeQuizHTML(*q.ParagraphText)
			q.ParagraphText = &v
		}
		if q.AnswerExplanation != nil {
			v := safeQuizHTML(*q.AnswerExplanation)
			q.AnswerExplanation = &v
		}
	}
	q.AudioURL = nil
	q.Options = append([]model.QuizQuestionOption{}, q.Options...)
	if !revealed {
		q.CorrectAnswer = nil
		q.AnswerExplanation = nil
		for n := range q.Options {
			q.Options[n].IsCorrect = false
		}
	}
	return model.QuizPageItem{Index: i, Question: q, HasAudio: it.Question.AudioURL != nil && *it.Question.AudioURL != "", Revealed: revealed}
}
func validQuizAnswer(q model.QuizQuestion, answer string) bool {
	if strings.TrimSpace(answer) == "" {
		return false
	}
	if q.QuestionType == "multiple_choice" && len(q.Options) > 0 {
		for _, o := range q.Options {
			if answer == o.Text {
				return true
			}
		}
		return false
	}
	if q.QuestionType == "true_false" {
		return answer == "true" || answer == "false"
	}
	return true
}
func quizCorrect(q model.QuizQuestion, answer string) bool {
	normalize := func(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
	if normalize(answer) == "" {
		return false
	}
	correct := ""
	if q.CorrectAnswer != nil {
		correct = *q.CorrectAnswer
	}
	if q.QuestionType == "multiple_choice" {
		for _, o := range q.Options {
			if o.IsCorrect {
				return normalize(answer) == normalize(o.Text)
			}
		}
		letter := strings.ToUpper(strings.TrimSpace(correct))
		if len(letter) == 1 && letter[0] >= 'A' && int(letter[0]-'A') < len(q.Options) {
			correct = q.Options[int(letter[0]-'A')].Text
		}
	}
	return normalize(answer) == normalize(correct)
}

var partTag = regexp.MustCompile(`(?i)^part[\s_-]*([1-7])$`)

func quizPart(tags []string) int {
	for _, tag := range tags {
		m := partTag.FindStringSubmatch(strings.TrimSpace(tag))
		if len(m) > 1 {
			return int(m[1][0] - '0')
		}
	}
	return 0
}
func inferredPart(number int) int {
	for p, end := range []int{6, 31, 70, 100, 130, 146, 200} {
		if number <= end {
			return p + 1
		}
	}
	return 7
}
func sessionItems(questions []model.QuizQuestion, sanitize bool) ([]model.SessionItem, error) {
	items := []model.SessionItem{}
	for group, q := range questions {
		children := []model.QuizQuestion{q}
		if q.QuestionType == "paragraph" {
			raw := q.SubQuestions
			var text string
			if json.Unmarshal(raw, &text) == nil {
				raw = []byte(text)
			}
			if err := json.Unmarshal(raw, &children); err != nil {
				return nil, ErrValidation
			}
			if len(children) > 100 {
				return nil, ErrValidation
			}
		}
		for _, child := range children {
			number := len(items) + 1
			part := quizPart(child.Tags)
			if part == 0 {
				part = quizPart(q.Tags)
			}
			if part == 0 {
				part = inferredPart(number)
			}
			if child.QuestionType == "" {
				child.QuestionType = "multiple_choice"
			}
			if child.ImageURL == nil || *child.ImageURL == "" {
				child.ImageURL = q.ImageURL
			}
			if child.AudioURL == nil || *child.AudioURL == "" {
				child.AudioURL = q.AudioURL
			}
			if q.QuestionType == "paragraph" {
				child.ParagraphText = q.ParagraphText
				if child.ParagraphText == nil {
					child.ParagraphText = &q.QuestionText
				}
			}
			child.SubQuestions = nil
			if sanitize {
				child.QuestionText = safeQuizHTML(child.QuestionText)
				if child.ParagraphText != nil {
					v := safeQuizHTML(*child.ParagraphText)
					child.ParagraphText = &v
				}
				if child.AnswerExplanation != nil {
					v := safeQuizHTML(*child.AnswerExplanation)
					child.AnswerExplanation = &v
				}
			}
			child.Options = append([]model.QuizQuestionOption{}, child.Options...)
			sort.SliceStable(child.Options, func(i, j int) bool { return child.Options[i].Position < child.Options[j].Position })
			if child.QuestionType == "sorting" { // Avoid exposing the expected order through option positions.
				for i := len(child.Options) - 1; i > 0; i-- {
					var b [1]byte
					if _, err := rand.Read(b[:]); err != nil {
						return nil, err
					}
					j := int(b[0]) % (i + 1)
					child.Options[i], child.Options[j] = child.Options[j], child.Options[i]
				}
			}
			for i := range child.Options {
				child.Options[i].Position = i
				child.Options[i].ID = 0
				child.Options[i].QuestionID = 0
			}
			items = append(items, model.SessionItem{Number: number, Part: part, Group: group, Question: child})
		}
	}
	return items, nil
}
