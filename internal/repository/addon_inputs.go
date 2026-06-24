package repository

// CreateAddonInput is the data required to create an addon.
type CreateAddonInput struct {
	Name         string
	Price        float64
	GroupName    string
	DisplayOrder int
	IsActive     bool
}

// UpdateAddonInput is the data required to update an addon.
type UpdateAddonInput struct {
	Name         *string
	Price        *float64
	GroupName    *string
	DisplayOrder *int
	IsActive     *bool
}
