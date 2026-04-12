// cmd/seed-flavors/main.go — Seeds the database with test flavors and addons.
//
// Usage:
//
//	go run ./cmd/seed-flavors
package main

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/ent/addon"
	"github.com/cjrdz/ishosBackEnd/ent/flavor"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/database"
)

type TestFlavor struct {
	Name         string
	DisplayOrder int
	IsSeasonal   bool
}

type TestAddon struct {
	Name         string
	Price        float64
	GroupName    string
	DisplayOrder int
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	cfg := config.Load()
	client, rawDB, err := database.OpenClient(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer client.Close()

	ctx := context.Background()

	if err := database.EnsureSchema(ctx, client, rawDB); err != nil {
		log.Fatal().Err(err).Msg("Failed to verify database schema")
	}
	log.Info().Msg("Database schema verified")

	// ── Seed Flavors ─────────────────────────────────────────────────────
	testFlavors := []TestFlavor{
		{Name: "Vainilla", DisplayOrder: 1, IsSeasonal: false},
		{Name: "Chocolate", DisplayOrder: 2, IsSeasonal: false},
		{Name: "Fresa", DisplayOrder: 3, IsSeasonal: false},
		{Name: "Pistacho", DisplayOrder: 4, IsSeasonal: false},
		{Name: "Menta", DisplayOrder: 5, IsSeasonal: false},
		{Name: "Limón", DisplayOrder: 6, IsSeasonal: false},
		{Name: "Café", DisplayOrder: 7, IsSeasonal: false},
		{Name: "Cookies & Cream", DisplayOrder: 8, IsSeasonal: false},
		{Name: "Pumpkin Spice", DisplayOrder: 9, IsSeasonal: true},
		{Name: "Eggnog", DisplayOrder: 10, IsSeasonal: true},
	}

	flavorCount := 0
	now := time.Now()

	for _, tf := range testFlavors {
		// Check if flavor already exists
		exists, _ := client.Flavor.Query().Where(flavor.NameEQ(tf.Name)).Exist(ctx)
		if exists {
			log.Info().Str("name", tf.Name).Msg("Flavor already exists, skipping")
			continue
		}

		f, err := client.Flavor.Create().
			SetName(tf.Name).
			SetDisplayOrder(tf.DisplayOrder).
			SetIsSeasonal(tf.IsSeasonal).
			SetIsActive(true).
			SetCreatedAt(now).
			SetUpdatedAt(now).
			Save(ctx)

		if err != nil {
			log.Error().Err(err).Str("name", tf.Name).Msg("Failed to create flavor")
			continue
		}

		flavorCount++
		log.Info().Str("name", tf.Name).Str("id", f.ID.String()).Msg("Flavor created")
	}

	// ── Seed Addons ──────────────────────────────────────────────────────
	testAddons := []TestAddon{
		{Name: "Mani", Price: 0.35, GroupName: "toppings", DisplayOrder: 1},
		{Name: "Crispy", Price: 0.35, GroupName: "toppings", DisplayOrder: 2},
		{Name: "Dulcitos", Price: 0.35, GroupName: "toppings", DisplayOrder: 3},
		{Name: "Granola", Price: 0.35, GroupName: "toppings", DisplayOrder: 4},
		{Name: "Chispas", Price: 0.35, GroupName: "toppings", DisplayOrder: 5},
		{Name: "Coco", Price: 0.35, GroupName: "toppings", DisplayOrder: 6},
		{Name: "Galleta", Price: 0.35, GroupName: "toppings", DisplayOrder: 7},
		{Name: "Jalea Roja", Price: 0.35, GroupName: "jalea", DisplayOrder: 8},
		{Name: "Jalea Frutos Rojos", Price: 0.35, GroupName: "jalea", DisplayOrder: 9},
		{Name: "Jalea Maracuya", Price: 0.35, GroupName: "jalea", DisplayOrder: 10},
		{Name: "Jalea Tamarindo", Price: 0.35, GroupName: "jalea", DisplayOrder: 11},
		{Name: "Jalea Chocolate", Price: 0.35, GroupName: "jalea", DisplayOrder: 12},
		{Name: "Jalea Caramelo", Price: 0.35, GroupName: "jalea", DisplayOrder: 13},
		{Name: "Jalea Condensada", Price: 0.35, GroupName: "jalea", DisplayOrder: 14},
		{Name: "1/2 Onza de Jalea", Price: 0.35, GroupName: "extras", DisplayOrder: 15},
		{Name: "1 Onza de Jalea", Price: 0.50, GroupName: "extras", DisplayOrder: 16},
		{Name: "1/2 Litro de Jalea", Price: 4.00, GroupName: "extras", DisplayOrder: 17},
		{Name: "Barquillos", Price: 0.35, GroupName: "extras", DisplayOrder: 18},
		{Name: "Conito", Price: 0.25, GroupName: "extras", DisplayOrder: 19},
		{Name: "Cono", Price: 0.35, GroupName: "extras", DisplayOrder: 20},
		{Name: "Crema Batida", Price: 0.50, GroupName: "extras", DisplayOrder: 21},
	}

	addonCount := 0

	for _, ta := range testAddons {
		// Check if addon already exists
		exists, _ := client.Addon.Query().Where(addon.NameEQ(ta.Name)).Exist(ctx)
		if exists {
			log.Info().Str("name", ta.Name).Msg("Addon already exists, skipping")
			continue
		}

		a, err := client.Addon.Create().
			SetName(ta.Name).
			SetPrice(ta.Price).
			SetGroupName(ta.GroupName).
			SetDisplayOrder(ta.DisplayOrder).
			SetIsActive(true).
			SetCreatedAt(now).
			SetUpdatedAt(now).
			Save(ctx)

		if err != nil {
			log.Error().Err(err).Str("name", ta.Name).Msg("Failed to create addon")
			continue
		}

		addonCount++
		log.Info().Str("name", ta.Name).Float64("price", ta.Price).Str("id", a.ID.String()).Msg("Addon created")
	}

	log.Info().
		Int("flavors", flavorCount).
		Int("addons", addonCount).
		Msg("Seed complete!")
}
