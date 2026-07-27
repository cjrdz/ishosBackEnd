package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

type containerTypeRepoStub struct {
	listFn   func(ctx context.Context) ([]dto.ContainerType, error)
	getByID  func(ctx context.Context, id uuid.UUID) (dto.ContainerType, error)
	createFn func(ctx context.Context, name string, ballsPerContainer int, isCustom bool) (dto.ContainerType, error)
	updateFn func(ctx context.Context, id uuid.UUID, name *string, ballsPerContainer *int, isCustom *bool, isActive *bool) (dto.ContainerType, error)
	deleteFn func(ctx context.Context, id uuid.UUID) error
}

func (s *containerTypeRepoStub) List(ctx context.Context) ([]dto.ContainerType, error) {
	if s.listFn != nil {
		return s.listFn(ctx)
	}
	return nil, nil
}

func (s *containerTypeRepoStub) GetByID(ctx context.Context, id uuid.UUID) (dto.ContainerType, error) {
	if s.getByID != nil {
		return s.getByID(ctx, id)
	}
	return dto.ContainerType{}, nil
}

func (s *containerTypeRepoStub) Create(ctx context.Context, name string, ballsPerContainer int, isCustom bool) (dto.ContainerType, error) {
	if s.createFn != nil {
		return s.createFn(ctx, name, ballsPerContainer, isCustom)
	}
	return dto.ContainerType{}, nil
}

func (s *containerTypeRepoStub) Update(ctx context.Context, id uuid.UUID, name *string, ballsPerContainer *int, isCustom *bool, isActive *bool) (dto.ContainerType, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, name, ballsPerContainer, isCustom, isActive)
	}
	return dto.ContainerType{}, nil
}

func (s *containerTypeRepoStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func TestContainerTypeServiceList(t *testing.T) {
	repo := &containerTypeRepoStub{listFn: func(_ context.Context) ([]dto.ContainerType, error) {
		return []dto.ContainerType{{Name: "Small"}, {Name: "Large"}}, nil
	}}
	svc := NewContainerTypeService(repo)

	items, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestContainerTypeServiceGetByID_NotFound(t *testing.T) {
	repo := &containerTypeRepoStub{getByID: func(_ context.Context, _ uuid.UUID) (dto.ContainerType, error) {
		return dto.ContainerType{}, &apperror.NotFoundError{Resource: "container_type"}
	}}
	svc := NewContainerTypeService(repo)

	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestContainerTypeServiceCreate(t *testing.T) {
	repo := &containerTypeRepoStub{createFn: func(_ context.Context, name string, ballsPerContainer int, isCustom bool) (dto.ContainerType, error) {
		return dto.ContainerType{Name: name, BallsPerContainer: ballsPerContainer, IsCustom: isCustom}, nil
	}}
	svc := NewContainerTypeService(repo)

	item, err := svc.Create(context.Background(), "Medium", 3, false)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}
	if item.Name != "Medium" || item.BallsPerContainer != 3 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestContainerTypeServiceUpdate(t *testing.T) {
	repo := &containerTypeRepoStub{updateFn: func(_ context.Context, _ uuid.UUID, name *string, ballsPerContainer *int, isCustom *bool, isActive *bool) (dto.ContainerType, error) {
		return dto.ContainerType{Name: *name, BallsPerContainer: *ballsPerContainer}, nil
	}}
	svc := NewContainerTypeService(repo)

	name := "Large"
	balls := 5
	item, err := svc.Update(context.Background(), uuid.New(), &name, &balls, nil, nil)
	if err != nil {
		t.Fatalf("Update error = %v", err)
	}
	if item.Name != "Large" || item.BallsPerContainer != 5 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestContainerTypeServiceDelete(t *testing.T) {
	repo := &containerTypeRepoStub{deleteFn: func(_ context.Context, _ uuid.UUID) error {
		return nil
	}}
	svc := NewContainerTypeService(repo)

	if err := svc.Delete(context.Background(), uuid.New()); err != nil {
		t.Fatalf("Delete error = %v", err)
	}
}
