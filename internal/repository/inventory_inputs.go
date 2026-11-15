package repository

import "github.com/google/uuid"

// CreateInventoryItemInput is the data required to create an inventory item.
type CreateInventoryItemInput struct {
	LocationID        uuid.UUID
	Name              string
	Type              string
	ProductID         *uuid.UUID
	FlavorID          *uuid.UUID
	LowStockThreshold int
	CurrentStock      int
}

// UpdateInventoryItemInput is the data required to update an inventory item.
// A nil ProductID/FlavorID means "do not change" unless the corresponding Clear flag is set.
type UpdateInventoryItemInput struct {
	Name              *string
	Type              *string
	ProductID         *uuid.UUID
	FlavorID          *uuid.UUID
	ClearProductID    bool
	ClearFlavorID     bool
	LowStockThreshold *int
}

// StockDeductionInput is a single stock deduction inside an order.
type StockDeductionInput struct {
	InventoryItemID uuid.UUID
	FlavorID        *uuid.UUID
	Quantity        int
}

// RecordEntryInput is the data required to record a stock entry.
type RecordEntryInput struct {
	InventoryItemID    uuid.UUID
	ContainerTypeID    uuid.UUID
	QuantityContainers int
	TotalBalls         int
	CreatedByID        string
}

// RecordAdjustmentInput is the data required to record a stock adjustment.
type RecordAdjustmentInput struct {
	InventoryItemID uuid.UUID
	Quantity        int
	Reason          string
	CreatedByID     string
}

// RecordUnitEntryInput is the data required to record a unit-based stock entry.
type RecordUnitEntryInput struct {
	InventoryItemID uuid.UUID
	Quantity        int
	CreatedByID     string
}
