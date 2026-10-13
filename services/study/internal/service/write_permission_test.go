package service

import (
	"context"
	"errors"
	"testing"
)

func TestServerWritePermissionIsBoundToResourceAndActor(t *testing.T) {
	svc := newSvc(map[int64]int64{1: 42, 2: 42}, nil, &fakeProgress{})
	sets := NewStudySetService(svc.sets, svc.cards)
	ctx := WithStudyWritePermission(context.Background(), 99, 1)
	if err := sets.checkOwner(ctx, 1, 99); err != nil {
		t.Fatal(err)
	}
	if err := sets.checkOwner(ctx, 2, 99); !errors.Is(err, ErrForbidden) {
		t.Fatal("permission escaped resource", err)
	}
	if err := sets.checkOwner(ctx, 1, 100); !errors.Is(err, ErrForbidden) {
		t.Fatal("permission escaped actor", err)
	}
	if err := sets.checkOwner(context.Background(), 1, 99); !errors.Is(err, ErrForbidden) {
		t.Fatal("missing server permission accepted", err)
	}
}
