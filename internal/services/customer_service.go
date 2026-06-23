// customer_service.go — Customer management service with input sanitization.
package services

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

type CustomerService struct {
	repo CustomerRepository
}

func NewCustomerService(repo CustomerRepository) *CustomerService {
	return &CustomerService{repo: repo}
}

func (s *CustomerService) List(ctx context.Context, status, search string) ([]dto.Customer, error) {
	return s.repo.List(ctx, status, search)
}

func (s *CustomerService) ListPaginated(ctx context.Context, status, search string, offset, limit int) ([]dto.Customer, int, error) {
	return s.repo.ListPaginated(ctx, status, search, offset, limit)
}

func (s *CustomerService) GetByID(ctx context.Context, id uuid.UUID) (dto.Customer, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *CustomerService) FindByPhone(ctx context.Context, phone string) (dto.Customer, error) {
	return s.repo.FindByPhone(ctx, phone)
}

// UpsertByPhone creates a customer if the phone does not exist, otherwise
// updates the existing customer with the provided fields. It is used when
// saving a customer from an order so repeated clicks do not fail.
func (s *CustomerService) UpsertByPhone(ctx context.Context, input repository.CreateCustomerInput) (dto.Customer, error) {
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

	existing, err := s.repo.FindByPhone(ctx, input.Phone)
	if err == nil {
		update := repository.UpdateCustomerInput{}
		if input.Name != "" {
			update.Name = &input.Name
		}
		if input.UserType != "" {
			update.UserType = &input.UserType
		}
		if input.Email != nil {
			update.Email = input.Email
		}
		if input.Status != "" {
			update.Status = &input.Status
		}
		return s.repo.Update(ctx, existing.ID, update)
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		return dto.Customer{}, err
	}

	return s.repo.Create(ctx, input)
}

func (s *CustomerService) Create(ctx context.Context, input repository.CreateCustomerInput) (dto.Customer, error) {
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

func (s *CustomerService) Update(ctx context.Context, id uuid.UUID, input repository.UpdateCustomerInput) (dto.Customer, error) {
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

func (s *CustomerService) OrdersByCustomer(ctx context.Context, customerID uuid.UUID, limit int) ([]dto.Order, error) {
	if _, err := s.repo.FindByID(ctx, customerID); err != nil {
		return nil, err
	}
	return s.repo.ListOrdersByCustomerID(ctx, customerID, limit)
}
