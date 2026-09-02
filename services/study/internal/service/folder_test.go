package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
)

// ---------- fake folder repo ----------

type fakeFolderRepo struct {
	folders map[int64]model.Folder
	links   map[int64][]int64 // folderID -> []studySetID
	nextID  int64
}

func newFakeFolderRepo() *fakeFolderRepo {
	return &fakeFolderRepo{
		folders: make(map[int64]model.Folder),
		links:   make(map[int64][]int64),
		nextID:  1,
	}
}

func (r *fakeFolderRepo) List(_ context.Context, userID int64) ([]model.Folder, error) {
	var out []model.Folder
	for _, f := range r.folders {
		if f.UserID == userID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *fakeFolderRepo) Get(_ context.Context, id int64) (model.Folder, error) {
	f, ok := r.folders[id]
	if !ok {
		return model.Folder{}, errors.New("not found")
	}
	return f, nil
}

func (r *fakeFolderRepo) Create(_ context.Context, userID int64, in model.CreateFolderInput) (model.Folder, error) {
	f := model.Folder{ID: r.nextID, UserID: userID, Title: in.Title, Description: in.Description}
	r.folders[r.nextID] = f
	r.nextID++
	return f, nil
}

func (r *fakeFolderRepo) Update(_ context.Context, id int64, in model.UpdateFolderInput) (model.Folder, error) {
	f, ok := r.folders[id]
	if !ok {
		return model.Folder{}, errors.New("not found")
	}
	f.Title = in.Title
	f.Description = in.Description
	r.folders[id] = f
	return f, nil
}

func (r *fakeFolderRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.folders[id]; !ok {
		return errors.New("not found")
	}
	delete(r.folders, id)
	delete(r.links, id)
	return nil
}

func (r *fakeFolderRepo) IsOwner(_ context.Context, id, userID int64) (bool, error) {
	f, ok := r.folders[id]
	if !ok {
		return false, errors.New("not found")
	}
	return f.UserID == userID, nil
}

func (r *fakeFolderRepo) ListStudySets(_ context.Context, folderID int64) ([]model.StudySet, error) {
	return nil, nil
}

func (r *fakeFolderRepo) AddStudySet(_ context.Context, folderID, studySetID int64) error {
	r.links[folderID] = append(r.links[folderID], studySetID)
	return nil
}

func (r *fakeFolderRepo) RemoveStudySet(_ context.Context, folderID, studySetID int64) error {
	ids := r.links[folderID]
	for i, id := range ids {
		if id == studySetID {
			r.links[folderID] = append(ids[:i], ids[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

// ---------- tests ----------

func newFolderSvc() (*service.FolderService, *fakeFolderRepo, *fakeSetRepo) {
	fr := newFakeFolderRepo()
	sr := newFakeSetRepo()
	svc := service.NewFolderService(fr, sr)
	return svc, fr, sr
}

func TestFolderCreate_RequiresTitle(t *testing.T) {
	svc, _, _ := newFolderSvc()
	_, err := svc.Create(context.Background(), 1, model.CreateFolderInput{Title: "  "})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestFolderCreate_RequiresAuth(t *testing.T) {
	svc, _, _ := newFolderSvc()
	_, err := svc.Create(context.Background(), 0, model.CreateFolderInput{Title: "My Folder"})
	if err == nil {
		t.Fatal("expected error when unauthenticated")
	}
}

func TestFolderUpdate_ForbiddenForOtherUser(t *testing.T) {
	svc, fr, _ := newFolderSvc()
	folder, _ := fr.Create(context.Background(), 1, model.CreateFolderInput{Title: "Owner"})

	_, err := svc.Update(context.Background(), folder.ID, 2, model.UpdateFolderInput{Title: "Hijack"})
	if !errors.Is(err, service.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestFolderDelete_OwnerCanDelete(t *testing.T) {
	svc, fr, _ := newFolderSvc()
	folder, _ := fr.Create(context.Background(), 1, model.CreateFolderInput{Title: "My Folder"})

	if err := svc.Delete(context.Background(), folder.ID, 1); err != nil {
		t.Errorf("owner should be able to delete, got %v", err)
	}
}

func TestFolderAddStudySet_ForbiddenIfNotSetOwner(t *testing.T) {
	svc, fr, sr := newFolderSvc()
	folder, _ := fr.Create(context.Background(), 1, model.CreateFolderInput{Title: "Folder"})
	// study set owned by user 2
	set, _ := sr.Create(context.Background(), 2, model.CreateStudySetInput{Title: "Other's set"})

	err := svc.AddStudySet(context.Background(), folder.ID, set.ID, 1)
	if !errors.Is(err, service.ErrForbidden) {
		t.Errorf("expected ErrForbidden when adding another user's set, got %v", err)
	}
}

func TestFolderList_OnlyReturnsOwnFolders(t *testing.T) {
	svc, fr, _ := newFolderSvc()
	fr.Create(context.Background(), 1, model.CreateFolderInput{Title: "User1 Folder"})
	fr.Create(context.Background(), 2, model.CreateFolderInput{Title: "User2 Folder"})

	folders, err := svc.List(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range folders {
		if f.UserID != 1 {
			t.Errorf("expected only user 1 folders, got userID=%d", f.UserID)
		}
	}
}
