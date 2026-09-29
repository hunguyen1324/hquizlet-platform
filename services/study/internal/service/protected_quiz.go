package service

import (
	"context"
	"crypto/rand"
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
	paymentURL string
	now        func() time.Time
}

func NewProtectedQuizService(sets repository.StudySets, questions repository.QuizQuestions, sessions repository.QuizSessions, paymentURL string) *ProtectedQuizService {
	return &ProtectedQuizService{sets: sets, questions: questions, sessions: sessions, paymentURL: strings.TrimRight(paymentURL, "/"), now: time.Now}
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
	items, err := sessionItems(qs)
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
	return map[string]any{"total": len(items), "parts": counts, "sessions": ids}, nil
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
	all, err := sessionItems(qs)
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
	session := model.QuizSession{ID: hex.EncodeToString(token[:]), UserID: uid, StudySetID: setID, Mode: in.Mode, Instant: in.Mode == "practice" && in.Instant, Layout: in.Layout, StartedAt: now, ExpiresAt: now.Add(24 * time.Hour), Items: items, Answers: map[int]string{}, Seen: map[int]bool{}}
	if in.DurationSeconds > 0 {
		deadline := now.Add(time.Duration(in.DurationSeconds) * time.Second)
		session.Deadline = &deadline
	}
	view := sessionView(&session, 0, now)
	if err = s.sessions.Create(ctx, session); err != nil {
		return empty, err
	}
	return view, nil
}

// Act serializes navigation, answers and finalization under a row lock.
// The deadline is enforced here, not trusted from the browser.
func (s *ProtectedQuizService) Act(ctx context.Context, setID, uid int64, id, action string, index int, answer string) (model.QuizSessionView, error) {
	var view model.QuizSessionView
	if err := s.access(ctx, setID, uid); err != nil {
		return view, err
	}
	if err := s.Allow(ctx, uid, "play", 120, time.Minute); err != nil {
		return view, err
	}
	err := s.sessions.Update(ctx, id, uid, setID, func(session *model.QuizSession) error {
		now := s.now().UTC()
		if index == -1 {
			index = session.CurrentIndex
		}
		if index < 0 || index >= len(session.Items) {
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
				q := session.Items[index].Question
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
		view = sessionView(session, index, now)
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
	err := s.sessions.Update(ctx, id, uid, setID, func(session *model.QuizSession) error {
		if index < 0 || index >= len(session.Items) || !session.Seen[index] {
			return ErrForbidden
		}
		if u := session.Items[index].Question.AudioURL; u != nil {
			source = *u
		}
		if source == "" {
			return repository.ErrNotFound
		}
		return nil
	})
	return source, err
}
func sessionView(s *model.QuizSession, index int, now time.Time) model.QuizSessionView {
	s.CurrentIndex = index
	view := model.QuizSessionView{ID: s.ID, Mode: s.Mode, Instant: s.Instant, Layout: s.Layout, ServerTime: now, Deadline: s.Deadline, Submitted: s.SubmittedAt != nil, CurrentIndex: index, Manifest: []model.QuizManifestItem{}, Page: []model.QuizPageItem{}, Answers: s.Answers, Results: map[int]bool{}}
	score := 0
	for i, it := range s.Items {
		view.Manifest = append(view.Manifest, model.QuizManifestItem{Number: it.Number, Part: it.Part, Group: it.Group})
		answer, answered := s.Answers[i]
		correct := answered && quizCorrect(it.Question, answer)
		if correct {
			score++
		}
		if s.SubmittedAt != nil || (s.Instant && answered) {
			view.Results[i] = correct
		}
		if (s.Layout == "toeic" && it.Group == s.Items[index].Group) || (s.Layout != "toeic" && i == index) {
			s.Seen[i] = true
			// Only reveal solutions for attempted questions. Blank submissions cannot dump an answer bank.
			revealed := answered && (s.SubmittedAt != nil || s.Instant)
			q := it.Question
			q.SubQuestions = nil
			q.QuestionText = safeQuizHTML(q.QuestionText)
			if q.ParagraphText != nil {
				v := safeQuizHTML(*q.ParagraphText)
				q.ParagraphText = &v
			}
			if q.AnswerExplanation != nil {
				v := safeQuizHTML(*q.AnswerExplanation)
				q.AnswerExplanation = &v
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
			view.Page = append(view.Page, model.QuizPageItem{Index: i, Question: q, HasAudio: it.Question.AudioURL != nil && *it.Question.AudioURL != "", Revealed: revealed})
		}
	}
	if s.SubmittedAt != nil {
		view.Score = &score
		view.TimeUsed = int(s.SubmittedAt.Sub(s.StartedAt).Seconds())
	}
	return view
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
func sessionItems(questions []model.QuizQuestion) ([]model.SessionItem, error) {
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
