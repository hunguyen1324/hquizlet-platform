package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
)

// ---------- fakes ----------

// fakeSetRepo implements repository.StudySets for testing.
type fakeSetRepo struct {
	sets   map[int64]model.StudySet
	nextID int64
}

func newFakeSetRepo() *fakeSetRepo {
	return &fakeSetRepo{sets: make(map[int64]model.StudySet), nextID: 1}
}

// ensure interface compliance
var _ interface {
	List(context.Context, int64) ([]model.StudySet, error)
	ListAll(context.Context) ([]model.StudySet, error)
	ListWithFilter(context.Context, int64, model.StudySetFilter) (model.StudySetListResult, error)
	Get(context.Context, int64) (model.StudySet, error)
	Create(context.Context, int64, model.CreateStudySetInput) (model.StudySet, error)
	Update(context.Context, int64, model.UpdateStudySetInput) (model.StudySet, error)
	Delete(context.Context, int64) error
	IsOwner(context.Context, int64, int64) (bool, error)
} = (*fakeSetRepo)(nil)

func (r *fakeSetRepo) List(_ context.Context, userID int64) ([]model.StudySet, error) {
	var out []model.StudySet
	for _, s := range r.sets {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *fakeSetRepo) ListAll(_ context.Context) ([]model.StudySet, error) {
	var out []model.StudySet
	for _, s := range r.sets {
		out = append(out, s)
	}
	return out, nil
}

func (r *fakeSetRepo) ListWithFilter(_ context.Context, userID int64, f model.StudySetFilter) (model.StudySetListResult, error) {
	var items []model.StudySet
	for _, s := range r.sets {
		if s.UserID != userID {
			continue
		}
		if f.Search != "" && len(s.Title) < len(f.Search) {
			continue
		}
		items = append(items, s)
	}
	return model.StudySetListResult{Items: items, Total: len(items), Page: 1, PerPage: 20, TotalPages: 1}, nil
}

func (r *fakeSetRepo) Get(_ context.Context, id int64) (model.StudySet, error) {
	s, ok := r.sets[id]
	if !ok {
		return model.StudySet{}, errors.New("not found")
	}
	return s, nil
}

func (r *fakeSetRepo) Create(_ context.Context, userID int64, in model.CreateStudySetInput) (model.StudySet, error) {
	s := model.StudySet{ID: r.nextID, UserID: userID, Title: in.Title, Description: in.Description}
	r.sets[r.nextID] = s
	r.nextID++
	return s, nil
}

func (r *fakeSetRepo) Update(_ context.Context, id int64, in model.UpdateStudySetInput) (model.StudySet, error) {
	s, ok := r.sets[id]
	if !ok {
		return model.StudySet{}, errors.New("not found")
	}
	s.Title = in.Title
	s.Description = in.Description
	r.sets[id] = s
	return s, nil
}

func (r *fakeSetRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.sets[id]; !ok {
		return errors.New("not found")
	}
	delete(r.sets, id)
	return nil
}

func (r *fakeSetRepo) IsOwner(_ context.Context, id, userID int64) (bool, error) {
	s, ok := r.sets[id]
	if !ok {
		return false, errors.New("not found")
	}
	return s.UserID == userID, nil
}

type fakeCardRepo struct{}

func (r *fakeCardRepo) ListByStudySet(_ context.Context, _ int64) ([]model.Flashcard, error) {
	return nil, nil
}

// ---------- tests ----------

func newSetSvc() *service.StudySetService {
	setRepo := newFakeSetRepo()
	return service.NewStudySetService(setRepo, nil)
}

func TestCreate_RequiresTitle(t *testing.T) {
	svc := newSetSvc()
	_, err := svc.Create(context.Background(), 1, model.CreateStudySetInput{Title: "  "})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestCreate_TrimsTitle(t *testing.T) {
	svc := newSetSvc()
	set, err := svc.Create(context.Background(), 1, model.CreateStudySetInput{Title: "  Hello  "})
	if err != nil {
		t.Fatal(err)
	}
	if set.Title != "Hello" {
		t.Errorf("expected trimmed title 'Hello', got %q", set.Title)
	}
}

func TestUpdate_ForbiddenForOtherUser(t *testing.T) {
	setRepo := newFakeSetRepo()
	svc := service.NewStudySetService(setRepo, nil)

	// user 1 creates
	set, _ := setRepo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Owner's set"})

	// user 2 tries to update
	_, err := svc.Update(context.Background(), set.ID, 2, model.UpdateStudySetInput{Title: "Hijack"})
	if !errors.Is(err, service.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestDelete_ForbiddenForOtherUser(t *testing.T) {
	setRepo := newFakeSetRepo()
	svc := service.NewStudySetService(setRepo, nil)

	set, _ := setRepo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "My set"})

	err := svc.Delete(context.Background(), set.ID, 99)
	if !errors.Is(err, service.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestDelete_OwnerCanDelete(t *testing.T) {
	setRepo := newFakeSetRepo()
	svc := service.NewStudySetService(setRepo, nil)

	set, _ := setRepo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "My set"})
	if err := svc.Delete(context.Background(), set.ID, 1); err != nil {
		t.Errorf("owner should be able to delete, got %v", err)
	}
}

func TestUpdate_RequiresTitle(t *testing.T) {
	setRepo := newFakeSetRepo()
	svc := service.NewStudySetService(setRepo, nil)
	set, _ := setRepo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Original"})

	_, err := svc.Update(context.Background(), set.ID, 1, model.UpdateStudySetInput{Title: ""})
	if err == nil {
		t.Fatal("expected validation error for empty title")
	}
}

func TestListWithFilter_OnlyReturnsOwnSets(t *testing.T) {
	setRepo := newFakeSetRepo()
	svc := service.NewStudySetService(setRepo, nil)

	setRepo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "User1 Set"})
	setRepo.Create(context.Background(), 2, model.CreateStudySetInput{Title: "User2 Set"})

	result, err := svc.ListWithFilter(context.Background(), 1, model.StudySetFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range result.Items {
		if s.UserID != 1 {
			t.Errorf("expected only user 1 sets, got userID=%d", s.UserID)
		}
	}
}
