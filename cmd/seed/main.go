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
	"github.com/cjrdz/ishosBackEnd/ent/containertype"
	"github.com/cjrdz/ishosBackEnd/ent/flavor"
	"github.com/cjrdz/ishosBackEnd/ent/inventoryitem"
	"github.com/cjrdz/ishosBackEnd/ent/product"
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
	ID                   int    `json:"id"`
	Name                 string `json:"name"`
	Description          string `json:"description"`
	Price                string `json:"price"`
	ImagePath            string `json:"image_path"`
	IsAvailable          bool   `json:"is_available"`
	StockStatus          string `json:"stock_status"`
	ExcludeGlobalFlavors bool   `json:"exclude_global_flavors"`
	ExcludeGlobalAddons  bool   `json:"exclude_global_addons"`
	BallBased            bool   `json:"ball_based"`
	BallQuantity         int    `json:"ball_quantity"`
	AllowsMixedFlavors   bool   `json:"allows_mixed_flavors"`
	CategoryID           int    `json:"category_id"`
}

// seedFlavor matches the JSON shape from flavors.json
type seedFlavor struct {
	Name         string `json:"name"`
	DisplayOrder int    `json:"display_order"`
	IsSeasonal   bool   `json:"is_seasonal"`
	IsActive     *bool  `json:"is_active"`
}

// seedAddon matches the JSON shape from addons.json
type seedAddon struct {
	Name         string  `json:"name"`
	Price        float64 `json:"price"`
	GroupName    string  `json:"group_name"`
	DisplayOrder int     `json:"display_order"`
	IsActive     *bool   `json:"is_active"`
}

// seedContainerType matches the JSON shape from inventory.json
type seedContainerType struct {
	Name              string `json:"name"`
	BallsPerContainer int    `json:"balls_per_container"`
	IsCustom          bool   `json:"is_custom"`
	IsActive          bool   `json:"is_active"`
}

// seedInventoryItem matches the JSON shape from inventory.json
type seedInventoryItem struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	LowStockThreshold int    `json:"low_stock_threshold"`
}

// seedInventory matches the JSON shape from inventory.json
type seedInventory struct {
	ContainerTypes  []seedContainerType  `json:"container_types"`
	InventoryItems  []seedInventoryItem  `json:"inventory_items"`
}

// fileExists checks if a file exists at the given path
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// resolveSeedPath determines which file path to use for a seed file
func resolveSeedPath(explicit, fallback string) string {
	if explicit != "" {
		return explicit
	}
	if fileExists(fallback) {
		return fallback
	}
	return ""
}

// mustReadJSON reads a JSON file and unmarshals it into the target struct, with safety checks on the file path
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

