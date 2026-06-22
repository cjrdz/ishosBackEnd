// middleware.go — Audit logging middleware that captures domain events.
// Integrates with the audit trail for compliance and debugging.
package audit

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	orderDomain "github.com/cjrdz/ishosBackEnd/internal/domain/order"
)

// LogMiddleware captures domain events and writes them to the audit log.
// Should be called from order_command_service.go after each domain event is published.
type LogMiddleware struct {
	svc Service
}

// NewLogMiddleware creates a new audit log middleware.
func NewLogMiddleware(svc Service) *LogMiddleware {
	return &LogMiddleware{svc: svc}
}

// OnDomainEvent processes a domain event and writes to audit log.
// Called after domain operations (Create, Approve, Reject, UpdateStatus, etc).
func (m *LogMiddleware) OnDomainEvent(ctx context.Context, event orderDomain.DomainEvent) error {
	if m.svc == nil || event == nil {
		return nil
	}

	// Map event to action, resource, and changes
	action, resource, changes := m.extractEventDetails(event)
	if action == "" {
		return nil
	}

	// Get IP from context (injected by handlers)
	ip := IPFromContext(ctx)

	// Actor ID is extracted from the context by the caller (order_command_service)
	// For now, we pass empty actorID — the service layer will enrich it.
	// Actually, the middleware should not know about auth context.
	// The publishDomainEvent in command service will call Log directly with actor info.
	// So this middleware is kept for backward compat and direct event-to-audit mapping.
	// In the new flow, command service calls auditSvc.Log directly with full context.
	return m.svc.Log(ctx, action, "", resource, changes, ip)
}

// extractEventDetails converts a domain event to action, resource, and changes details.
func (m *LogMiddleware) extractEventDetails(event orderDomain.DomainEvent) (action, resource string, changes map[string]any) {
	switch e := event.(type) {
	case *orderDomain.OrderCreated:
		action = ActionOrderCreated
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = map[string]any{
			"order_number":  e.OrderNumber,
			"customer_name": e.CustomerName,
			"payment":       e.PaymentMethod,
			"items":         e.ItemCount,
			"total":         e.TotalAmount,
		}

	case *orderDomain.OrderApproved:
		action = ActionOrderApproved
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = map[string]any{
			"order_number": e.OrderNumber,
			"reviewer_id":  e.ReviewerID,
		}

	case *orderDomain.OrderRejected:
		action = ActionOrderRejected
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = map[string]any{
			"order_number": e.OrderNumber,
			"reviewer_id":  e.ReviewerID,
			"reason":       e.Reason,
		}

	case *orderDomain.OrderReady:
		action = ActionOrderReady
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = map[string]any{
			"order_number": e.OrderNumber,
		}

	case *orderDomain.OrderUpdated:
		action = ActionOrderUpdated
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = e.Changes

	case *orderDomain.OrderDeleted:
		action = ActionOrderDeleted
		resource = fmt.Sprintf("order:%s", e.AggregateID().String())
		changes = map[string]any{
			"order_number": e.OrderNumber,
		}

	default:
		log.Warn().Str("event_type", event.EventType()).Msg("Unknown domain event type in audit")
	}

	return action, resource, changes
}
