// order_enrichment_service.go — Validates and enriches order customizations with pricing calculations.
package services

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

// OrderEnrichmentService handles customization validation and pricing enrichment.
type OrderEnrichmentService struct{}

// NewOrderEnrichmentService creates a new enrichment service.
func NewOrderEnrichmentService() *OrderEnrichmentService {
	return &OrderEnrichmentService{}
}

// EnrichCustomizations validates and enriches customizations with pricing.
// Returns (enriched customizations, addon extra cost, error).
func (s *OrderEnrichmentService) EnrichCustomizations(product dto.Product, raw map[string]any) (map[string]any, float64, error) {
	enriched := s.clientCustomizations(raw)
	addonExtra := 0.0
	selectedAddonGroups := make(map[string]bool)

	activeFlavors := s.activeFlavorIndex(product)
	selectedFlavorID := s.extractCustomizationString(raw, "flavor_id")
	selectedFlavorIDs := s.extractCustomizationStrings(raw, "flavor_ids")

	log.Debug().
		Str("product_name", product.Name).
		Str("product_id", product.ID.String()).
		Bool("exclude_global_flavors", product.ExcludeGlobalFlavors).
		Int("attached_flavor_count", len(product.Flavors)).
		Int("active_flavor_count", len(activeFlavors)).
		Str("selected_flavor_id", selectedFlavorID).
		Strs("selected_flavor_ids", selectedFlavorIDs).
		Msg("EnrichCustomizations: flavor validation input")

	if selectedFlavorID != "" {
		parsedFlavorID, parseErr := uuid.Parse(selectedFlavorID)
		if parseErr != nil {
			log.Error().Err(parseErr).Str("selected_flavor_id", selectedFlavorID).Msg("EnrichCustomizations: failed to parse flavor ID")
			return nil, 0, fmt.Errorf("%w: %s", ErrFlavorNotAllowed, selectedFlavorID)
		}

		selectedFlavor, ok := activeFlavors[parsedFlavorID]
		if !ok {
			log.Error().
				Str("parsed_flavor_id", parsedFlavorID.String()).
				Str("product_name", product.Name).
				Msg("EnrichCustomizations: selected flavor not in active flavor index")
			return nil, 0, fmt.Errorf("%w: %s", ErrFlavorNotAllowed, selectedFlavorID)
		}

		if enriched == nil {
			enriched = make(map[string]any)
		}
		enriched["flavor_id"] = selectedFlavor.ID.String()
		enriched["flavor_name"] = selectedFlavor.Name
	}

	if len(selectedFlavorIDs) > 0 {
		flavorNames := make([]string, 0, len(selectedFlavorIDs))
		for _, fid := range selectedFlavorIDs {
			parsedFlavorID, parseErr := uuid.Parse(fid)
			if parseErr != nil {
				log.Error().Err(parseErr).Str("flavor_id", fid).Msg("EnrichCustomizations: failed to parse flavor ID")
				return nil, 0, fmt.Errorf("%w: %s", ErrFlavorNotAllowed, fid)
			}
			selectedFlavor, ok := activeFlavors[parsedFlavorID]
			if !ok {
				log.Error().
					Str("parsed_flavor_id", parsedFlavorID.String()).
					Str("product_name", product.Name).
					Msg("EnrichCustomizations: selected flavor not in active flavor index")
				return nil, 0, fmt.Errorf("%w: %s", ErrFlavorNotAllowed, fid)
			}
			flavorNames = append(flavorNames, selectedFlavor.Name)
		}
		if enriched == nil {
			enriched = make(map[string]any)
		}
		enriched["flavor_ids"] = selectedFlavorIDs
		enriched["flavor_names"] = flavorNames
	}

	activeAddons := s.activeAddonIndex(product)

	legacyAddonIDs := s.extractCustomizationStrings(raw, "addon_ids")
	includedAddonIDs := s.extractCustomizationStrings(raw, "included_addon_ids")
	extraAddonIDs := s.extractCustomizationStrings(raw, "extra_addon_ids")

	if len(legacyAddonIDs) > 0 || len(includedAddonIDs) > 0 || len(extraAddonIDs) > 0 {
		resolveAddon := func(addonID string) (dto.Addon, error) {
			parsedAddonID, parseErr := uuid.Parse(addonID)
			if parseErr != nil {
				return dto.Addon{}, fmt.Errorf("%w: %s", ErrAddonNotAllowed, addonID)
			}

			selectedAddon, ok := activeAddons[parsedAddonID]
			if !ok {
				return dto.Addon{}, fmt.Errorf("%w: %s", ErrAddonNotAllowed, addonID)
			}
			return selectedAddon, nil
		}

		if len(includedAddonIDs) == 0 && len(extraAddonIDs) == 0 && len(legacyAddonIDs) > 0 {
			seenLegacy := make(map[uuid.UUID]struct{})
			normalizedAddonIDs := make([]string, 0, len(legacyAddonIDs))
			addonNames := make([]string, 0, len(legacyAddonIDs))
			addonPrices := make([]float64, 0, len(legacyAddonIDs))

			for _, addonID := range legacyAddonIDs {
				selectedAddon, resolveErr := resolveAddon(addonID)
				if resolveErr != nil {
					return nil, 0, resolveErr
				}
				if _, duplicate := seenLegacy[selectedAddon.ID]; duplicate {
					continue
				}
				seenLegacy[selectedAddon.ID] = struct{}{}

				selectedAddonGroups[s.normalizeAddonGroupName(selectedAddon.GroupName)] = true

				normalizedAddonIDs = append(normalizedAddonIDs, selectedAddon.ID.String())
				addonNames = append(addonNames, selectedAddon.Name)
				addonPrices = append(addonPrices, selectedAddon.Price)
				addonExtra += selectedAddon.Price
			}

			if len(normalizedAddonIDs) > 0 {
				if enriched == nil {
					enriched = make(map[string]any)
				}
				enriched["addon_ids"] = normalizedAddonIDs
				enriched["addon_names"] = addonNames
				enriched["addon_prices"] = addonPrices
			}
		} else {
			seenIncluded := make(map[uuid.UUID]struct{})
			seenExtra := make(map[uuid.UUID]struct{})
			includedByGroupCount := make(map[string]int)

			normalizedIncludedIDs := make([]string, 0, len(includedAddonIDs))
			includedNames := make([]string, 0, len(includedAddonIDs))

			normalizedExtraIDs := make([]string, 0, len(extraAddonIDs)+len(legacyAddonIDs))
			extraNames := make([]string, 0, len(extraAddonIDs)+len(legacyAddonIDs))
			extraPrices := make([]float64, 0, len(extraAddonIDs)+len(legacyAddonIDs))

			allSelectedIDs := make([]string, 0, len(includedAddonIDs)+len(extraAddonIDs))
			allSelectedNames := make([]string, 0, len(includedAddonIDs)+len(extraAddonIDs))

			for _, addonID := range includedAddonIDs {
				selectedAddon, resolveErr := resolveAddon(addonID)
				if resolveErr != nil {
					return nil, 0, resolveErr
				}
				if _, duplicate := seenIncluded[selectedAddon.ID]; duplicate {
					continue
				}
				seenIncluded[selectedAddon.ID] = struct{}{}

				selectedAddonGroups[s.normalizeAddonGroupName(selectedAddon.GroupName)] = true

				groupName := s.normalizeAddonGroupName(selectedAddon.GroupName)
				allowance := s.freeAllowanceForGroup(product, groupName)
				if allowance > 0 && includedByGroupCount[groupName] < allowance {
					includedByGroupCount[groupName]++
					normalizedIncludedIDs = append(normalizedIncludedIDs, selectedAddon.ID.String())
					includedNames = append(includedNames, selectedAddon.Name)
					allSelectedIDs = append(allSelectedIDs, selectedAddon.ID.String())
					allSelectedNames = append(allSelectedNames, selectedAddon.Name)
					continue
				}

				normalizedExtraIDs = append(normalizedExtraIDs, selectedAddon.ID.String())
				extraNames = append(extraNames, selectedAddon.Name)
				extraPrices = append(extraPrices, selectedAddon.Price)
				addonExtra += selectedAddon.Price
				allSelectedIDs = append(allSelectedIDs, selectedAddon.ID.String())
				allSelectedNames = append(allSelectedNames, selectedAddon.Name)
			}

			for _, addonID := range extraAddonIDs {
				selectedAddon, resolveErr := resolveAddon(addonID)
				if resolveErr != nil {
					return nil, 0, resolveErr
				}
				if _, duplicate := seenExtra[selectedAddon.ID]; duplicate {
					continue
				}
				seenExtra[selectedAddon.ID] = struct{}{}

				selectedAddonGroups[s.normalizeAddonGroupName(selectedAddon.GroupName)] = true

				normalizedExtraIDs = append(normalizedExtraIDs, selectedAddon.ID.String())
				extraNames = append(extraNames, selectedAddon.Name)
				extraPrices = append(extraPrices, selectedAddon.Price)
				addonExtra += selectedAddon.Price
				allSelectedIDs = append(allSelectedIDs, selectedAddon.ID.String())
				allSelectedNames = append(allSelectedNames, selectedAddon.Name)
			}

			if len(normalizedIncludedIDs) > 0 || len(normalizedExtraIDs) > 0 {
				if enriched == nil {
					enriched = make(map[string]any)
				}
				if len(normalizedIncludedIDs) > 0 {
					enriched["included_addon_ids"] = normalizedIncludedIDs
					enriched["included_addon_names"] = includedNames
				}
				if len(normalizedExtraIDs) > 0 {
					enriched["extra_addon_ids"] = normalizedExtraIDs
					enriched["extra_addon_names"] = extraNames
					enriched["extra_addon_prices"] = extraPrices
				}
				if len(allSelectedIDs) > 0 {
					enriched["all_addon_ids"] = allSelectedIDs
					enriched["all_addon_names"] = allSelectedNames
				}
			}
		}
	}

	// Check if active topping/jalea options exist in product
	hasActiveToppingOptions := false
	hasActiveJaleaOptions := false

	for _, addon := range product.Addons {
		if !addon.IsActive {
			continue
		}
		groupName := s.normalizeAddonGroupName(addon.GroupName)
		if groupName == "toppings" {
			hasActiveToppingOptions = true
		}
		if groupName == "jalea" {
			hasActiveJaleaOptions = true
		}
	}

	toppingSelection := strings.ToLower(s.extractCustomizationString(raw, "topping_selection"))
	jaleaSelection := strings.ToLower(s.extractCustomizationString(raw, "jalea_selection"))

	if hasActiveToppingOptions {
		switch {
		case selectedAddonGroups["toppings"]:
			// A specific topping addon was selected; no "none" marker needed.
		case toppingSelection == "none" || toppingSelection == "":
			// Explicit "none" or a missing selection both mean "without topping".
			if enriched == nil {
				enriched = make(map[string]any)
			}
			enriched["topping_selection"] = "none"
		default:
			return nil, 0, fmt.Errorf("%w: topping selection is required", ErrToppingRequired)
		}
	}

	if hasActiveJaleaOptions {
		switch {
		case selectedAddonGroups["jalea"]:
			// A specific jalea addon was selected; no "none" marker needed.
		case jaleaSelection == "none" || jaleaSelection == "":
			// Explicit "none" or a missing selection both mean "without jalea".
			if enriched == nil {
				enriched = make(map[string]any)
			}
			enriched["jalea_selection"] = "none"
		default:
			return nil, 0, fmt.Errorf("%w: jalea selection is required", ErrJaleaRequired)
		}
	}

	// Clean up notes field
	if enriched != nil {
		if notes, ok := enriched["notes"].(string); ok {
			trimmed := strings.TrimSpace(notes)
			if trimmed == "" {
				delete(enriched, "notes")
			} else {
				enriched["notes"] = trimmed
			}
		}
	}

	if enriched == nil {
		enriched = make(map[string]any)
	}

	return enriched, addonExtra, nil
}

