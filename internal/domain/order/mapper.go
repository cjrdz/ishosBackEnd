// mapper.go — Convert between DTO (dto.Order) and domain (Order) representations.
package order

import (
	"github.com/cjrdz/ishosBackEnd/internal/dto"
)

// FromDTOOrder converts a dto.Order (data transfer object) to a domain Order.
// The domain Order is the business logic representation, independent of the database.
func FromDTOOrder(dtoOrder *dto.Order) (*Order, error) {
	if dtoOrder == nil {
		return nil, nil
	}

	status, err := NewOrderStatus(dtoOrder.Status)
	if err != nil {
		return nil, err
	}

	domainOrder := &Order{
		ID:               dtoOrder.ID,
		OrderNumber:      dtoOrder.OrderNumber,
		Status:           status,
		CustomerName:     dtoOrder.CustomerName,
		CustomerPhone:    dtoOrder.CustomerPhone,
		CustomerEmail:    stringVal(dtoOrder.CustomerEmail),
		PaymentMethod:    dtoOrder.PaymentMethod,
		PaymentStatus:    dtoOrder.PaymentStatus,
		OrderType:        dtoOrder.OrderType,
		TableNumber:      dtoOrder.TableNumber,
		Notes:            stringVal(dtoOrder.Notes),
		RejectionReason:  stringVal(dtoOrder.RejectionReason),
		ReviewedByUserID: stringVal(dtoOrder.ReviewedByUserID),
		ReviewedAt:       dtoOrder.ReviewedAt,
		CreatedByUserID:  stringVal(dtoOrder.CreatedByUserID),
		IsArchived:       dtoOrder.IsArchived,
		CreatedAt:        dtoOrder.CreatedAt,
		UpdatedAt:        dtoOrder.UpdatedAt,
		StatusTimestamps: dtoOrder.StatusTimestamps,
		TotalAmount:      dtoOrder.TotalAmount,
		Items:            []OrderItem{},
	}

	// Convert order items if loaded
	if dtoOrder.Items != nil {
		for _, dtoItem := range dtoOrder.Items {
			item := OrderItem{
				ID:             dtoItem.ID,
				ProductID:      dtoItem.ProductID,
				ProductName:    dtoItem.ProductName,
				Quantity:       dtoItem.Quantity,
				UnitPrice:      dtoItem.UnitPrice,
				Subtotal:       dtoItem.Subtotal,
				Customizations: Customizations(dtoItem.Customizations),
			}
			domainOrder.Items = append(domainOrder.Items, item)
		}
	}

	return domainOrder, nil
}

// stringVal safely converts *string to string, returning "" if nil.
func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
