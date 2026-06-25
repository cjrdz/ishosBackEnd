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

type categoryRepoStub struct {
	listCalls              int
	listFn                 func(ctx context.Context, showAll bool) ([]dto.Category, error)
	findBySlugWithProducts func(ctx context.Context, slug string) (dto.Category, error)
	createFn               func(ctx context.Context, input repository.CreateCategoryInput) (dto.Category, error)
	updateFn               func(ctx context.Context, id uuid.UUID, input repository.UpdateCategoryInput) (dto.Category, error)
	deleteFn               func(ctx context.Context, id uuid.UUID) error
}

func (s *categoryRepoStub) FindAll(ctx context.Context) ([]dto.Category, error) {
	s.listCalls++
	if s.listFn != nil {
		return s.listFn(ctx, true)
	}
	return nil, nil
}

func (s *categoryRepoStub) FindActive(ctx context.Context) ([]dto.Category, error) {
	s.listCalls++
	if s.listFn != nil {
		return s.listFn(ctx, false)
	}
	return nil, nil
}

func (s *categoryRepoStub) FindByID(context.Context, uuid.UUID) (dto.Category, error) {
	return dto.Category{}, nil
}

func (s *categoryRepoStub) FindBySlug(context.Context, string) (dto.Category, error) {
	return dto.Category{}, nil
}

func (s *categoryRepoStub) FindBySlugWithProducts(ctx context.Context, slug string) (dto.Category, error) {
	if s.findBySlugWithProducts != nil {
		return s.findBySlugWithProducts(ctx, slug)
	}
	return dto.Category{}, nil
}

func (s *categoryRepoStub) Create(ctx context.Context, input repository.CreateCategoryInput) (dto.Category, error) {
	if s.createFn != nil {
		return s.createFn(ctx, input)
	}
	return dto.Category{}, nil
}

func (s *categoryRepoStub) Update(ctx context.Context, id uuid.UUID, input repository.UpdateCategoryInput) (dto.Category, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, input)
	}
	return dto.Category{}, nil
}

func (s *categoryRepoStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func TestCategoryServiceList_UsesCacheOnSecondCall(t *testing.T) {
	repo := &categoryRepoStub{listFn: func(_ context.Context, _ bool) ([]dto.Category, error) {
		return []dto.Category{{Name: "Gelatos"}}, nil
	}}
	svc := NewCategoryService(repo)

	_, err := svc.List(context.Background(), false)
	if err != nil {
		t.Fatalf("first List() error = %v", err)
	}
	_, err = svc.List(context.Background(), false)
	if err != nil {
		t.Fatalf("second List() error = %v", err)
	}

	if repo.listCalls != 1 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 1)
	}
}

func TestCategoryServiceList_DoesNotCacheErrors(t *testing.T) {
	repo := &categoryRepoStub{listFn: func(_ context.Context, _ bool) ([]dto.Category, error) {
		return nil, errors.New("db unavailable")
	}}
	svc := NewCategoryService(repo)

	_, _ = svc.List(context.Background(), false)
	_, _ = svc.List(context.Background(), false)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestCategoryServiceList_ShowAllUsesSeparateCacheKey(t *testing.T) {
	repo := &categoryRepoStub{listFn: func(_ context.Context, showAll bool) ([]dto.Category, error) {
		if showAll {
			return []dto.Category{{Name: "All"}}, nil
		}
		return []dto.Category{{Name: "Active"}}, nil
	}}
	svc := NewCategoryService(repo)

	active, err := svc.List(context.Background(), false)
	if err != nil {
		t.Fatalf("List(false) error = %v", err)
	}
	if active[0].Name != "Active" {
		t.Fatalf("active name = %q, want %q", active[0].Name, "Active")
	}

	all, err := svc.List(context.Background(), true)
	if err != nil {
		t.Fatalf("List(true) error = %v", err)
	}
	if all[0].Name != "All" {
		t.Fatalf("all name = %q, want %q", all[0].Name, "All")
	}

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestCategoryServiceCreate_InvalidatesCache(t *testing.T) {
	repo := &categoryRepoStub{
		listFn: func(_ context.Context, _ bool) ([]dto.Category, error) {
			return []dto.Category{{Name: "Gelatos"}}, nil
		},
		createFn: func(_ context.Context, _ repository.CreateCategoryInput) (dto.Category, error) {
			return dto.Category{Name: "New"}, nil
		},
	}
	svc := NewCategoryService(repo)

	// Warm cache
	_, _ = svc.List(context.Background(), false)
	// Create should invalidate
	_, _ = svc.Create(context.Background(), repository.CreateCategoryInput{Name: "New", Slug: "new", DisplayOrder: 1})
	// This should hit the repo again
	_, _ = svc.List(context.Background(), false)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestCategoryServiceGetBySlug_NotFound(t *testing.T) {
	repo := &categoryRepoStub{findBySlugWithProducts: func(_ context.Context, _ string) (dto.Category, error) {
		return dto.Category{}, &apperror.NotFoundError{Resource: "category"}
	}}
	svc := NewCategoryService(repo)

	_, err := svc.GetBySlug(context.Background(), "unknown")
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
