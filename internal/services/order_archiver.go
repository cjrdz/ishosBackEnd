// order_archiver.go — Background/archiving operations for terminal orders.
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/models"
)

// OrderArchiver runs automatic archiving of old terminal orders.
type OrderArchiver struct {
	repo       OrderRepository
	settings   *AdminSettingService
	defaults   ArchiveConfig
	batchSize  int
	terminal   []string
}

// NewOrderArchiver creates an archiver using the supplied repository and
// settings service. defaults provides the env-based fallback values.
func NewOrderArchiver(repo OrderRepository, settings *AdminSettingService, defaults ArchiveConfig) *OrderArchiver {
	return &OrderArchiver{
		repo:      repo,
		settings:  settings,
		defaults:  defaults,
		batchSize: 500,
		terminal: []string{
			string(models.OrderStatusEntregada),
			string(models.OrderStatusCancelada),
		},
	}
}

// RunOnce evaluates archive eligibility and archives one batch of orders.
// It returns the number of orders archived in this run.
func (a *OrderArchiver) RunOnce(ctx context.Context) (int, error) {
	cfg, err := a.settings.GetArchiveSettings(ctx, a.defaults)
	if err != nil {
		return 0, fmt.Errorf("load archive settings: %w", err)
	}
	if !cfg.Enabled {
		return 0, nil
	}

	cutoff := time.Now().AddDate(0, 0, -cfg.AgeDays)
	candidates, err := a.repo.FindEligibleForArchive(ctx, a.terminal, cutoff, a.batchSize)
	if err != nil {
		return 0, fmt.Errorf("find eligible orders: %w", err)
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	ids := make([]uuid.UUID, len(candidates))
	for i, o := range candidates {
		ids[i] = o.ID
	}

	archived, err := a.repo.ArchiveOrders(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("archive orders: %w", err)
	}

	log.Info().Int("archived", archived).Time("cutoff", cutoff).Msg("Auto-archived terminal orders")
	return archived, nil
}

// Start runs the archiver in a background goroutine until ctx is cancelled.
// It reads settings each interval so admin changes take effect without a restart.
func (a *OrderArchiver) Start(ctx context.Context) {
	if a.repo == nil || a.settings == nil {
		return
	}

	cfg, err := a.settings.GetArchiveSettings(ctx, a.defaults)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load initial archive settings; using defaults")
		cfg = a.defaults
	}

	ticker := time.NewTicker(time.Duration(cfg.IntervalMinutes) * time.Minute)
	defer ticker.Stop()

	// Run shortly after startup to catch any backlog.
	if _, runErr := a.RunOnce(ctx); runErr != nil {
		log.Warn().Err(runErr).Msg("Initial order archive run failed")
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Refresh interval in case settings changed.
			fresh, err := a.settings.GetArchiveSettings(ctx, a.defaults)
			if err == nil && fresh.IntervalMinutes > 0 && fresh.IntervalMinutes != cfg.IntervalMinutes {
				cfg.IntervalMinutes = fresh.IntervalMinutes
				ticker.Reset(time.Duration(cfg.IntervalMinutes) * time.Minute)
			}

			if _, runErr := a.RunOnce(ctx); runErr != nil {
				log.Warn().Err(runErr).Msg("Periodic order archive run failed")
			}
		}
	}
}