// maxItemNotesRunes caps the free-text note a customer can attach to an item.
const maxItemNotesRunes = 300

// clientCustomizations returns the only client-supplied values stored as-is:
// the item note. Everything else in the stored customizations (flavor and
// add-on IDs, names, prices, selection markers) is written by
// EnrichCustomizations from validated catalog data, so a client cannot inject
// arbitrary keys (e.g. a fake "flavor_name") that staff and tracking display.
func (s *OrderEnrichmentService) clientCustomizations(raw map[string]any) map[string]any {
	if raw == nil {
		return nil
	}
	out := make(map[string]any, 1)
	if notes, ok := raw["notes"].(string); ok {
		if runes := []rune(notes); len(runes) > maxItemNotesRunes {
			notes = string(runes[:maxItemNotesRunes])
		}
		out["notes"] = notes
	}
	return out
}

// extractCustomizationString extracts a single string value from customizations.
func (s *OrderEnrichmentService) extractCustomizationString(raw map[string]any, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

// extractCustomizationStrings extracts a slice of strings from customizations.
func (s *OrderEnrichmentService) extractCustomizationStrings(raw map[string]any, key string) []string {
	if raw == nil {
		return nil
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				trimmed := strings.TrimSpace(s)
				if trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
		return result
	default:
		return nil
	}
}

// normalizeAddonGroupName normalizes addon group names for comparison.
func (s *OrderEnrichmentService) normalizeAddonGroupName(value string) string {
	if value == "" {
		return defaultAddonGroupName
	}
	return value
}

// freeAllowanceForGroup returns the number of addons from a primary group that
// are included in the product base price. Toppings are configurable per-product;
// jalea currently defaults to one.
func (s *OrderEnrichmentService) freeAllowanceForGroup(product dto.Product, groupName string) int {
	if groupName == "toppings" {
		if product.FreeToppings > 0 {
			return product.FreeToppings
		}
		return 0
	}
	return freeAddonAllowanceByGroup[groupName]
}

// activeFlavorIndex builds an index of active flavors by ID.
func (s *OrderEnrichmentService) activeFlavorIndex(product dto.Product) map[uuid.UUID]dto.Flavor {
	index := make(map[uuid.UUID]dto.Flavor)
	for i := range product.Flavors {
		flavor := product.Flavors[i]
		if flavor.IsActive {
			index[flavor.ID] = flavor
		}
	}
	return index
}

// activeAddonIndex builds an index of active addons by ID.
func (s *OrderEnrichmentService) activeAddonIndex(product dto.Product) map[uuid.UUID]dto.Addon {
	index := make(map[uuid.UUID]dto.Addon)
	for i := range product.Addons {
		addon := product.Addons[i]
		if addon.IsActive {
			index[addon.ID] = addon
		}
	}
	return index
}
