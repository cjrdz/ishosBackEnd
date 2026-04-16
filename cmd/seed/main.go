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
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/addon"
	"github.com/cjrdz/ishosBackEnd/ent/category"
	"github.com/cjrdz/ishosBackEnd/ent/flavor"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/database"
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

type seedFlavor struct {
	Name         string `json:"name"`
	DisplayOrder int    `json:"display_order"`
	IsSeasonal   bool   `json:"is_seasonal"`
	IsActive     *bool  `json:"is_active"`
}

type seedAddon struct {
	Name         string  `json:"name"`
	Price        float64 `json:"price"`
	GroupName    string  `json:"group_name"`
	DisplayOrder int     `json:"display_order"`
	IsActive     *bool   `json:"is_active"`
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func resolveSeedPath(explicit, fallback string) string {
	if explicit != "" {
		return explicit
	}
	if fileExists(fallback) {
		return fallback
	}
	return ""
}

func mustReadJSON(filePath string, target any) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		log.Fatal().Err(err).Str("file", filePath).Msg("Invalid seed file path")
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to resolve current working directory")
	}

	seedsRoot := filepath.Clean(filepath.Join(cwd, "seeds"))
	absClean := filepath.Clean(absPath)
	seedsPrefix := seedsRoot + string(os.PathSeparator)
	if absClean != seedsRoot && !strings.HasPrefix(absClean, seedsPrefix) {
		log.Fatal().Str("file", filePath).Msg("invalid seed file path: must be under seeds/")
	}

	data, err := os.ReadFile(absClean)
	if err != nil {
		log.Fatal().Err(err).Str("file", absClean).Msg("Failed to read JSON seed file")
	}
	if err := json.Unmarshal(data, target); err != nil {
		log.Fatal().Err(err).Str("file", absClean).Msg("Failed to parse JSON seed file")
	}
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	categoriesFile := flag.String("categories", "", "Path to products_category.json")
	productsFile := flag.String("products", "", "Path to products_product.json")
	flavorsFile := flag.String("flavors", "", "Path to flavors.json")
	addonsFile := flag.String("addons", "", "Path to addons.json")
	flag.Parse()

	resolvedCategories := resolveSeedPath(*categoriesFile, filepath.FromSlash("seeds/products_category.json"))
	resolvedProducts := resolveSeedPath(*productsFile, filepath.FromSlash("seeds/products_product.json"))
	resolvedFlavors := resolveSeedPath(*flavorsFile, filepath.FromSlash("seeds/flavors.json"))
	resolvedAddons := resolveSeedPath(*addonsFile, filepath.FromSlash("seeds/addons.json"))

	catalogRequested := resolvedCategories != "" || resolvedProducts != ""
	optionsRequested := resolvedFlavors != "" || resolvedAddons != ""

	if !catalogRequested && !optionsRequested {
		log.Fatal().Msg("No seed files found. Provide --categories/--products and/or --flavors/--addons, or create default files under seeds/")
	}
	if (resolvedCategories == "") != (resolvedProducts == "") {
		log.Fatal().Msg("Categories and products must be provided together")
	}
	if (resolvedFlavors == "") != (resolvedAddons == "") {
		log.Fatal().Msg("Flavors and addons must be provided together")
	}

	cfg := config.Load()
	db, rawDB, err := database.OpenClient(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer db.Close()

	ctx := context.Background()

	if err := database.EnsureSchema(ctx, db, rawDB); err != nil {
		log.Fatal().Err(err).Msg("Failed to verify database schema")
	}
	log.Info().Msg("Database schema verified")

	createdCategories := 0
	createdProducts := 0
	createdFlavors := 0
	createdAddons := 0

	if resolvedCategories != "" {
		var seedCategories []seedCategory
		mustReadJSON(resolvedCategories, &seedCategories)

		// Map old integer IDs to UUID-backed Ent categories.
		catMap := make(map[int]*ent.Category)

		for i, sc := range seedCategories {
			exists, _ := db.Category.Query().Where(category.SlugEQ(sc.Slug)).Exist(ctx)
			if exists {
				log.Info().Str("slug", sc.Slug).Msg("Category already exists, skipping")
				existing, queryErr := db.Category.Query().Where(category.SlugEQ(sc.Slug)).Only(ctx)
				if queryErr == nil {
					catMap[sc.ID] = existing
				}
				continue
			}

			now := time.Now()
			var imgPath *string
			if sc.Image != "" {
				imgPath = &sc.Image
			}

			created, createErr := db.Category.Create().
				SetName(sc.Name).
				SetSlug(sc.Slug).
				SetDescription(sc.Description).
				SetNillableImagePath(imgPath).
				SetDisplayOrder(i).
				SetIsActive(sc.IsActive).
				SetCreatedAt(now).
				SetUpdatedAt(now).
				Save(ctx)
			if createErr != nil {
				log.Error().Err(createErr).Str("name", sc.Name).Msg("Failed to create category")
				continue
			}

			createdCategories++
			catMap[sc.ID] = created
			log.Info().Str("name", sc.Name).Str("id", created.ID.String()).Msg("Category created")
		}

		var seedProducts []seedProduct
		mustReadJSON(resolvedProducts, &seedProducts)

		for _, sp := range seedProducts {
			var price float64
			if _, scanErr := fmt.Sscanf(sp.Price, "%f", &price); scanErr != nil {
				log.Error().Err(scanErr).Str("name", sp.Name).Msg("Failed to parse price")
				continue
			}

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

			created, createErr := db.Product.Create().
				SetName(sp.Name).
				SetDescription(sp.Description).
				SetPrice(price).
				SetNillableImagePath(imgPath).
				SetIsAvailable(sp.IsActive).
				SetCategoryID(cat.ID).
				SetCreatedAt(now).
				SetUpdatedAt(now).
				Save(ctx)
			if createErr != nil {
				log.Error().Err(createErr).Str("name", sp.Name).Msg("Failed to create product")
				continue
			}

			createdProducts++
			log.Info().
				Str("name", sp.Name).
				Str("id", created.ID.String()).
				Str("category", cat.Name).
				Float64("price", price).
				Msg("Product created")
		}
	}

	if resolvedFlavors != "" {
		var seedFlavors []seedFlavor
		mustReadJSON(resolvedFlavors, &seedFlavors)

		for _, sf := range seedFlavors {
			exists, _ := db.Flavor.Query().Where(flavor.NameEQ(sf.Name)).Exist(ctx)
			if exists {
				log.Info().Str("name", sf.Name).Msg("Flavor already exists, skipping")
				continue
			}

			now := time.Now()
			active := true
			if sf.IsActive != nil {
				active = *sf.IsActive
			}

			created, createErr := db.Flavor.Create().
				SetName(sf.Name).
				SetDisplayOrder(sf.DisplayOrder).
				SetIsSeasonal(sf.IsSeasonal).
				SetIsActive(active).
				SetCreatedAt(now).
				SetUpdatedAt(now).
				Save(ctx)
			if createErr != nil {
				log.Error().Err(createErr).Str("name", sf.Name).Msg("Failed to create flavor")
				continue
			}

			createdFlavors++
			log.Info().Str("name", sf.Name).Str("id", created.ID.String()).Msg("Flavor created")
		}

		var seedAddons []seedAddon
		mustReadJSON(resolvedAddons, &seedAddons)

		for _, sa := range seedAddons {
			exists, _ := db.Addon.Query().Where(addon.NameEQ(sa.Name)).Exist(ctx)
			if exists {
				log.Info().Str("name", sa.Name).Msg("Addon already exists, skipping")
				continue
			}

			now := time.Now()
			active := true
			if sa.IsActive != nil {
				active = *sa.IsActive
			}
			groupName := sa.GroupName
			if groupName == "" {
				groupName = "extras"
			}

			created, createErr := db.Addon.Create().
				SetName(sa.Name).
				SetPrice(sa.Price).
				SetGroupName(groupName).
				SetDisplayOrder(sa.DisplayOrder).
				SetIsActive(active).
				SetCreatedAt(now).
				SetUpdatedAt(now).
				Save(ctx)
			if createErr != nil {
				log.Error().Err(createErr).Str("name", sa.Name).Msg("Failed to create addon")
				continue
			}

			createdAddons++
			log.Info().
				Str("name", sa.Name).
				Float64("price", sa.Price).
				Str("id", created.ID.String()).
				Msg("Addon created")
		}
	}

	log.Info().
		Str("categories_file", resolvedCategories).
		Str("products_file", resolvedProducts).
		Str("flavors_file", resolvedFlavors).
		Str("addons_file", resolvedAddons).
		Int("categories_created", createdCategories).
		Int("products_created", createdProducts).
		Int("flavors_created", createdFlavors).
		Int("addons_created", createdAddons).
		Msg("Seed complete!")
}
