// location_bun.go — Bun-backed repository for locations (sucursales) and their Agotados.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// LocationInput holds the editable fields of a location. Nil means "keep".
type LocationInput struct {
	Slug                *string
	Name                *string
	Address             *string
	MapsURL             *string
	MapsEmbedURL        *string
	WhatsApp            *string
	Phone               *string
	Hours               map[string][]dto.HoursWindow
	SetHours            bool
	OrdersEnabled       *bool
	OnlineOrdersEnabled *bool
	IsActive            *bool
	SortOrder           *int
}

// LocationBunRepository stores locations and per-location sold-out flags.
type LocationBunRepository struct {
	db bun.IDB
}

// NewLocationBunRepository creates a location repository.
func NewLocationBunRepository(db bun.IDB) *LocationBunRepository {
	return &LocationBunRepository{db: db}
}

// List returns locations in display order.
func (r *LocationBunRepository) List(ctx context.Context) ([]dto.Location, error) {
	var rows []models.Location
	if err := r.db.NewSelect().Model(&rows).Order("sort_order ASC", "created_at ASC").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]dto.Location, len(rows))
	for i, row := range rows {
		out[i] = toLocationDTO(row)
	}
	return out, nil
}

// FindByID returns one location.
func (r *LocationBunRepository) FindByID(ctx context.Context, id uuid.UUID) (dto.Location, error) {
	var row models.Location
	if err := r.db.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx); err != nil {
		return dto.Location{}, translateLocationError(err)
	}
	return toLocationDTO(row), nil
}

// Create inserts a location; fields left nil take the column defaults.
func (r *LocationBunRepository) Create(ctx context.Context, input LocationInput) (dto.Location, error) {
	now := time.Now()
	row := models.Location{ID: uuid.New(), Hours: map[string][]models.HoursWindow{}, OrdersEnabled: true, IsActive: true, CreatedAt: now, UpdatedAt: now}
	applyLocationInput(&row, input)
	if _, err := r.db.NewInsert().Model(&row).Exec(ctx); err != nil {
		return dto.Location{}, translateLocationError(err)
	}
	return toLocationDTO(row), nil
}

// Update changes the given fields of a location.
func (r *LocationBunRepository) Update(ctx context.Context, id uuid.UUID, input LocationInput) (dto.Location, error) {
	var result dto.Location
	err := runInTx(ctx, r.db, nil, func(ctx context.Context, tx bun.Tx) error {
		var row models.Location
		if err := tx.NewSelect().Model(&row).Where("id = ?", id).For("UPDATE").Scan(ctx); err != nil {
			return translateLocationError(err)
		}
		applyLocationInput(&row, input)
		row.UpdatedAt = time.Now()
		if _, err := tx.NewUpdate().Model(&row).WherePK().Exec(ctx); err != nil {
			return translateLocationError(err)
		}
		result = toLocationDTO(row)
		return nil
	})
	return result, err
}

// SoldOut lists the products and flavours marked sold out at a location,
// plus the tracked ones whose stock there is 0.
func (r *LocationBunRepository) SoldOut(ctx context.Context, locationID uuid.UUID) (dto.SoldOutItems, error) {
	out, err := loadSoldOut(ctx, r.db, locationID)
	if err != nil {
		return out, err
	}
	var empty []models.InventoryItem
	if err := r.db.NewSelect().Model(&empty).
		Column("flavor_id", "product_id").
		Where("location_id = ?", locationID).
		Where("current_stock <= 0").
		Where("(flavor_id IS NOT NULL OR product_id IS NOT NULL)").
		Scan(ctx); err != nil {
		return out, err
	}
	out.OutOfStockProducts = []uuid.UUID{}
	out.OutOfStockFlavors = []uuid.UUID{}
	for _, item := range empty {
		if item.FlavorID != nil {
			out.OutOfStockFlavors = append(out.OutOfStockFlavors, *item.FlavorID)
		} else if item.ProductID != nil {
			out.OutOfStockProducts = append(out.OutOfStockProducts, *item.ProductID)
		}
	}
	return out, nil
}

