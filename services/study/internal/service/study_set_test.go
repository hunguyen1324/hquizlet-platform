package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
)

type fakeSetRepo struct { sets map[int64]model.StudySet; nextID int64 }
func newFakeSetRepo() *fakeSetRepo { return &fakeSetRepo{sets: make(map[int64]model.StudySet), nextID: 1} }
var _ interface {
	List(context.Context, int64) ([]model.StudySet, error)
	ListAll(context.Context) ([]model.StudySet, error)
	ListWithFilter(context.Context, int64, model.StudySetFilter) (model.StudySetListResult, error)
	Get(context.Context, int64) (model.StudySet, error)
	GetOwned(context.Context, int64, int64) (model.StudySet, error)
	Create(context.Context, int64, model.CreateStudySetInput) (model.StudySet, error)
	Update(context.Context, int64, model.UpdateStudySetInput) (model.StudySet, error)
	Delete(context.Context, int64) error
	IsOwner(context.Context, int64, int64) (bool, error)
} = (*fakeSetRepo)(nil)
func (r *fakeSetRepo) List(_ context.Context, userID int64) ([]model.StudySet, error) { var out []model.StudySet; for _, s := range r.sets { if s.UserID == userID { out = append(out, s) } }; return out, nil }
func (r *fakeSetRepo) ListAll(_ context.Context) ([]model.StudySet, error) { var out []model.StudySet; for _, s := range r.sets { out = append(out, s) }; return out, nil }
func (r *fakeSetRepo) ListWithFilter(_ context.Context, userID int64, f model.StudySetFilter) (model.StudySetListResult, error) { var items []model.StudySet; for _, s := range r.sets { if s.UserID != userID { continue }; if f.Search != "" && len(s.Title) < len(f.Search) { continue }; items = append(items, s) }; return model.StudySetListResult{Items: items, Total: len(items), Page: 1, PerPage: 20, TotalPages: 1}, nil }
func (r *fakeSetRepo) Get(_ context.Context, id int64) (model.StudySet, error) { s, ok := r.sets[id]; if !ok { return model.StudySet{}, errors.New("not found") }; return s, nil }
func (r *fakeSetRepo) GetOwned(_ context.Context, id, userID int64) (model.StudySet, error) { s, ok := r.sets[id]; if !ok || s.UserID != userID { return model.StudySet{}, errors.New("not found") }; return s, nil }
func (r *fakeSetRepo) Create(_ context.Context, userID int64, in model.CreateStudySetInput) (model.StudySet, error) { s := model.StudySet{ID: r.nextID, UserID: userID, Title: in.Title, Description: in.Description}; r.sets[r.nextID] = s; r.nextID++; return s, nil }
func (r *fakeSetRepo) Update(_ context.Context, id int64, in model.UpdateStudySetInput) (model.StudySet, error) { s, ok := r.sets[id]; if !ok { return model.StudySet{}, errors.New("not found") }; s.Title = in.Title; s.Description = in.Description; r.sets[id] = s; return s, nil }
func (r *fakeSetRepo) Delete(_ context.Context, id int64) error { if _, ok := r.sets[id]; !ok { return errors.New("not found") }; delete(r.sets, id); return nil }
func (r *fakeSetRepo) IsOwner(_ context.Context, id, userID int64) (bool, error) { s, ok := r.sets[id]; if !ok { return false, errors.New("not found") }; return s.UserID == userID, nil }

type fakeCardRepo struct{}
var _ interface {
	ListByStudySet(context.Context, int64) ([]model.Flashcard, error)
	Get(context.Context, int64) (model.Flashcard, error)
	Create(context.Context, int64, model.CreateFlashcardInput) (model.Flashcard, error)
	Update(context.Context, int64, model.UpdateFlashcardInput) (model.Flashcard, error)
	ToggleStar(context.Context, int64) (model.Flashcard, error)
	Delete(context.Context, int64) error
	BulkSave(context.Context, int64, []model.BulkFlashcardItem) (model.BulkSaveResult, error)
} = (*fakeCardRepo)(nil)
func (r *fakeCardRepo) ListByStudySet(_ context.Context, _ int64) ([]model.Flashcard, error) { return []model.Flashcard{}, nil }
func (r *fakeCardRepo) Get(_ context.Context, _ int64) (model.Flashcard, error) { return model.Flashcard{}, errors.New("not found") }
func (r *fakeCardRepo) Create(_ context.Context, studySetID int64, in model.CreateFlashcardInput) (model.Flashcard, error) { return model.Flashcard{StudySetID: studySetID, Term: in.Term, Definition: in.Definition}, nil }
func (r *fakeCardRepo) Update(_ context.Context, _ int64, _ model.UpdateFlashcardInput) (model.Flashcard, error) { return model.Flashcard{}, errors.New("not found") }
func (r *fakeCardRepo) ToggleStar(_ context.Context, _ int64) (model.Flashcard, error) { return model.Flashcard{}, errors.New("not found") }
func (r *fakeCardRepo) Delete(_ context.Context, _ int64) error { return errors.New("not found") }
func (r *fakeCardRepo) BulkSave(_ context.Context, _ int64, _ []model.BulkFlashcardItem) (model.BulkSaveResult, error) { return model.BulkSaveResult{}, nil }

