// audit.go — Data-transfer object for audit log entries.
package dto

import "time"

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
