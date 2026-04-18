package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

type productRepoStub struct {
	listCalls int
	listFn    func(ctx context.Context, categorySlug string, onlyAvailable bool) ([]*ent.Product, error)
}

func (s *productRepoStub) FindAll(ctx context.Context, categorySlug string, onlyAvailable bool) ([]*ent.Product, error) {
	s.listCalls++
	if s.listFn != nil {
		return s.listFn(ctx, categorySlug, onlyAvailable)
	}
	return nil, nil
}

func (s *productRepoStub) FindByIDWithCategory(context.Context, uuid.UUID) (*ent.Product, error) {
	return nil, nil
}
func (s *productRepoStub) Create(context.Context, repository.CreateProductInput) (*ent.Product, error) {
	return nil, nil
}
func (s *productRepoStub) Update(context.Context, uuid.UUID, repository.UpdateProductInput) (*ent.Product, error) {
	return nil, nil
}
func (s *productRepoStub) Delete(context.Context, uuid.UUID) error {
	return nil
}
func (s *productRepoStub) ToggleAvailability(context.Context, uuid.UUID) (*ent.Product, error) {
	return nil, nil
}
func (s *productRepoStub) LinkFlavorToProduct(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (s *productRepoStub) UnlinkFlavorFromProduct(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (s *productRepoStub) LinkAddonToProduct(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (s *productRepoStub) UnlinkAddonFromProduct(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func TestProductServiceList_UsesCacheOnSecondCall(t *testing.T) {
	repo := &productRepoStub{listFn: func(_ context.Context, _ string, _ bool) ([]*ent.Product, error) {
		return []*ent.Product{{Name: "Vanilla"}}, nil
	}}
	svc := NewProductService(repo)

	_, err := svc.List(context.Background(), "", false)
	if err != nil {
		t.Fatalf("first List() error = %v", err)
	}
	_, err = svc.List(context.Background(), "", false)
	if err != nil {
		t.Fatalf("second List() error = %v", err)
	}

	if repo.listCalls != 1 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 1)
	}
}

func TestProductServiceList_DoesNotCacheErrors(t *testing.T) {
	repo := &productRepoStub{listFn: func(_ context.Context, _ string, _ bool) ([]*ent.Product, error) {
		return nil, errors.New("db unavailable")
	}}
	svc := NewProductService(repo)

	_, _ = svc.List(context.Background(), "", false)
	_, _ = svc.List(context.Background(), "", false)

	if repo.listCalls != 2 {
		t.Fatalf("repo list calls = %d, want %d", repo.listCalls, 2)
	}
}
