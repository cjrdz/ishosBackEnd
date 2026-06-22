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

type addonRepoStub struct {
	listCalls int
	listFn    func(ctx context.Context, onlyActive bool) ([]dto.Addon, error)
	findByID  func(ctx context.Context, id uuid.UUID) (dto.Addon, error)
	createFn  func(ctx context.Context, input repository.CreateAddonInput) (dto.Addon, error)
	updateFn  func(ctx context.Context, id uuid.UUID, input repository.UpdateAddonInput) (dto.Addon, error)
	deleteFn  func(ctx context.Context, id uuid.UUID) error
}

func (s *addonRepoStub) FindAll(ctx context.Context, onlyActive bool) ([]dto.Addon, error) {
	s.listCalls++
	if s.listFn != nil {
		return s.listFn(ctx, onlyActive)
	}
	return nil, nil
}

func (s *addonRepoStub) FindByID(ctx context.Context, id uuid.UUID) (dto.Addon, error) {
	if s.findByID != nil {
		return s.findByID(ctx, id)
	}
	return dto.Addon{}, nil
}

func (s *addonRepoStub) Create(ctx context.Context, input repository.CreateAddonInput) (dto.Addon, error) {
	if s.createFn != nil {
		return s.createFn(ctx, input)
	}
	return dto.Addon{}, nil
}

func (s *addonRepoStub) Update(ctx context.Context, id uuid.UUID, input repository.UpdateAddonInput) (dto.Addon, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, input)
	}
	return dto.Addon{}, nil
}

func (s *addonRepoStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func TestAddonServiceList_UsesCacheOnSecondCall(t *testing.T) {
	repo := &addonRepoStub{listFn: func(_ context.Context, _ bool) ([]dto.Addon, error) {
		return []dto.Addon{{Name: "Sprinkles"}}, nil
	}}
	svc := NewAddonService(repo)

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

func TestAddonServiceList_DoesNotCacheErrors(t *testing.T) {
	repo := &addonRepoStub{listFn: func(_ context.Context, _ bool) ([]dto.Addon, error) {
		return nil, errors.New("db unavailable")
	}}
	svc := NewAddonService(repo)

	_, _ = svc.List(context.Background(), true)
	_, _ = svc.List(context.Background(), true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestAddonServiceList_ActiveUsesSeparateCacheKey(t *testing.T) {
	repo := &addonRepoStub{listFn: func(_ context.Context, onlyActive bool) ([]dto.Addon, error) {
		if onlyActive {
			return []dto.Addon{{Name: "Active"}}, nil
		}
		return []dto.Addon{{Name: "All"}}, nil
	}}
	svc := NewAddonService(repo)

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

func TestAddonServiceCreate_InvalidatesCache(t *testing.T) {
	repo := &addonRepoStub{
		listFn: func(_ context.Context, _ bool) ([]dto.Addon, error) {
			return []dto.Addon{{Name: "Sprinkles"}}, nil
		},
		createFn: func(_ context.Context, _ repository.CreateAddonInput) (dto.Addon, error) {
			return dto.Addon{Name: "Caramel"}, nil
		},
	}
	svc := NewAddonService(repo)

	_, _ = svc.List(context.Background(), true)
	_, _ = svc.Create(context.Background(), repository.CreateAddonInput{Name: "Caramel", Price: 1.0, GroupName: "extras", DisplayOrder: 1, IsActive: true})
	_, _ = svc.List(context.Background(), true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestAddonServiceGetByID_NotFound(t *testing.T) {
	repo := &addonRepoStub{findByID: func(_ context.Context, _ uuid.UUID) (dto.Addon, error) {
		return dto.Addon{}, &apperror.NotFoundError{Resource: "addon"}
	}}
	svc := NewAddonService(repo)

	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAddonServiceUpdate_InvalidatesCache(t *testing.T) {
	repo := &addonRepoStub{
		listFn: func(_ context.Context, _ bool) ([]dto.Addon, error) {
			return []dto.Addon{{Name: "Sprinkles"}}, nil
		},
		updateFn: func(_ context.Context, _ uuid.UUID, _ repository.UpdateAddonInput) (dto.Addon, error) {
			return dto.Addon{Name: "Updated"}, nil
		},
	}
	svc := NewAddonService(repo)

	_, _ = svc.List(context.Background(), true)
	_, _ = svc.Update(context.Background(), uuid.New(), repository.UpdateAddonInput{})
	_, _ = svc.List(context.Background(), true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestAddonServiceDelete_InvalidatesCache(t *testing.T) {
	repo := &addonRepoStub{
		listFn: func(_ context.Context, _ bool) ([]dto.Addon, error) {
			return []dto.Addon{{Name: "Sprinkles"}}, nil
		},
		deleteFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	svc := NewAddonService(repo)

	_, _ = svc.List(context.Background(), true)
	_ = svc.Delete(context.Background(), uuid.New())
	_, _ = svc.List(context.Background(), true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}
