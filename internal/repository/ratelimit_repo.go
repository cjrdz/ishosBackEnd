// ratelimit_repo.go — Ent-backed repository for persisted per-IP and per-phone rate-limit records.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	"github.com/cjrdz/ishosBackEnd/ent/ratelimit"
)

type RateLimitRepository struct {
	db *ent.Client
}

func NewRateLimitRepository(db *ent.Client) *RateLimitRepository {
	return &RateLimitRepository{db: db}
}

func (r *RateLimitRepository) FindByIPAndAction(ctx context.Context, ip, action string) (*ent.RateLimit, error) {
	return r.db.RateLimit.Query().
		Where(
			ratelimit.IPAddressEQ(ip),
			ratelimit.ActionTypeEQ(action),
		).
		Only(ctx)
}

func (r *RateLimitRepository) FindByPhoneAndAction(ctx context.Context, phone, action string) (*ent.RateLimit, error) {
	return r.db.RateLimit.Query().
		Where(
			ratelimit.PhoneNumberEQ(phone),
			ratelimit.ActionTypeEQ(action),
		).
		Only(ctx)
}

func (r *RateLimitRepository) Create(ctx context.Context, input CreateRateLimitInput) (*ent.RateLimit, error) {
	create := r.db.RateLimit.Create().
		SetIPAddress(input.IPAddress).
		SetActionType(input.ActionType).
		SetAttempts(input.Attempts).
		SetLastAttempt(input.LastAttempt)

	if input.PhoneNumber != nil {
		create = create.SetPhoneNumber(*input.PhoneNumber)
	}
	if input.BlockedUntil != nil {
		create = create.SetBlockedUntil(*input.BlockedUntil)
	}

	return create.Save(ctx)
}

func (r *RateLimitRepository) Update(ctx context.Context, id uuid.UUID, input UpdateRateLimitInput) (*ent.RateLimit, error) {
	upd := r.db.RateLimit.UpdateOneID(id)

	if input.Attempts != nil {
		upd = upd.SetAttempts(*input.Attempts)
	}
	if input.LastAttempt != nil {
		upd = upd.SetLastAttempt(*input.LastAttempt)
	}
	if input.BlockedUntil != nil {
		upd = upd.SetBlockedUntil(*input.BlockedUntil)
	}

	return upd.Save(ctx)
}

func (r *RateLimitRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.RateLimit.DeleteOneID(id).Exec(ctx)
}

type CreateRateLimitInput struct {
	IPAddress    string
	PhoneNumber  *string
	ActionType   string
	Attempts     int
	LastAttempt  time.Time
	BlockedUntil *time.Time
}

type UpdateRateLimitInput struct {
	Attempts     *int
	LastAttempt  *time.Time
	BlockedUntil *time.Time
}