func newSetSvc() *service.StudySetService { return service.NewStudySetService(newFakeSetRepo(), &fakeCardRepo{}) }
func TestCreate_RequiresTitle(t *testing.T) { _, err := newSetSvc().Create(context.Background(), 1, model.CreateStudySetInput{Title: "  "}); if err == nil { t.Fatal("expected error") } }
func TestCreate_TrimsTitle(t *testing.T) { set, err := newSetSvc().Create(context.Background(), 1, model.CreateStudySetInput{Title: "  Hello  "}); if err != nil { t.Fatal(err) }; if set.Title != "Hello" { t.Errorf("got %q", set.Title) } }
func TestCreate_RejectsZeroUserID(t *testing.T) { _, err := newSetSvc().Create(context.Background(), 0, model.CreateStudySetInput{Title: "fail"}); if !errors.Is(err, service.ErrUnauthorized) { t.Fatalf("expected ErrUnauthorized, got %v", err) } }
func TestList_RejectsZeroUserID(t *testing.T) { _, err := newSetSvc().List(context.Background(), 0); if !errors.Is(err, service.ErrUnauthorized) { t.Fatalf("expected ErrUnauthorized, got %v", err) } }
func TestListWithFilter_RejectsZeroUserID(t *testing.T) { _, err := newSetSvc().ListWithFilter(context.Background(), 0, model.StudySetFilter{}); if !errors.Is(err, service.ErrUnauthorized) { t.Fatalf("expected ErrUnauthorized, got %v", err) } }
func TestGetWithCards_RejectsZeroUserID(t *testing.T) { repo := newFakeSetRepo(); set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Owner"}); _, err := service.NewStudySetService(repo, &fakeCardRepo{}).GetWithCards(context.Background(), set.ID, 0); if !errors.Is(err, service.ErrUnauthorized) { t.Fatalf("expected ErrUnauthorized, got %v", err) } }
func TestGetWithCards_IsOwnerScoped(t *testing.T) { repo := newFakeSetRepo(); set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Owner"}); _, err := service.NewStudySetService(repo, &fakeCardRepo{}).GetWithCards(context.Background(), set.ID, 2); if err == nil { t.Fatal("expected non-owner access to fail") } }
func TestGetWithCards_OwnerCanRead(t *testing.T) { repo := newFakeSetRepo(); set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Owner"}); got, err := service.NewStudySetService(repo, &fakeCardRepo{}).GetWithCards(context.Background(), set.ID, 1); if err != nil { t.Fatalf("owner should read: %v", err) }; if got.UserID != 1 { t.Fatalf("got userID=%d", got.UserID) } }
func TestUpdate_ForbiddenForOtherUser(t *testing.T) { repo := newFakeSetRepo(); svc := service.NewStudySetService(repo, nil); set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Owner"}); _, err := svc.Update(context.Background(), set.ID, 2, model.UpdateStudySetInput{Title: "Hijack"}); if !errors.Is(err, service.ErrForbidden) { t.Errorf("got %v", err) } }
func TestUpdate_RejectsZeroUserID(t *testing.T) { repo := newFakeSetRepo(); svc := service.NewStudySetService(repo, nil); set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Original"}); _, err := svc.Update(context.Background(), set.ID, 0, model.UpdateStudySetInput{Title: "Hijack"}); if !errors.Is(err, service.ErrUnauthorized) { t.Errorf("got %v", err) } }
func TestDelete_ForbiddenForOtherUser(t *testing.T) { repo := newFakeSetRepo(); svc := service.NewStudySetService(repo, nil); set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "My set"}); if err := svc.Delete(context.Background(), set.ID, 99); !errors.Is(err, service.ErrForbidden) { t.Errorf("got %v", err) } }
func TestDelete_OwnerCanDelete(t *testing.T) { repo := newFakeSetRepo(); svc := service.NewStudySetService(repo, nil); set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "My set"}); if err := svc.Delete(context.Background(), set.ID, 1); err != nil { t.Errorf("got %v", err) } }
func TestDelete_RejectsZeroUserID(t *testing.T) { repo := newFakeSetRepo(); svc := service.NewStudySetService(repo, nil); set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "My set"}); if err := svc.Delete(context.Background(), set.ID, 0); !errors.Is(err, service.ErrUnauthorized) { t.Errorf("got %v", err) } }
func TestUpdate_RequiresTitle(t *testing.T) { repo := newFakeSetRepo(); svc := service.NewStudySetService(repo, nil); set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Original"}); _, err := svc.Update(context.Background(), set.ID, 1, model.UpdateStudySetInput{Title: ""}); if err == nil { t.Fatal("expected validation error") } }
func TestListWithFilter_OnlyReturnsOwnSets(t *testing.T) { repo := newFakeSetRepo(); svc := service.NewStudySetService(repo, nil); repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "User1 Set"}); repo.Create(context.Background(), 2, model.CreateStudySetInput{Title: "User2 Set"}); result, err := svc.ListWithFilter(context.Background(), 1, model.StudySetFilter{}); if err != nil { t.Fatal(err) }; for _, s := range result.Items { if s.UserID != 1 { t.Errorf("got userID=%d", s.UserID) } } }
