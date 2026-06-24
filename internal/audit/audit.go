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

	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

// AuditEntry is an alias to dto.AuditEntry so callers can continue using the audit package name.
type AuditEntry = dto.AuditEntry

// LogRepository is the persistence contract required by audit.Service.
type LogRepository interface {
	// Log creates an audit entry for a sensitive action.
	Log(ctx context.Context, action, actorID, resource string, changes map[string]any, ip string) error

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
	ActionSettingsUpdated = "settings.updated"
	ActionExportTriggered = "export.triggered"
	ActionEmployeeAdded   = "employee.added"
	ActionEmployeeUpdated = "employee.updated"
	ActionEmployeeRemoved = "employee.removed"
)
