package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

type flavorRepoStub struct {
	listCalls int
	listFn    func(ctx context.Context, onlyActive bool) ([]dto.Flavor, error)
	findByID  func(ctx context.Context, id uuid.UUID) (dto.Flavor, error)
	createFn  func(ctx context.Context, input repository.CreateFlavorInput) (dto.Flavor, error)
	updateFn  func(ctx context.Context, id uuid.UUID, input repository.UpdateFlavorInput) (dto.Flavor, error)
	deleteFn  func(ctx context.Context, id uuid.UUID) error
}

func (s *flavorRepoStub) FindAll(ctx context.Context, onlyActive bool) ([]dto.Flavor, error) {
	s.listCalls++
	if s.listFn != nil {
		return s.listFn(ctx, onlyActive)
	}
	return nil, nil
}

func (s *flavorRepoStub) FindByID(ctx context.Context, id uuid.UUID) (dto.Flavor, error) {
	if s.findByID != nil {
		return s.findByID(ctx, id)
	}
	return dto.Flavor{}, nil
}

func (s *flavorRepoStub) Create(ctx context.Context, input repository.CreateFlavorInput) (dto.Flavor, error) {
	if s.createFn != nil {
		return s.createFn(ctx, input)
	}
	return dto.Flavor{}, nil
}

func (s *flavorRepoStub) Update(ctx context.Context, id uuid.UUID, input repository.UpdateFlavorInput) (dto.Flavor, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, input)
	}
	return dto.Flavor{}, nil
}

func (s *flavorRepoStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func TestFlavorServiceList_UsesCacheOnSecondCall(t *testing.T) {
	repo := &flavorRepoStub{listFn: func(_ context.Context, _ bool) ([]dto.Flavor, error) {
		return []dto.Flavor{{Name: "Vanilla"}}, nil
	}}
	svc := NewFlavorService(repo)

	_, err := svc.List(context.Background(), true)
	if err != nil {
		t.Fatalf("first List() error = %v", err)
	}
	_, err = svc.List(context.Background(), true)
	if err != nil {
		t.Fatalf("second List() error = %v", err)
	}

	if repo.listCalls != 1 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 1)
	}
}

func TestFlavorServiceList_DoesNotCacheErrors(t *testing.T) {
	repo := &flavorRepoStub{listFn: func(_ context.Context, _ bool) ([]dto.Flavor, error) {
		return nil, errors.New("db unavailable")
	}}
	svc := NewFlavorService(repo)

	_, _ = svc.List(context.Background(), true)
	_, _ = svc.List(context.Background(), true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestFlavorServiceList_ActiveUsesSeparateCacheKey(t *testing.T) {
	repo := &flavorRepoStub{listFn: func(_ context.Context, onlyActive bool) ([]dto.Flavor, error) {
		if onlyActive {
			return []dto.Flavor{{Name: "Active"}}, nil
		}
		return []dto.Flavor{{Name: "All"}}, nil
	}}
	svc := NewFlavorService(repo)

	active, err := svc.List(context.Background(), true)
	if err != nil {
		t.Fatalf("List(true) error = %v", err)
	}
	if active[0].Name != "Active" {
		t.Fatalf("active name = %q, want %q", active[0].Name, "Active")
	}

	all, err := svc.List(context.Background(), false)
	if err != nil {
		t.Fatalf("List(false) error = %v", err)
	}
	if all[0].Name != "All" {
		t.Fatalf("all name = %q, want %q", all[0].Name, "All")
	}

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestFlavorServiceCreate_InvalidatesCache(t *testing.T) {
	repo := &flavorRepoStub{
		listFn: func(_ context.Context, _ bool) ([]dto.Flavor, error) {
			return []dto.Flavor{{Name: "Vanilla"}}, nil
		},
		createFn: func(_ context.Context, _ repository.CreateFlavorInput) (dto.Flavor, error) {
			return dto.Flavor{Name: "Chocolate"}, nil
		},
	}
	svc := NewFlavorService(repo)

	_, _ = svc.List(context.Background(), true)
	displayOrder := 1
	_, _ = svc.Create(context.Background(), repository.CreateFlavorInput{Name: "Chocolate", DisplayOrder: &displayOrder, IsActive: true})
	_, _ = svc.List(context.Background(), true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestFlavorServiceGetByID_NotFound(t *testing.T) {
	repo := &flavorRepoStub{findByID: func(_ context.Context, _ uuid.UUID) (dto.Flavor, error) {
		return dto.Flavor{}, &apperror.NotFoundError{Resource: "flavor"}
	}}
	svc := NewFlavorService(repo)

	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFlavorServiceUpdate_InvalidatesCache(t *testing.T) {
	repo := &flavorRepoStub{
		listFn: func(_ context.Context, _ bool) ([]dto.Flavor, error) {
			return []dto.Flavor{{Name: "Vanilla"}}, nil
		},
		updateFn: func(_ context.Context, _ uuid.UUID, _ repository.UpdateFlavorInput) (dto.Flavor, error) {
			return dto.Flavor{Name: "Updated"}, nil
		},
	}
	svc := NewFlavorService(repo)

	_, _ = svc.List(context.Background(), true)
	_, _ = svc.Update(context.Background(), uuid.New(), repository.UpdateFlavorInput{})
	_, _ = svc.List(context.Background(), true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestFlavorServiceDelete_InvalidatesCache(t *testing.T) {
	repo := &flavorRepoStub{
		listFn: func(_ context.Context, _ bool) ([]dto.Flavor, error) {
			return []dto.Flavor{{Name: "Vanilla"}}, nil
		},
		deleteFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	svc := NewFlavorService(repo)

	_, _ = svc.List(context.Background(), true)
	_ = svc.Delete(context.Background(), uuid.New())
	_, _ = svc.List(context.Background(), true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}
