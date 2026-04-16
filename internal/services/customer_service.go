package services

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

type CustomerService struct {
	repo *repository.CustomerRepository
}

func NewCustomerService(repo *repository.CustomerRepository) *CustomerService {
	return &CustomerService{repo: repo}
}

func (s *CustomerService) List(ctx context.Context, status, search string) ([]*ent.Customer, error) {
	return s.repo.List(ctx, status, search)
}

func (s *CustomerService) ListPaginated(ctx context.Context, status, search string, offset, limit int) ([]*ent.Customer, int, error) {
	return s.repo.ListPaginated(ctx, status, search, offset, limit)
}

func (s *CustomerService) GetByID(ctx context.Context, id uuid.UUID) (*ent.Customer, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *CustomerService) Create(ctx context.Context, input repository.CreateCustomerInput) (*ent.Customer, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Phone = strings.TrimSpace(input.Phone)
	if input.Email != nil {
		email := strings.TrimSpace(*input.Email)
		if email == "" {
			input.Email = nil
		} else {
			input.Email = &email
		}
	}

	return s.repo.Create(ctx, input)
}

func (s *CustomerService) Update(ctx context.Context, id uuid.UUID, input repository.UpdateCustomerInput) (*ent.Customer, error) {
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		input.Name = &name
	}
	if input.Phone != nil {
		phone := strings.TrimSpace(*input.Phone)
		input.Phone = &phone
	}
	if input.Email != nil {
		email := strings.TrimSpace(*input.Email)
		input.Email = &email
	}

	return s.repo.Update(ctx, id, input)
}

func (s *CustomerService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *CustomerService) OrdersByCustomer(ctx context.Context, customerID uuid.UUID, limit int) ([]*ent.Order, error) {
	customer, err := s.repo.FindByID(ctx, customerID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListOrdersByPhone(ctx, customer.Phone, limit)
}
