package repository

// CreateCustomerInput is the data required to create a customer.
type CreateCustomerInput struct {
	Name     string
	UserType string
	Phone    string
	Email    *string
	Status   string
}

// UpdateCustomerInput is the data required to update a customer.
type UpdateCustomerInput struct {
	Name     *string
	UserType *string
	Phone    *string
	Email    *string
	Status   *string
}
