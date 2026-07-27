// order_tracking_service.go — Public order tracking via token-based access without authentication.
package services

import (
	"context"
	"strings"
	"time"
)

// OrderTrackingService provides public order tracking with token-based access.
type OrderTrackingService struct {
	tracker        *TrackingTokenService
	orderRepo      OrderRepository
	publicStoreURL string
}

// NewOrderTrackingService creates a new tracking service.
func NewOrderTrackingService(tracker *TrackingTokenService, orderRepo OrderRepository, publicStoreURL string) *OrderTrackingService {
	return &OrderTrackingService{
		tracker:        tracker,
		orderRepo:      orderRepo,
		publicStoreURL: strings.TrimRight(strings.TrimSpace(publicStoreURL), "/"),
	}
}

// IssueTrackingAccess issues a tracking token for an order.
// Returns a TrackingAccess with URL and token expiration.
func (s *OrderTrackingService) IssueTrackingAccess(ctx context.Context, orderNumber string) (*TrackingAccess, error) {
	if s.tracker == nil {
		return nil, ErrInvalidOrderState
	}

	token, expiresAt, err := s.tracker.Issue(ctx, orderNumber, time.Now())
	if err != nil {
		return nil, err
	}

	return &TrackingAccess{
		Token:     token,
		ExpiresAt: expiresAt,
		URL:       s.tracker.BuildTrackingURL(s.publicStoreURL, orderNumber, token),
	}, nil
}
