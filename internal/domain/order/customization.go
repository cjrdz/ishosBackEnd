// customization.go — Order customization value type.
package order

// Customizations is a map of group → value, representing all choices for an order item.
type Customizations map[string]interface{}

// NewCustomizations creates a new customizations map.
func NewCustomizations() Customizations {
	return make(map[string]interface{})
}