// SetSoldOut marks (or clears) an item as sold out at a location.
func (r *LocationBunRepository) SetSoldOut(ctx context.Context, locationID uuid.UUID, itemType models.AvailabilityItemType, itemID uuid.UUID, soldOut bool, by string) error {
	if !soldOut {
		_, err := r.db.NewDelete().Model((*models.LocationAvailability)(nil)).
			Where("location_id = ? AND item_type = ? AND item_id = ?", locationID, itemType, itemID).Exec(ctx)
		return err
	}
	row := models.LocationAvailability{LocationID: locationID, ItemType: itemType, ItemID: itemID, UpdatedAt: time.Now()}
	if by != "" {
		row.UpdatedBy = &by
	}
	_, err := r.db.NewInsert().Model(&row).
		On("CONFLICT (location_id, item_type, item_id) DO UPDATE").
		Set("updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func loadSoldOut(ctx context.Context, db bun.IDB, locationID uuid.UUID) (dto.SoldOutItems, error) {
	var rows []models.LocationAvailability
	if err := db.NewSelect().Model(&rows).Where("location_id = ?", locationID).Order("updated_at ASC").Scan(ctx); err != nil {
		return dto.SoldOutItems{}, err
	}
	out := dto.SoldOutItems{LocationID: locationID, Products: []uuid.UUID{}, Flavors: []uuid.UUID{}}
	for _, row := range rows {
		switch row.ItemType {
		case models.AvailabilityItemProduct:
			out.Products = append(out.Products, row.ItemID)
		case models.AvailabilityItemFlavor:
			out.Flavors = append(out.Flavors, row.ItemID)
		}
	}
	return out, nil
}

func applyLocationInput(row *models.Location, in LocationInput) {
	setStr := func(dst *string, v *string) {
		if v != nil {
			*dst = *v
		}
	}
	setStr(&row.Slug, in.Slug)
	setStr(&row.Name, in.Name)
	setStr(&row.Address, in.Address)
	setStr(&row.MapsURL, in.MapsURL)
	setStr(&row.MapsEmbedURL, in.MapsEmbedURL)
	setStr(&row.WhatsApp, in.WhatsApp)
	setStr(&row.Phone, in.Phone)
	if in.SetHours {
		row.Hours = make(map[string][]models.HoursWindow, len(in.Hours))
		for day, windows := range in.Hours {
			for _, w := range windows {
				row.Hours[day] = append(row.Hours[day], models.HoursWindow{Open: w.Open, Close: w.Close})
			}
		}
	}
	if in.OrdersEnabled != nil {
		row.OrdersEnabled = *in.OrdersEnabled
	}
	if in.OnlineOrdersEnabled != nil {
		row.OnlineOrdersEnabled = *in.OnlineOrdersEnabled
	}
	if in.IsActive != nil {
		row.IsActive = *in.IsActive
	}
	if in.SortOrder != nil {
		row.SortOrder = *in.SortOrder
	}
}

func toLocationDTO(row models.Location) dto.Location {
	hours := make(map[string][]dto.HoursWindow, len(row.Hours))
	for day, windows := range row.Hours {
		for _, w := range windows {
			hours[day] = append(hours[day], dto.HoursWindow{Open: w.Open, Close: w.Close})
		}
	}
	return dto.Location{
		ID: row.ID, Slug: row.Slug, Name: row.Name, Address: row.Address,
		MapsURL: row.MapsURL, MapsEmbedURL: row.MapsEmbedURL, WhatsApp: row.WhatsApp, Phone: row.Phone,
		Hours: hours, OrdersEnabled: row.OrdersEnabled, OnlineOrdersEnabled: row.OnlineOrdersEnabled,
		IsActive: row.IsActive, SortOrder: row.SortOrder, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func translateLocationError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return &apperror.NotFoundError{Resource: "location", Cause: err}
	}
	if apperror.IsUniqueViolation(err) {
		return &apperror.ConflictError{Resource: "location", Reason: "slug already in use", Cause: err}
	}
	return err
}
