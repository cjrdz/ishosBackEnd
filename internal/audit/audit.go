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
)

// AuditEntry represents a logged action with actor, resource, and changes.
type AuditEntry struct {
	ID        string         // UUID or identifier
	Action    string         // "order.created", "order.approved", "settings.updated"
	ActorID   string         // Employee/user who performed the action
	Resource  string         // "order:12345", "settings:store_hours"
	Changes   map[string]any // Before/after values for the change
	IP        string         // Client IP address
	CreatedAt time.Time      // When the action occurred
}

// Service logs audit entries to the database.
type Service interface {
	// Log creates an audit entry for a sensitive action.
	Log(ctx context.Context, action, actorID, resource string, changes map[string]any, ip string) error

	// GetByResource retrieves all audit entries for a specific resource.
	GetByResource(ctx context.Context, resource string, limit, offset int) ([]*AuditEntry, error)

	// GetByActor retrieves all audit entries performed by a specific actor.
	GetByActor(ctx context.Context, actorID string, limit, offset int) ([]*AuditEntry, error)

	// GetByAction retrieves all entries of a specific action type (e.g., "order.approved").
	GetByAction(ctx context.Context, action string, limit, offset int) ([]*AuditEntry, error)

	// CleanupOlderThan deletes audit entries older than the given duration.
	// Useful for retention policies (e.g., delete entries older than 90 days).
	CleanupOlderThan(ctx context.Context, before time.Time) (deletedCount int, err error)
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
