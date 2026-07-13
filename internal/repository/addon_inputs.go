package repository

// CreateAddonInput is the data required to create an addon.
// DisplayOrder is optional; when nil the repository assigns the next available order.
type CreateAddonInput struct {
	Name         string
	Price        float64
	GroupName    string
	DisplayOrder *int
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
