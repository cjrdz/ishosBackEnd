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

type productRepoStub struct {
	listCalls            int
	listFn               func(ctx context.Context, categorySlug string, onlyAvailable bool) ([]dto.Product, error)
	findByID             func(ctx context.Context, id uuid.UUID) (dto.Product, error)
	findByIDWithCategory func(ctx context.Context, id uuid.UUID) (dto.Product, error)
	createFn             func(ctx context.Context, input repository.CreateProductInput) (dto.Product, error)
	updateFn             func(ctx context.Context, id uuid.UUID, input repository.UpdateProductInput) (dto.Product, error)
	deleteFn             func(ctx context.Context, id uuid.UUID) error
	toggleFn             func(ctx context.Context, id uuid.UUID) (dto.Product, error)
}

func (s *productRepoStub) FindAll(ctx context.Context, categorySlug string, onlyAvailable bool) ([]dto.Product, error) {
	s.listCalls++
	if s.listFn != nil {
		return s.listFn(ctx, categorySlug, onlyAvailable)
	}
	return nil, nil
}

func (s *productRepoStub) FindAllPaginated(ctx context.Context, categorySlug string, onlyAvailable bool, page, perPage int) ([]dto.Product, int, error) {
	s.listCalls++
	if s.listFn != nil {
		items, err := s.listFn(ctx, categorySlug, onlyAvailable)
		return items, len(items), err
	}
	return nil, 0, nil
}

func (s *productRepoStub) FindByID(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	if s.findByID != nil {
		return s.findByID(ctx, id)
	}
	return dto.Product{}, nil
}

func (s *productRepoStub) FindByIDWithCategory(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	if s.findByIDWithCategory != nil {
		return s.findByIDWithCategory(ctx, id)
	}
	return dto.Product{}, nil
}

func (s *productRepoStub) Create(ctx context.Context, input repository.CreateProductInput) (dto.Product, error) {
	if s.createFn != nil {
		return s.createFn(ctx, input)
	}
	return dto.Product{}, nil
}

func (s *productRepoStub) Update(ctx context.Context, id uuid.UUID, input repository.UpdateProductInput) (dto.Product, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, input)
	}
	return dto.Product{}, nil
}

func (s *productRepoStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *productRepoStub) ToggleAvailability(ctx context.Context, id uuid.UUID) (dto.Product, error) {
	if s.toggleFn != nil {
		return s.toggleFn(ctx, id)
	}
	return dto.Product{}, nil
}

func TestProductServiceList_UsesCacheOnSecondCall(t *testing.T) {
	repo := &productRepoStub{listFn: func(_ context.Context, _ string, _ bool) ([]dto.Product, error) {
		return []dto.Product{{Name: "Vanilla"}}, nil
	}}
	svc := NewProductService(repo)

	_, err := svc.List(context.Background(), "", true)
	if err != nil {
		t.Fatalf("first List() error = %v", err)
	}
	_, err = svc.List(context.Background(), "", true)
	if err != nil {
		t.Fatalf("second List() error = %v", err)
	}

	if repo.listCalls != 1 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 1)
	}
}

func TestProductServiceListPaginated_ReturnsPageAndTotal(t *testing.T) {
	repo := &productRepoStub{listFn: func(_ context.Context, _ string, onlyAvailable bool) ([]dto.Product, error) {
		if onlyAvailable {
			return []dto.Product{{Name: "Vanilla"}, {Name: "Chocolate"}, {Name: "Strawberry"}}, nil
		}
		return []dto.Product{}, nil
	}}
	svc := NewProductService(repo)

	products, total, err := svc.ListPaginated(context.Background(), "", false, 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(products) != 3 {
		t.Fatalf("expected 3 products from stub, got %d", len(products))
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if repo.listCalls != 1 {
		t.Fatalf("expected 1 repo call, got %d", repo.listCalls)
	}
}

func TestProductServiceList_DoesNotCacheErrors(t *testing.T) {
	repo := &productRepoStub{listFn: func(_ context.Context, _ string, _ bool) ([]dto.Product, error) {
		return nil, errors.New("db unavailable")
	}}
	svc := NewProductService(repo)

	_, _ = svc.List(context.Background(), "", true)
	_, _ = svc.List(context.Background(), "", true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestProductServiceCreate_InvalidatesCache(t *testing.T) {
	repo := &productRepoStub{
		listFn: func(_ context.Context, _ string, _ bool) ([]dto.Product, error) {
			return []dto.Product{{Name: "Vanilla"}}, nil
		},
		createFn: func(_ context.Context, _ repository.CreateProductInput) (dto.Product, error) {
			return dto.Product{Name: "Chocolate"}, nil
		},
	}
	svc := NewProductService(repo)

	_, _ = svc.List(context.Background(), "", true)
	_, _ = svc.Create(context.Background(), repository.CreateProductInput{Name: "Chocolate", Description: "Dark", Price: 1.0, CategoryID: uuid.New()})
	_, _ = svc.List(context.Background(), "", true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestProductServiceGetByIDWithCategory_NotFound(t *testing.T) {
	repo := &productRepoStub{findByIDWithCategory: func(_ context.Context, _ uuid.UUID) (dto.Product, error) {
		return dto.Product{}, &apperror.NotFoundError{Resource: "product"}
	}}
	svc := NewProductService(repo)

	_, err := svc.GetByIDWithCategory(context.Background(), uuid.New())
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProductServiceUpdate_InvalidatesCache(t *testing.T) {
	repo := &productRepoStub{
		listFn: func(_ context.Context, _ string, _ bool) ([]dto.Product, error) {
			return []dto.Product{{Name: "Vanilla"}}, nil
		},
		updateFn: func(_ context.Context, _ uuid.UUID, _ repository.UpdateProductInput) (dto.Product, error) {
			return dto.Product{Name: "Updated"}, nil
		},
	}
	svc := NewProductService(repo)

	_, _ = svc.List(context.Background(), "", true)
	_, _ = svc.Update(context.Background(), uuid.New(), repository.UpdateProductInput{})
	_, _ = svc.List(context.Background(), "", true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestProductServiceToggleAvailability_InvalidatesCache(t *testing.T) {
	repo := &productRepoStub{
		listFn: func(_ context.Context, _ string, _ bool) ([]dto.Product, error) {
			return []dto.Product{{Name: "Vanilla"}}, nil
		},
		toggleFn: func(_ context.Context, _ uuid.UUID) (dto.Product, error) {
			return dto.Product{Name: "Vanilla", IsAvailable: false}, nil
		},
	}
	svc := NewProductService(repo)

	_, _ = svc.List(context.Background(), "", true)
	_, _ = svc.ToggleAvailability(context.Background(), uuid.New())
	_, _ = svc.List(context.Background(), "", true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}

func TestProductServiceDelete_InvalidatesCache(t *testing.T) {
	repo := &productRepoStub{
		listFn: func(_ context.Context, _ string, _ bool) ([]dto.Product, error) {
			return []dto.Product{{Name: "Vanilla"}}, nil
		},
		deleteFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	svc := NewProductService(repo)

	_, _ = svc.List(context.Background(), "", true)
	_ = svc.Delete(context.Background(), uuid.New())
	_, _ = svc.List(context.Background(), "", true)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}
