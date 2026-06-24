package repository

import "github.com/google/uuid"

// CreateProductInput is the data required to create a product.
type CreateProductInput struct {
	Name                 string
	Description          string
	Price                float64
	CategoryID           uuid.UUID
	ImagePath            *string
	ExcludeGlobalFlavors bool
	ExcludeGlobalAddons  bool
	BallBased            *bool
	BallQuantity         *int
	AllowsMixedFlavors   *bool
}

// UpdateProductInput is the data required to update a product.
type UpdateProductInput struct {
	Name                 *string
	Description          *string
	Price                *float64
	CategoryID           *uuid.UUID
	ImagePath            *string
	IsAvailable          *bool
	ExcludeGlobalFlavors *bool
	ExcludeGlobalAddons  *bool
	BallBased            *bool
	BallQuantity         *int
	AllowsMixedFlavors   *bool
}
