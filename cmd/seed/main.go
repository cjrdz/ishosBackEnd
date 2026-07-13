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

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/database"
	"github.com/cjrdz/ishosBackEnd/internal/models"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
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
	ContainerTypes []seedContainerType `json:"container_types"`
	InventoryItems []seedInventoryItem `json:"inventory_items"`
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
	rawDB, err := database.OpenRawDB(cfg.DatabaseURL)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to PostgreSQL")
		return
	}
	defer func() { _ = rawDB.Close() }()

	bunDB := database.OpenBunDB(rawDB)
	defer func() { _ = bunDB.Close() }()

	ctx := context.Background()

	if err := database.EnsureBunSchema(ctx, bunDB); err != nil {
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

		categoryRepo := repository.NewCategoryBunRepository(bunDB)
		productRepo := repository.NewProductBunRepository(bunDB)

		// Map old integer IDs to UUID-backed Bun categories.
		catMap := make(map[int]models.Category)

		for i, sc := range seedCategories {
			existing, findErr := categoryRepo.FindBySlug(ctx, sc.Slug)
			if findErr == nil {
				log.Info().Str("slug", sc.Slug).Msg("Category already exists, skipping")
				var desc string
				if existing.Description != "" {
					desc = existing.Description
				}
				catMap[sc.ID] = models.Category{
					ID:           existing.ID,
					Name:         existing.Name,
					Slug:         existing.Slug,
					Description:  &desc,
					ImagePath:    existing.ImagePath,
					DisplayOrder: existing.DisplayOrder,
					IsActive:     existing.IsActive,
					CreatedAt:    existing.CreatedAt,
					UpdatedAt:    existing.UpdatedAt,
				}
				continue
			}

			var imgPath *string
			if sc.Image != "" {
				imgPath = &sc.Image
			}

		displayOrder := i
		created, createErr := categoryRepo.Create(ctx, repository.CreateCategoryInput{
			Name:         sc.Name,
			Slug:         sc.Slug,
			Description:  sc.Description,
			ImagePath:    imgPath,
			DisplayOrder: &displayOrder,
		})
			if createErr != nil {
				log.Error().Err(createErr).Str("name", sc.Name).Msg("Failed to create category")
				continue
			}
			if !sc.IsActive {
				isActive := sc.IsActive
				if _, updateErr := categoryRepo.Update(ctx, created.ID, repository.UpdateCategoryInput{IsActive: &isActive}); updateErr != nil {
					log.Error().Err(updateErr).Str("name", sc.Name).Msg("Failed to set category active state")
				}
			}

			createdCategories++
			catMap[sc.ID] = models.Category{
				ID:           created.ID,
				Name:         created.Name,
				Slug:         created.Slug,
				Description:  &sc.Description,
				ImagePath:    imgPath,
				DisplayOrder: i,
				IsActive:     sc.IsActive,
			}
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

			var imgPath *string
			if sp.ImagePath != "" {
				imgPath = &sp.ImagePath
			}

			// Check if product exists by name
			var existing models.Product
			existsErr := bunDB.NewSelect().Model(&existing).Where("name = ?", sp.Name).Scan(ctx)
			if existsErr == nil {
				update := repository.UpdateProductInput{
					Description:          &sp.Description,
					Price:                &price,
					ImagePath:            imgPath,
					IsAvailable:          &sp.IsAvailable,
					ExcludeGlobalFlavors: &sp.ExcludeGlobalFlavors,
					ExcludeGlobalAddons:  &sp.ExcludeGlobalAddons,
					BallBased:            &sp.BallBased,
					BallQuantity:         &sp.BallQuantity,
					AllowsMixedFlavors:   &sp.AllowsMixedFlavors,
					CategoryID:           &cat.ID,
				}
				if _, updateErr := productRepo.Update(ctx, existing.ID, update); updateErr != nil {
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

			created, createErr := productRepo.Create(ctx, repository.CreateProductInput{
				Name:                 sp.Name,
				Description:          sp.Description,
				Price:                price,
				ImagePath:            imgPath,
				ExcludeGlobalFlavors: sp.ExcludeGlobalFlavors,
				ExcludeGlobalAddons:  sp.ExcludeGlobalAddons,
				BallBased:            &sp.BallBased,
				BallQuantity:         &sp.BallQuantity,
				AllowsMixedFlavors:   &sp.AllowsMixedFlavors,
				CategoryID:           cat.ID,
			})
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

		flavorRepo := repository.NewFlavorBunRepository(bunDB)
		for _, sf := range seedFlavors {
			existing, findErr := flavorRepo.FindAll(ctx, false)
			if findErr == nil {
				var exists bool
				for _, f := range existing {
					if f.Name == sf.Name {
						exists = true
						break
					}
				}
				if exists {
					log.Info().Str("name", sf.Name).Msg("Flavor already exists, skipping")
					continue
				}
			}

			active := true
			if sf.IsActive != nil {
				active = *sf.IsActive
			}

		flavorOrder := sf.DisplayOrder
		created, createErr := flavorRepo.Create(ctx, repository.CreateFlavorInput{
			Name:         sf.Name,
			DisplayOrder: &flavorOrder,
			IsSeasonal:   sf.IsSeasonal,
			IsActive:     active,
		})
			if createErr != nil {
				log.Error().Err(createErr).Str("name", sf.Name).Msg("Failed to create flavor")
				continue
			}

			createdFlavors++
			log.Info().Str("name", sf.Name).Str("id", created.ID.String()).Msg("Flavor created")
		}

		var seedAddons []seedAddon
		mustReadJSON(resolvedAddons, &seedAddons)

		addonRepo := repository.NewAddonBunRepository(bunDB)
		for _, sa := range seedAddons {
			existing, findErr := addonRepo.FindAll(ctx, false)
			if findErr == nil {
				var exists bool
				for _, a := range existing {
					if a.Name == sa.Name {
						exists = true
						break
					}
				}
				if exists {
					log.Info().Str("name", sa.Name).Msg("Addon already exists, skipping")
					continue
				}
			}

			active := true
			if sa.IsActive != nil {
				active = *sa.IsActive
			}
			groupName := sa.GroupName
			if groupName == "" {
				groupName = "extras"
			}

		addonOrder := sa.DisplayOrder
		created, createErr := addonRepo.Create(ctx, repository.CreateAddonInput{
			Name:         sa.Name,
			Price:        sa.Price,
			GroupName:    groupName,
			DisplayOrder: &addonOrder,
			IsActive:     active,
		})
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

		containerTypeRepo := repository.NewContainerTypeBunRepository(bunDB)

		// Seed container types
		for _, sct := range seedInventoryData.ContainerTypes {
			existing, _ := containerTypeRepo.List(ctx)
			var exists bool
			for _, ct := range existing {
				if ct.Name == sct.Name {
					exists = true
					break
				}
			}
			if exists {
				log.Info().Str("name", sct.Name).Msg("Container type already exists, skipping")
				continue
			}

			created, createErr := containerTypeRepo.Create(ctx, sct.Name, sct.BallsPerContainer, sct.IsCustom)
			if createErr != nil {
				log.Error().Err(createErr).Str("name", sct.Name).Msg("Failed to create container type")
				continue
			}

			if !sct.IsActive {
				isActive := sct.IsActive
				if _, updateErr := containerTypeRepo.Update(ctx, created.ID, nil, nil, nil, &isActive); updateErr != nil {
					log.Error().Err(updateErr).Str("name", sct.Name).Msg("Failed to set container type active state")
				}
			}

			createdContainerTypes++
			log.Info().Str("name", sct.Name).Str("id", created.ID.String()).Msg("Container type created")
		}

		inventoryRepo := repository.NewInventoryBunRepository(bunDB)
		flavorRepo := repository.NewFlavorBunRepository(bunDB)
		productRepo := repository.NewProductBunRepository(bunDB)

		// Seed inventory items
		for _, sii := range seedInventoryData.InventoryItems {
			existing, _ := inventoryRepo.List(ctx)
			var exists bool
			for _, ii := range existing {
				if ii.Name == sii.Name {
					exists = true
					break
				}
			}
			if exists {
				log.Info().Str("name", sii.Name).Msg("Inventory item already exists, skipping")
				continue
			}

			input := repository.CreateInventoryItemInput{
				Name:              sii.Name,
				Type:              sii.Type,
				LowStockThreshold: sii.LowStockThreshold,
			}

			// For ball-based items: link to matching flavor
			if sii.Type == "ball_based" {
				flavors, flvErr := flavorRepo.FindAll(ctx, false)
				if flvErr == nil {
					for _, f := range flavors {
						if f.Name == sii.Name {
							input.FlavorID = &f.ID
							log.Info().Str("name", sii.Name).Str("flavor_id", f.ID.String()).Msg("Linked inventory item to flavor")
							break
						}
					}
				}
				if input.FlavorID == nil {
					log.Warn().Str("name", sii.Name).Msg("No matching flavor found for inventory item")
				}
			}

			// For unit-based items: link to matching product
			if sii.Type == "unit_based" {
				products, prodErr := productRepo.FindAll(ctx, "", false)
				if prodErr == nil {
					for _, p := range products {
						if p.Name == sii.Name {
							input.ProductID = &p.ID
							log.Info().Str("name", sii.Name).Str("product_id", p.ID.String()).Msg("Linked inventory item to product")
							break
						}
					}
				}
				if input.ProductID == nil {
					log.Warn().Str("name", sii.Name).Msg("No matching product found for inventory item")
				}
			}

			created, createErr := inventoryRepo.Create(ctx, input)
			if createErr != nil {
				log.Error().Err(createErr).Str("name", sii.Name).Msg("Failed to create inventory item")
				continue
			}

			createdInventoryItems++
			log.Info().Str("name", sii.Name).Str("id", created.ID.String()).Msg("Inventory item created")
		}

		// ── Cleanup orphaned inventory items ─────────────────────────────
		// Delete items that have neither a flavor nor a product link
		res, err := bunDB.NewDelete().Model((*models.InventoryItem)(nil)).Where("flavor_id IS NULL AND product_id IS NULL").Exec(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to delete orphaned inventory items")
		} else if rows, _ := res.RowsAffected(); rows > 0 {
			log.Info().Int64("count", rows).Msg("Deleted orphaned inventory items")
		}

		// ── Auto-create unit-based inventory for existing products ───────
		var unitProducts []models.Product
		if err := bunDB.NewSelect().Model(&unitProducts).Where("ball_based = ?", false).Scan(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to fetch unit-based products")
		} else {
			existingItems, _ := inventoryRepo.ListByType(ctx, string(models.InventoryItemTypeUnitBased))
			linkedProductIDs := make(map[uuid.UUID]struct{})
			for _, ii := range existingItems {
				if ii.ProductID != nil {
					linkedProductIDs[*ii.ProductID] = struct{}{}
				}
			}

			for _, prod := range unitProducts {
				if _, ok := linkedProductIDs[prod.ID]; ok {
					continue
				}

				_, createErr := inventoryRepo.Create(ctx, repository.CreateInventoryItemInput{
					Name:              prod.Name,
					Type:              string(models.InventoryItemTypeUnitBased),
					ProductID:         &prod.ID,
					LowStockThreshold: 10,
				})
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
