package repository

// CreateFlavorInput is the data required to create a flavor.
// DisplayOrder is optional; when nil the repository assigns the next available order.
type CreateFlavorInput struct {
	Name         string
	DisplayOrder *int
	IsSeasonal   bool
	IsActive     bool
}

// UpdateFlavorInput is the data required to update a flavor.
type UpdateFlavorInput struct {
	Name         *string
	DisplayOrder *int
	IsSeasonal   *bool
	IsActive     *bool
}
