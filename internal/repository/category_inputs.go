package repository

// CreateCategoryInput is the data required to create a category.
// DisplayOrder is optional; when nil the repository assigns the next available order.
type CreateCategoryInput struct {
	Name         string
	Slug         string
	Description  string
	ImagePath    *string
	DisplayOrder *int
}

// UpdateCategoryInput is the data required to update a category.
type UpdateCategoryInput struct {
	Name         *string
	Slug         *string
	Description  *string
	ImagePath    *string
	DisplayOrder *int
	IsActive     *bool
}
