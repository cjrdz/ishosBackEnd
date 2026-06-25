// customization.go — Order customization domain rules and validation.
// Encodes business rules for flavors, addons, toppings, and pricing.
package order

import (
	"fmt"
	"maps"
)

// CustomizationRule represents a single customization choice (flavor, addon, topping).
type CustomizationRule struct {
	// Key is the group name: "flavor", "addon_salsa", "addon_jalea", etc.
	Key string
	// Value is the selected value (e.g., "chilaquiles", "sour_cream")
	Value string
	// Price is the additional cost, 0 if free/included
	Price float64
}

// Customizations is a map of group → value, representing all choices for an order item.
type Customizations map[string]interface{}

// NewCustomizations creates a new customizations map.
func NewCustomizations() Customizations {
	return make(map[string]interface{})
}

// Set adds or updates a customization choice.
func (c Customizations) Set(key string, value interface{}) {
	c[key] = value
}

// Get retrieves a customization value by key.
func (c Customizations) Get(key string) (interface{}, bool) {
	val, ok := c[key]
	return val, ok
}

// Clone creates a shallow copy of the customizations.
func (c Customizations) Clone() Customizations {
	return maps.Clone(c)
}

// GetString retrieves a customization as a string, returning empty if not found.
func (c Customizations) GetString(key string) string {
	val, ok := c[key]
	if !ok {
		return ""
	}
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}

// GetStrings retrieves a customization as a string slice.
// Returns empty slice if not found or not a slice.
func (c Customizations) GetStrings(key string) []string {
	val, ok := c[key]
	if !ok {
		return []string{}
	}

	// Try to cast to []interface{} (JSON unmarshals as this)
	if ifaces, ok := val.([]interface{}); ok {
		result := make([]string, 0, len(ifaces))
		for _, iface := range ifaces {
			if str, ok := iface.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}

	// Try direct cast to []string
	if strs, ok := val.([]string); ok {
		return strs
	}

	return []string{}
}

// CustomizationValidator encodes business rules for customizations.
type CustomizationValidator struct {
	// FreeAddonAllowanceByGroup defines how many free addons are allowed per group
	// e.g., {"toppings": 1, "jalea": 1} means 1 free topping + 1 free jalea
	FreeAddonAllowanceByGroup map[string]int

	// ValidFlavors is the set of valid flavor IDs
	ValidFlavors map[string]bool

	// ValidAddons is the set of valid addon IDs
	ValidAddons map[string]bool

	// AddonPrices maps addon ID → unit price
	AddonPrices map[string]float64
}

// NewCustomizationValidator creates a new validator with empty rules.
func NewCustomizationValidator() *CustomizationValidator {
	return &CustomizationValidator{
		FreeAddonAllowanceByGroup: make(map[string]int),
		ValidFlavors:              make(map[string]bool),
		ValidAddons:               make(map[string]bool),
		AddonPrices:               make(map[string]float64),
	}
}

// WithFreeAddonAllowance sets the free addon allowance for a group.
func (v *CustomizationValidator) WithFreeAddonAllowance(group string, count int) *CustomizationValidator {
	v.FreeAddonAllowanceByGroup[group] = count
	return v
}

// WithValidFlavor adds a valid flavor ID.
func (v *CustomizationValidator) WithValidFlavor(flavorID string) *CustomizationValidator {
	v.ValidFlavors[flavorID] = true
	return v
}

// WithValidAddon adds a valid addon ID with optional price.
func (v *CustomizationValidator) WithValidAddon(addonID string, price float64) *CustomizationValidator {
	v.ValidAddons[addonID] = true
	v.AddonPrices[addonID] = price
	return v
}

// ValidateFlavor checks if a flavor selection is valid.
func (v *CustomizationValidator) ValidateFlavor(flavorID string) error {
	if flavorID == "" {
		return fmt.Errorf("flavor cannot be empty")
	}
	if !v.ValidFlavors[flavorID] {
		return fmt.Errorf("invalid flavor: %s", flavorID)
	}
	return nil
}

// ValidateAddons checks if a list of addon selections is valid and returns total addon cost.
// Enforces free addon allowance per group.
func (v *CustomizationValidator) ValidateAddons(group string, addonIDs []string) (float64, error) {
	if len(addonIDs) == 0 {
		return 0, nil
	}

	freeAllowance := v.FreeAddonAllowanceByGroup[group]
	totalCost := 0.0

	for i, addonID := range addonIDs {
		if addonID == "" {
			return 0, fmt.Errorf("addon cannot be empty")
		}
		if !v.ValidAddons[addonID] {
			return 0, fmt.Errorf("invalid addon: %s", addonID)
		}

		// First N addons are free (up to freeAllowance), rest are paid
		if i >= freeAllowance {
			totalCost += v.AddonPrices[addonID]
		}
	}

	return totalCost, nil
}

// ValidateCustomizations validates a complete customization map.
// Returns the total addon cost for this item.
func (v *CustomizationValidator) ValidateCustomizations(custom Customizations) (float64, error) {
	totalAddonCost := 0.0

	// Validate flavor if present
	if flavorVal, ok := custom.Get("flavor"); ok {
		if flavorID, ok := flavorVal.(string); ok {
			if err := v.ValidateFlavor(flavorID); err != nil {
				return 0, err
			}
		} else {
			return 0, fmt.Errorf("flavor must be a string")
		}
	}

	// Validate each addon group
	for group := range v.FreeAddonAllowanceByGroup {
		if addonVals, ok := custom.Get("addon_" + group); ok {
			addonIDs := []string{}

			// Handle both string and []string formats
			if str, ok := addonVals.(string); ok {
				if str != "" {
					addonIDs = []string{str}
				}
			} else if ifaces, ok := addonVals.([]interface{}); ok {
				for _, iface := range ifaces {
					if str, ok := iface.(string); ok {
						addonIDs = append(addonIDs, str)
					}
				}
			}

			if len(addonIDs) > 0 {
				cost, err := v.ValidateAddons(group, addonIDs)
				if err != nil {
					return 0, err
				}
				totalAddonCost += cost
			}
		}
	}

	return totalAddonCost, nil
}
