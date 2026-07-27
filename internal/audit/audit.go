// Package audit provides audit logging functionality for tracking sensitive operations.
//
// Audit logging captures who did what, when, and from where, enabling compliance
// and security investigations.
//
// Example usage:
//
//	auditSvc := audit.NewService(client)
//	err := auditSvc.Log(ctx, "order.created", userID, fmt.Sprintf("order:%s", orderID), map[string]any{
//	    "total": 100.0,
//	    "customer": "John Doe",
//	}, "192.168.1.1")
package audit

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

// AuditEntry is an alias to dto.AuditEntry so callers can continue using the audit package name.
type AuditEntry = dto.AuditEntry

// LogRepository is the persistence contract required by audit.Service.
type LogRepository interface {
	// Log creates an audit entry for a sensitive action.
	Log(ctx context.Context, action, actorID, resource string, changes map[string]any, ip string) error

	// LogWithDB creates an audit entry using the provided bun transaction or DB
	// so the audit record is written in the same database transaction as the
	// mutation it describes.
	LogWithDB(ctx context.Context, db bun.IDB, action, actorID, resource string, changes map[string]any, ip string) error

	// GetByResource retrieves all audit entries for a specific resource.
	GetByResource(ctx context.Context, resource string, limit, offset int) ([]*AuditEntry, error)

	// GetByActor retrieves all audit entries performed by a specific actor.
	GetByActor(ctx context.Context, actorID string, limit, offset int) ([]*AuditEntry, error)

	// GetByAction retrieves all entries of a specific action type (e.g., "order.approved").
	GetByAction(ctx context.Context, action string, limit, offset int) ([]*AuditEntry, error)

	// CleanupOlderThan deletes audit entries older than the given time.
	CleanupOlderThan(ctx context.Context, before time.Time) (deletedCount int, err error)
}

// Service logs audit entries to the database.
type Service interface {
	LogRepository
}

// Critical actions that should always be audited
const (
	ActionOrderCreated    = "order.created"
	ActionOrderApproved   = "order.approved"
	ActionOrderRejected   = "order.rejected"
	ActionOrderReady      = "order.ready"
	ActionOrderUpdated    = "order.updated"
	ActionOrderDeleted    = "order.deleted"
	ActionOrderArchived   = "order.archived"
	ActionSettingsUpdated = "settings.updated"
	ActionExportTriggered = "export.triggered"
	ActionEmployeeAdded   = "employee.added"
	ActionEmployeeUpdated = "employee.updated"
	ActionEmployeeRemoved = "employee.removed"

	ActionProductCreated             = "product.created"
	ActionProductUpdated             = "product.updated"
	ActionProductDeleted             = "product.deleted"
	ActionProductAvailabilityToggled = "product.availability_toggled"

	ActionCategoryCreated = "category.created"
	ActionCategoryUpdated = "category.updated"
	ActionCategoryDeleted = "category.deleted"

	ActionFlavorCreated = "flavor.created"
	ActionFlavorUpdated = "flavor.updated"
	ActionFlavorDeleted = "flavor.deleted"

	ActionAddonCreated = "addon.created"
	ActionAddonUpdated = "addon.updated"
	ActionAddonDeleted = "addon.deleted"

	ActionContainerTypeCreated = "container_type.created"
	ActionContainerTypeUpdated = "container_type.updated"
	ActionContainerTypeDeleted = "container_type.deleted"

	ActionInventoryEntryRecorded      = "inventory.entry_recorded"
	ActionInventoryAdjustmentRecorded = "inventory.adjustment_recorded"
	ActionInventoryUnitEntryRecorded  = "inventory.unit_entry_recorded"
	ActionInventoryItemLinked         = "inventory.item_linked"
	ActionInventoryItemUnlinked       = "inventory.item_unlinked"
	ActionInventoryUnitItemCreated    = "inventory.unit_item_created"

	ActionUserCreated  = "user.created"
	ActionUserUpdated  = "user.updated"
	ActionUserDeleted  = "user.deleted"
	ActionUserUpserted = "user.upserted"

	ActionImageDeleted = "image.deleted"
)