// main is the entry point for the seed command. It reads seed data from JSON files
func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	categoriesFile := flag.String("categories", "", "Path to products_category.json")
	productsFile := flag.String("products", "", "Path to products_product.json")
	flavorsFile := flag.String("flavors", "", "Path to flavors.json")
	addonsFile := flag.String("addons", "", "Path to addons.json")
	inventoryFile := flag.String("inventory", "", "Path to inventory.json")
	flag.Parse()

	resolvedCategories := resolveSeedPath(*categoriesFile, filepath.FromSlash("seeds/products_category.json"))
	resolvedProducts := resolveSeedPath(*productsFile, filepath.FromSlash("seeds/products_product.json"))
	resolvedFlavors := resolveSeedPath(*flavorsFile, filepath.FromSlash("seeds/flavors.json"))
	resolvedAddons := resolveSeedPath(*addonsFile, filepath.FromSlash("seeds/addons.json"))
	resolvedInventory := resolveSeedPath(*inventoryFile, filepath.FromSlash("seeds/inventory.json"))

	catalogRequested := resolvedCategories != "" || resolvedProducts != ""
	optionsRequested := resolvedFlavors != "" || resolvedAddons != ""
	inventoryRequested := resolvedInventory != ""

	if !catalogRequested && !optionsRequested && !inventoryRequested {
		log.Error().Msg("No seed files found. Provide --categories/--products, --flavors/--addons, or --inventory, or create default files under seeds/")
		return
	}
	if (resolvedCategories == "") != (resolvedProducts == "") {
		log.Error().Msg("Categories and products must be provided together")
		return
	}
	if (resolvedFlavors == "") != (resolvedAddons == "") {
		log.Error().Msg("Flavors and addons must be provided together")
		return
	}

	cfg := config.Load()
	db, rawDB, err := database.OpenClient(cfg.DatabaseURL)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to PostgreSQL")
		return
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	if err := database.EnsureSchema(ctx, db, rawDB); err != nil {
		log.Error().Err(err).Msg("Failed to verify database schema")
		return
	}
	log.Info().Msg("Database schema verified")

	createdCategories := 0
	updatedCategories := 0
	createdProducts := 0
	updatedProducts := 0
	createdFlavors := 0
	createdAddons := 0
	createdContainerTypes := 0
	createdInventoryItems := 0

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
			if sp.ImagePath != "" {
				imgPath = &sp.ImagePath
			}

			// Check if product exists by name
			existing, existsErr := db.Product.Query().Where(product.NameEQ(sp.Name)).Only(ctx)
			if existsErr == nil && existing != nil {
				// Update existing product
				update := db.Product.UpdateOneID(existing.ID).
					SetDescription(sp.Description).
					SetPrice(price).
					SetNillableImagePath(imgPath).
					SetIsAvailable(sp.IsAvailable).
					SetExcludeGlobalFlavors(sp.ExcludeGlobalFlavors).
					SetExcludeGlobalAddons(sp.ExcludeGlobalAddons).
					SetBallBased(sp.BallBased).
					SetBallQuantity(sp.BallQuantity).
					SetAllowsMixedFlavors(sp.AllowsMixedFlavors).
					SetCategoryID(cat.ID).
					SetUpdatedAt(now)
				if _, updateErr := update.Save(ctx); updateErr != nil {
					log.Error().Err(updateErr).Str("name", sp.Name).Msg("Failed to update product")
					continue
				}
				updatedProducts++
				log.Info().
					Str("name", sp.Name).
					Str("id", existing.ID.String()).
					Str("category", cat.Name).
					Float64("price", price).
					Msg("Product updated")
				continue
			}

			created, createErr := db.Product.Create().
				SetName(sp.Name).
				SetDescription(sp.Description).
				SetPrice(price).
				SetNillableImagePath(imgPath).
				SetIsAvailable(sp.IsAvailable).
				SetExcludeGlobalFlavors(sp.ExcludeGlobalFlavors).
				SetExcludeGlobalAddons(sp.ExcludeGlobalAddons).
				SetBallBased(sp.BallBased).
				SetBallQuantity(sp.BallQuantity).
				SetAllowsMixedFlavors(sp.AllowsMixedFlavors).
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

	if resolvedInventory != "" {
		var seedInventoryData seedInventory
		mustReadJSON(resolvedInventory, &seedInventoryData)

		// Seed container types
		for _, sct := range seedInventoryData.ContainerTypes {
			exists, _ := db.ContainerType.Query().Where(containertype.NameEQ(sct.Name)).Exist(ctx)
			if exists {
				log.Info().Str("name", sct.Name).Msg("Container type already exists, skipping")
				continue
			}

			now := time.Now()
			created, createErr := db.ContainerType.Create().
				SetName(sct.Name).
				SetBallsPerContainer(sct.BallsPerContainer).
				SetIsCustom(sct.IsCustom).
				SetIsActive(sct.IsActive).
				SetCreatedAt(now).
				SetUpdatedAt(now).
				Save(ctx)
			if createErr != nil {
				log.Error().Err(createErr).Str("name", sct.Name).Msg("Failed to create container type")
				continue
			}

			createdContainerTypes++
			log.Info().Str("name", sct.Name).Str("id", created.ID.String()).Msg("Container type created")
		}

		// Seed inventory items
		for _, sii := range seedInventoryData.InventoryItems {
			exists, _ := db.InventoryItem.Query().Where(inventoryitem.NameEQ(sii.Name)).Exist(ctx)
			if exists {
				log.Info().Str("name", sii.Name).Msg("Inventory item already exists, skipping")
				continue
			}

			now := time.Now()
			create := db.InventoryItem.Create().
				SetName(sii.Name).
				SetType(inventoryitem.Type(sii.Type)).
				SetLowStockThreshold(sii.LowStockThreshold).
				SetCreatedAt(now).
				SetUpdatedAt(now)

			// For ball-based items: link to matching flavor
			if sii.Type == "ball_based" {
				flv, flvErr := db.Flavor.Query().Where(flavor.NameEQ(sii.Name)).Only(ctx)
				if flvErr == nil {
					create.SetFlavorID(flv.ID)
					log.Info().Str("name", sii.Name).Str("flavor_id", flv.ID.String()).Msg("Linked inventory item to flavor")
				} else {
					log.Warn().Str("name", sii.Name).Msg("No matching flavor found for inventory item")
				}
			}

			// For unit-based items: link to matching product
			if sii.Type == "unit_based" {
				prod, prodErr := db.Product.Query().Where(product.NameEQ(sii.Name)).Only(ctx)
				if prodErr == nil {
					create.SetProductID(prod.ID)
					log.Info().Str("name", sii.Name).Str("product_id", prod.ID.String()).Msg("Linked inventory item to product")
				} else {
					log.Warn().Str("name", sii.Name).Msg("No matching product found for inventory item")
				}
			}

			created, createErr := create.Save(ctx)
			if createErr != nil {
				log.Error().Err(createErr).Str("name", sii.Name).Msg("Failed to create inventory item")
				continue
			}

			createdInventoryItems++
			log.Info().Str("name", sii.Name).Str("id", created.ID.String()).Msg("Inventory item created")
		}

		// ── Cleanup orphaned inventory items ─────────────────────────────
		// Delete items that have neither a flavor nor a product link
		deletedOrphans, err := db.InventoryItem.Delete().
			Where(
				inventoryitem.And(
					inventoryitem.FlavorIDIsNil(),
					inventoryitem.ProductIDIsNil(),
				),
			).
			Exec(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to delete orphaned inventory items")
		} else if deletedOrphans > 0 {
			log.Info().Int("count", deletedOrphans).Msg("Deleted orphaned inventory items")
		}

		// ── Auto-create unit-based inventory for existing products ───────
		unitProducts, err := db.Product.Query().
			Where(product.BallBasedEQ(false)).
			All(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to fetch unit-based products")
		} else {
			for _, prod := range unitProducts {
				exists, _ := db.InventoryItem.Query().
					Where(inventoryitem.ProductIDEQ(prod.ID)).
					Exist(ctx)
				if exists {
					continue
				}

				now := time.Now()
				_, createErr := db.InventoryItem.Create().
					SetName(prod.Name).
					SetType(inventoryitem.TypeUnitBased).
					SetProductID(prod.ID).
					SetLowStockThreshold(10).
					SetCurrentStock(0).
					SetCreatedAt(now).
					SetUpdatedAt(now).
					Save(ctx)
				if createErr != nil {
					log.Error().Err(createErr).Str("product", prod.Name).Msg("Failed to auto-create inventory item")
				} else {
					createdInventoryItems++
					log.Info().Str("product", prod.Name).Str("id", prod.ID.String()).Msg("Auto-created unit-based inventory item")
				}
			}
		}
	}

	log.Info().
		Str("categories_file", resolvedCategories).
		Str("products_file", resolvedProducts).
		Str("flavors_file", resolvedFlavors).
		Str("addons_file", resolvedAddons).
		Str("inventory_file", resolvedInventory).
		Int("categories_created", createdCategories).
		Int("categories_updated", updatedCategories).
		Int("products_created", createdProducts).
		Int("products_updated", updatedProducts).
		Int("flavors_created", createdFlavors).
		Int("addons_created", createdAddons).
		Int("container_types_created", createdContainerTypes).
		Int("inventory_items_created", createdInventoryItems).
		Msg("Seed complete!")
}
