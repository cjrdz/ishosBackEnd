// auditlog_bun.go — Bun-backed implementation of audit.LogRepository.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// AuditLogBunRepository implements audit.LogRepository using Bun.
type AuditLogBunRepository struct {
	db bun.IDB
}

// NewAuditLogBunRepository creates a new Bun-backed audit log repository.
func NewAuditLogBunRepository(db bun.IDB) *AuditLogBunRepository {
	return &AuditLogBunRepository{db: db}
}

// Log creates an audit entry for a sensitive action.
func (r *AuditLogBunRepository) Log(ctx context.Context, action, actorID, resource string, changes map[string]any, ip string) error {
	return r.logWithDB(ctx, r.db, action, actorID, resource, changes, ip)
}

// LogWithDB creates an audit entry inside the supplied bun transaction or DB.
func (r *AuditLogBunRepository) LogWithDB(ctx context.Context, db bun.IDB, action, actorID, resource string, changes map[string]any, ip string) error {
	return r.logWithDB(ctx, db, action, actorID, resource, changes, ip)
}

func (r *AuditLogBunRepository) logWithDB(ctx context.Context, db bun.IDB, action, actorID, resource string, changes map[string]any, ip string) error {
	if db == nil {
		return nil
	}

	entry := models.AuditLog{
		ID:        uuid.New(),
		Action:    action,
		Resource:  resource,
		CreatedAt: time.Now(),
	}
	if actorID != "" {
		entry.ActorID = &actorID
	}
	if ip != "" {
		entry.IP = &ip
	}
	if len(changes) > 0 {
		entry.Changes = changes
	}

	if _, err := db.NewInsert().Model(&entry).Exec(ctx); err != nil {
		return fmt.Errorf("audit log: %w", err)
	}
	return nil
}

// GetByResource retrieves audit entries for a specific resource.
func (r *AuditLogBunRepository) GetByResource(ctx context.Context, resource string, limit, offset int) ([]*dto.AuditEntry, error) {
	if r.db == nil {
		return nil, nil
	}

	q := r.db.NewSelect().Model((*models.AuditLog)(nil)).Where("resource = ?", resource).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}

	var logs []models.AuditLog
	if err := q.Scan(ctx, &logs); err != nil {
		return nil, fmt.Errorf("get audit by resource: %w", err)
	}
	return toAuditEntriesFromModels(logs), nil
}

// GetByActor retrieves audit entries performed by a specific actor.
func (r *AuditLogBunRepository) GetByActor(ctx context.Context, actorID string, limit, offset int) ([]*dto.AuditEntry, error) {
	if r.db == nil {
		return nil, nil
	}

	q := r.db.NewSelect().Model((*models.AuditLog)(nil)).Where("actor_id = ?", actorID).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}

	var logs []models.AuditLog
	if err := q.Scan(ctx, &logs); err != nil {
		return nil, fmt.Errorf("get audit by actor: %w", err)
	}
	return toAuditEntriesFromModels(logs), nil
}

// GetByAction retrieves all entries of a specific action type.
func (r *AuditLogBunRepository) GetByAction(ctx context.Context, action string, limit, offset int) ([]*dto.AuditEntry, error) {
	if r.db == nil {
		return nil, nil
	}

	q := r.db.NewSelect().Model((*models.AuditLog)(nil)).Where("action = ?", action).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}

	var logs []models.AuditLog
	if err := q.Scan(ctx, &logs); err != nil {
		return nil, fmt.Errorf("get audit by action: %w", err)
	}
	return toAuditEntriesFromModels(logs), nil
}

// CleanupOlderThan deletes audit entries older than the given time.
func (r *AuditLogBunRepository) CleanupOlderThan(ctx context.Context, before time.Time) (int, error) {
	if r.db == nil {
		return 0, nil
	}

	res, err := r.db.NewDelete().Model((*models.AuditLog)(nil)).Where("created_at < ?", before).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("cleanup audit logs: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toAuditEntriesFromModels(logs []models.AuditLog) []*dto.AuditEntry {
	entries := make([]*dto.AuditEntry, len(logs))
	for i, l := range logs {
		entries[i] = &dto.AuditEntry{
			ID:        l.ID.String(),
			Action:    l.Action,
			ActorID:   derefString(l.ActorID),
			Resource:  l.Resource,
			Changes:   l.Changes,
			IP:        derefString(l.IP),
			CreatedAt: l.CreatedAt,
		}
	}
	return entries
}
