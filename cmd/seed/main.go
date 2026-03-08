// cmd/seed/main.go — Seeds the database with categories and products from JSON files.
//
// Usage:
//
//	go run ./cmd/seed --categories=path/to/products_category.json --products=path/to/products_product.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosFactory-backend/ent"
	"github.com/cjrdz/ishosFactory-backend/ent/category"
	"github.com/cjrdz/ishosFactory-backend/ent/product"
	"github.com/cjrdz/ishosFactory-backend/internal/config"
)

// seedCategory matches the JSON shape from products_category.json
type seedCategory struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Image       string `json:"image"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// seedProduct matches the JSON shape from products_product.json
type seedProduct struct {
	ID             int                    `json:"id"`
	Name           string                 `json:"name"`
	Slug           string                 `json:"slug"`
	Description    string                 `json:"description"`
	Price          string                 `json:"price"`
	Cost           string                 `json:"cost"`
	DiscountPrice  *string                `json:"discount_price"`
	SKU            string                 `json:"sku"`
	SKUType        string                 `json:"sku_type"`
	Quantity       int                    `json:"quantity"`
	ReorderLevel   int                    `json:"reorder_level"`
	Image          string                 `json:"image"`
	Images         []string               `json:"images"`
	IsActive       bool                   `json:"is_active"`
	IsFeatured     bool                   `json:"is_featured"`
	Tags           []string               `json:"tags"`
	Specifications map[string]interface{} `json:"specifications"`
	CategoryID     int                    `json:"category_id"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	categoriesFile := flag.String("categories", "", "Path to products_category.json")
	productsFile := flag.String("products", "", "Path to products_product.json")
	flag.Parse()

	if *categoriesFile == "" || *productsFile == "" {
		log.Fatal().Msg("Usage: go run ./cmd/seed --categories=<path> --products=<path>")
	}

	cfg := config.Load()

	// Connect to database
	drv, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	db := ent.NewClient(ent.Driver(drv))

	ctx := context.Background()

	// Run migrations first
	if err := db.Schema.Create(ctx); err != nil {
		_ = db.Close()
		log.Fatal().Err(err).Msg("Failed to run migrations")
	}
	log.Info().Msg("Migrations complete")

	// ── Seed Categories ──────────────────────────────────────────────────
	catData, err := os.ReadFile(*categoriesFile)
	if err != nil {
		log.Fatal().Err(err).Str("file", *categoriesFile).Msg("Failed to read categories file")
	}

	var seedCategories []seedCategory
	if err := json.Unmarshal(catData, &seedCategories); err != nil {
		log.Fatal().Err(err).Msg("Failed to parse categories JSON")
	}

	// Map old integer IDs to new UUID-based Ent categories
	// Key: old integer ID from the seed JSON, Value: the Ent category entity
	catMap := make(map[int]*ent.Category)

	for i, sc := range seedCategories {
		// Check if category already exists by slug
		exists, _ := db.Category.Query().Where(category.SlugEQ(sc.Slug)).Exist(ctx)
		if exists {
			log.Info().Str("slug", sc.Slug).Msg("Category already exists, skipping")
			existing, _ := db.Category.Query().Where(category.SlugEQ(sc.Slug)).Only(ctx)
			catMap[sc.ID] = existing
			continue
		}

		now := time.Now()
		var imgPath *string
		if sc.Image != "" {
			imgPath = &sc.Image
		}

		c, err := db.Category.Create().
			SetName(sc.Name).
			SetSlug(sc.Slug).
			SetDescription(sc.Description).
			SetNillableImagePath(imgPath).
			SetDisplayOrder(i).
			SetIsActive(sc.IsActive).
			SetCreatedAt(now).
			SetUpdatedAt(now).
			Save(ctx)
		if err != nil {
			log.Error().Err(err).Str("name", sc.Name).Msg("Failed to create category")
			continue
		}

		catMap[sc.ID] = c
		log.Info().Str("name", sc.Name).Str("id", c.ID.String()).Msg("Category created")
	}

	// ── Seed Products ────────────────────────────────────────────────────
	prodData, err := os.ReadFile(*productsFile)
	if err != nil {
		log.Fatal().Err(err).Str("file", *productsFile).Msg("Failed to read products file")
	}

	var seedProducts []seedProduct
	if err := json.Unmarshal(prodData, &seedProducts); err != nil {
		log.Fatal().Err(err).Msg("Failed to parse products JSON")
	}

	for _, sp := range seedProducts {
		// Parse price from string to float64
		var price float64
		if _, err := fmt.Sscanf(sp.Price, "%f", &price); err != nil {
			log.Error().Err(err).Str("name", sp.Name).Msg("Failed to parse price")
			continue
		}

		// Find the Ent category for this product
		cat, ok := catMap[sp.CategoryID]
		if !ok {
			log.Error().
				Int("category_id", sp.CategoryID).
				Str("product", sp.Name).
				Msg("Category not found for product, skipping")
			continue
		}

		now := time.Now()
		var imgPath *string
		if sp.Image != "" {
			imgPath = &sp.Image
		}

		// Map is_active to is_available and stock_status
		stockStatus := "in_stock"
		if !sp.IsActive {
			stockStatus = "out_of_stock"
		}

		p, err := db.Product.Create().
			SetName(sp.Name).
			SetDescription(sp.Description).
			SetPrice(price).
			SetNillableImagePath(imgPath).
			SetIsAvailable(sp.IsActive).
			SetStockStatus(product.StockStatus(stockStatus)).
			SetCategoryID(cat.ID).
			SetCreatedAt(now).
			SetUpdatedAt(now).
			Save(ctx)
		if err != nil {
			log.Error().Err(err).Str("name", sp.Name).Msg("Failed to create product")
			continue
		}

		log.Info().
			Str("name", sp.Name).
			Str("id", p.ID.String()).
			Str("category", cat.Name).
			Float64("price", price).
			Msg("Product created")
	}

	_ = db.Close()
	log.Info().
		Int("categories", len(seedCategories)).
		Int("products", len(seedProducts)).
		Msg("Seed complete!")
}
