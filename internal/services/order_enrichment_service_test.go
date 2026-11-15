package services

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

func TestEnrichCustomizations_DropsClientInjectedKeys(t *testing.T) {
	fresa := dto.Flavor{ID: uuid.New(), Name: "Fresa", IsActive: true}
	product := dto.Product{ID: uuid.New(), Name: "Cono", Price: 1, Flavors: []dto.Flavor{fresa}}

	raw := map[string]any{
		"flavor_id":      fresa.ID.String(),
		"flavor_name":    "Pistacho gratis", // forged display name
		"addon_names":    []any{"Extra falso"},
		"price_override": 0,
		"notes":          "  sin cono  " + strings.Repeat("x", 400),
	}

	enriched, extra, err := NewOrderEnrichmentService().EnrichCustomizations(product, raw)
	if err != nil {
		t.Fatal(err)
	}
	if extra != 0 {
		t.Fatalf("addon extra = %v, want 0", extra)
	}
	if enriched["flavor_name"] != "Fresa" {
		t.Errorf("flavor_name = %v, want the catalog name", enriched["flavor_name"])
	}
	for _, key := range []string{"addon_names", "price_override"} {
		if _, ok := enriched[key]; ok {
			t.Errorf("client-only key %q was stored", key)
		}
	}
	notes, _ := enriched["notes"].(string)
	if !strings.HasPrefix(notes, "sin cono") || len([]rune(notes)) > maxItemNotesRunes {
		t.Errorf("notes not trimmed/capped: %d runes, prefix %q", len([]rune(notes)), notes[:10])
	}
}

func TestEnrichCustomizations_NilStaysEmpty(t *testing.T) {
	enriched, _, err := NewOrderEnrichmentService().EnrichCustomizations(dto.Product{ID: uuid.New()}, nil)
	if err != nil || len(enriched) != 0 {
		t.Fatalf("got %v, %v; want empty map", enriched, err)
	}
}
