// service.go — Ent-based implementation of the audit.Service interface.
package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/auditlog"
)

// entService implements audit.Service using Ent ORM.
type entService struct {
	client *ent.Client
}

// NewService creates a new Ent-backed audit service.
func NewService(client *ent.Client) Service {
	return &entService{client: client}
}

// Log creates an audit entry for a sensitive action.
func (s *entService) Log(ctx context.Context, action, actorID, resource string, changes map[string]any, ip string) error {
	if s.client == nil {
		return nil
	}

	b := s.client.AuditLog.Create().
		SetAction(action).
		SetResource(resource)

	if actorID != "" {
		b = b.SetActorID(actorID)
	}
	if ip != "" {
		b = b.SetIP(ip)
	}
	if len(changes) > 0 {
		b = b.SetChanges(changes)
	}

	_, err := b.Save(ctx)
	if err != nil {
		return fmt.Errorf("audit log: %w", err)
	}
	return nil
}

// GetByResource retrieves audit entries for a specific resource.
func (s *entService) GetByResource(ctx context.Context, resource string, limit, offset int) ([]*AuditEntry, error) {
	if s.client == nil {
		return nil, nil
	}

	q := s.client.AuditLog.Query().
		Where(auditlog.ResourceEQ(resource)).
		Order(ent.Desc(auditlog.FieldCreatedAt))

	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}

	logs, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get audit by resource: %w", err)
	}

	return toEntries(logs), nil
}

// GetByActor retrieves audit entries performed by a specific actor.
func (s *entService) GetByActor(ctx context.Context, actorID string, limit, offset int) ([]*AuditEntry, error) {
	if s.client == nil {
		return nil, nil
	}

	q := s.client.AuditLog.Query().
		Where(auditlog.ActorIDEQ(actorID)).
		Order(ent.Desc(auditlog.FieldCreatedAt))

	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}

	logs, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get audit by actor: %w", err)
	}

	return toEntries(logs), nil
}

// GetByAction retrieves all entries of a specific action type.
func (s *entService) GetByAction(ctx context.Context, action string, limit, offset int) ([]*AuditEntry, error) {
	if s.client == nil {
		return nil, nil
	}

	q := s.client.AuditLog.Query().
		Where(auditlog.ActionEQ(action)).
		Order(ent.Desc(auditlog.FieldCreatedAt))

	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}

	logs, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get audit by action: %w", err)
	}

	return toEntries(logs), nil
}

// CleanupOlderThan deletes audit entries older than the given time.
func (s *entService) CleanupOlderThan(ctx context.Context, before time.Time) (int, error) {
	if s.client == nil {
		return 0, nil
	}

	n, err := s.client.AuditLog.Delete().
		Where(auditlog.CreatedAtLT(before)).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("cleanup audit logs: %w", err)
	}
	return n, nil
}

// toEntries converts Ent AuditLog entities to domain AuditEntry structs.
func toEntries(logs []*ent.AuditLog) []*AuditEntry {
	entries := make([]*AuditEntry, len(logs))
	for i, l := range logs {
		entries[i] = &AuditEntry{
			ID:        l.ID.String(),
			Action:    l.Action,
			ActorID:   deref(l.ActorID),
			Resource:  l.Resource,
			Changes:   l.Changes,
			IP:        deref(l.IP),
			CreatedAt: l.CreatedAt,
		}
	}
	return entries
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
