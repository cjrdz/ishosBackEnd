// service.go — Audit service implementation.
// Persistence is delegated to a LogRepository so the service does not depend on a concrete ORM.
package audit

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// serviceImpl implements audit.Service by delegating to a LogRepository.
type serviceImpl struct {
	repo LogRepository
}

// NewService creates a new audit service backed by the supplied repository.
func NewService(repo LogRepository) Service {
	return &serviceImpl{repo: repo}
}

// Log creates an audit entry for a sensitive action.
func (s *serviceImpl) Log(ctx context.Context, action, actorID, resource string, changes map[string]any, ip string) error {
	return s.repo.Log(ctx, action, actorID, resource, changes, ip)
}

// LogWithDB creates an audit entry inside the provided bun transaction or DB.
func (s *serviceImpl) LogWithDB(ctx context.Context, db bun.IDB, action, actorID, resource string, changes map[string]any, ip string) error {
	return s.repo.LogWithDB(ctx, db, action, actorID, resource, changes, ip)
}

// GetByResource retrieves audit entries for a specific resource.
func (s *serviceImpl) GetByResource(ctx context.Context, resource string, limit, offset int) ([]*AuditEntry, error) {
	return s.repo.GetByResource(ctx, resource, limit, offset)
}

// GetByActor retrieves audit entries performed by a specific actor.
func (s *serviceImpl) GetByActor(ctx context.Context, actorID string, limit, offset int) ([]*AuditEntry, error) {
	return s.repo.GetByActor(ctx, actorID, limit, offset)
}

// GetByAction retrieves all entries of a specific action type.
func (s *serviceImpl) GetByAction(ctx context.Context, action string, limit, offset int) ([]*AuditEntry, error) {
	return s.repo.GetByAction(ctx, action, limit, offset)
}

// CleanupOlderThan deletes audit entries older than the given time.
func (s *serviceImpl) CleanupOlderThan(ctx context.Context, before time.Time) (int, error) {
	return s.repo.CleanupOlderThan(ctx, before)
}
