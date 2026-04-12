package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/ent"
	entOrder "github.com/cjrdz/ishosBackEnd/ent/order"
	entOrderItem "github.com/cjrdz/ishosBackEnd/ent/orderitem"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrInvalidOrderState  = errors.New("invalid order state")
	ErrProductNotFound    = errors.New("product not found")
	ErrProductUnavailable = errors.New("product unavailable")
	ErrFlavorRequired     = errors.New("flavor selection required")
	ErrFlavorNotAllowed   = errors.New("selected flavor is not available for this product")
	ErrAddonNotAllowed    = errors.New("one or more selected addons are not available for this product")
)

const defaultAddonGroupName = "extras"

var freeAddonAllowanceByGroup = map[string]int{
	"toppings": 1,
	"jalea":    1,
}

type OrderService struct {
	db             *ent.Client
	orderRepo      *repository.OrderRepository
	productRepo    *repository.ProductRepository
	whatsapp       *WhatsAppService
	tracker        *TrackingTokenService
	publicStoreURL string
}

type CreateOrderInput struct {
	CustomerName  string
	CustomerPhone string
	CustomerEmail *string
	CustomerIP    *string
	PaymentMethod string
	OrderType     string
	TableNumber   *int
	Notes         *string
	Items         []OrderItemInput
	CreatedByID   *string
}

type OrderItemInput struct {
	ProductID      uuid.UUID
	Quantity       int
	Customizations map[string]interface{}
}

type UpdateOrderInput struct {
	CustomerName  string
	CustomerPhone string
	CustomerEmail *string
	PaymentMethod string
	OrderType     string
	TableNumber   *int
	Notes         *string
	Items         []OrderItemInput
}

func NewOrderService(db *ent.Client, orderRepo *repository.OrderRepository, productRepo *repository.ProductRepository, whatsapp *WhatsAppService, tracker *TrackingTokenService, publicStoreURL string) *OrderService {
	return &OrderService{
		db:             db,
		orderRepo:      orderRepo,
		productRepo:    productRepo,
		whatsapp:       whatsapp,
		tracker:        tracker,
		publicStoreURL: strings.TrimRight(strings.TrimSpace(publicStoreURL), "/"),
	}
}

func (s *OrderService) IssueTrackingAccess(ctx context.Context, orderNumber string) (*TrackingAccess, error) {
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

func (s *OrderService) buildTrackingURL(ctx context.Context, orderNumber string) string {
	trackingAccess, err := s.IssueTrackingAccess(ctx, orderNumber)
	if err != nil {
		return ""
	}
	return trackingAccess.URL
}

func (s *OrderService) ListPaginated(ctx context.Context, status string, page, limit int) ([]*ent.Order, int, error) {
	return s.orderRepo.ListPaginated(ctx, status, page, limit)
}

func (s *OrderService) GetByIDWithItems(ctx context.Context, id uuid.UUID) (*ent.Order, error) {
	return s.orderRepo.FindByIDWithItems(ctx, id)
}

func (s *OrderService) GetByOrderNumber(ctx context.Context, orderNumber string) (*ent.Order, error) {
	order, err := s.orderRepo.FindByOrderNumber(ctx, orderNumber)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return order, nil
}

func (s *OrderService) GetByOrderNumberAndPhone(ctx context.Context, orderNumber, customerPhone string) (*ent.Order, error) {
	order, err := s.orderRepo.FindByOrderNumberAndPhone(ctx, orderNumber, customerPhone)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return order, nil
}

func (s *OrderService) ListRecentByPhone(ctx context.Context, customerPhone string, limit int) ([]*ent.Order, error) {
	orders, err := s.orderRepo.FindRecentByPhone(ctx, customerPhone, limit)
	if err != nil {
		return nil, err
	}
	return orders, nil
}

func cloneCustomizations(raw map[string]interface{}) map[string]interface{} {
	if raw == nil {
		return nil
	}

	cloned := make(map[string]interface{}, len(raw))
	for key, value := range raw {
		cloned[key] = value
	}

	return cloned
}

func extractCustomizationString(raw map[string]interface{}, key string) string {
	if raw == nil {
		return ""
	}

	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}

	text, ok := value.(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(text)
}

func extractCustomizationStrings(raw map[string]interface{}, key string) []string {
	if raw == nil {
		return nil
	}

	value, ok := raw[key]
	if !ok || value == nil {
		return nil
	}

	switch typed := value.(type) {
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			trimmed := strings.TrimSpace(text)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	default:
		return nil
	}
}

func normalizeAddonGroupName(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return defaultAddonGroupName
	}

	switch normalized {
	case "topping", "toppings":
		return "toppings"
	case "jalea", "jaleas":
		return "jalea"
	case "extra", "extras":
		return "extras"
	}

	return normalized
}

func activeFlavorIndex(product *ent.Product) map[uuid.UUID]*ent.Flavor {
	indexed := make(map[uuid.UUID]*ent.Flavor)
	for _, flavor := range product.Edges.Flavors {
		if flavor == nil || !flavor.IsActive {
			continue
		}
		indexed[flavor.ID] = flavor
	}
	return indexed
}

func activeAddonIndex(product *ent.Product) map[uuid.UUID]*ent.Addon {
	indexed := make(map[uuid.UUID]*ent.Addon)
	for _, addon := range product.Edges.Addons {
		if addon == nil || !addon.IsActive {
			continue
		}
		indexed[addon.ID] = addon
	}
	return indexed
}

// enrichCustomizations validates customizations against the product's assigned,
// active flavors and addons, adds resolved names/prices, and returns the price
// delta introduced by selected paid addons.
func (s *OrderService) enrichCustomizations(product *ent.Product, raw map[string]interface{}) (map[string]interface{}, float64, error) {
	enriched := cloneCustomizations(raw)
	addonExtra := 0.0

	activeFlavors := activeFlavorIndex(product)
	selectedFlavorID := extractCustomizationString(raw, "flavor_id")

	if selectedFlavorID != "" {
		parsedFlavorID, parseErr := uuid.Parse(selectedFlavorID)
		if parseErr != nil {
			return nil, 0, fmt.Errorf("%w: %s", ErrFlavorNotAllowed, selectedFlavorID)
		}

		selectedFlavor, ok := activeFlavors[parsedFlavorID]
		if !ok {
			return nil, 0, fmt.Errorf("%w: %s", ErrFlavorNotAllowed, selectedFlavorID)
		}

		if enriched == nil {
			enriched = make(map[string]interface{})
		}
		enriched["flavor_id"] = selectedFlavor.ID.String()
		enriched["flavor_name"] = selectedFlavor.Name
	}

	activeAddons := activeAddonIndex(product)

	legacyAddonIDs := extractCustomizationStrings(raw, "addon_ids")
	includedAddonIDs := extractCustomizationStrings(raw, "included_addon_ids")
	extraAddonIDs := extractCustomizationStrings(raw, "extra_addon_ids")

	if len(legacyAddonIDs) > 0 || len(includedAddonIDs) > 0 || len(extraAddonIDs) > 0 {
		seenAll := make(map[uuid.UUID]struct{})

		resolveAddon := func(addonID string) (*ent.Addon, error) {
			parsedAddonID, parseErr := uuid.Parse(addonID)
			if parseErr != nil {
				return nil, fmt.Errorf("%w: %s", ErrAddonNotAllowed, addonID)
			}

			selectedAddon, ok := activeAddons[parsedAddonID]
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrAddonNotAllowed, addonID)
			}

			if _, duplicate := seenAll[selectedAddon.ID]; duplicate {
				return nil, nil
			}
			seenAll[selectedAddon.ID] = struct{}{}
			return selectedAddon, nil
		}

		if len(includedAddonIDs) == 0 && len(extraAddonIDs) == 0 && len(legacyAddonIDs) > 0 {
			normalizedAddonIDs := make([]string, 0, len(legacyAddonIDs))
			addonNames := make([]string, 0, len(legacyAddonIDs))
			addonPrices := make([]float64, 0, len(legacyAddonIDs))

			for _, addonID := range legacyAddonIDs {
				selectedAddon, resolveErr := resolveAddon(addonID)
				if resolveErr != nil {
					return nil, 0, resolveErr
				}
				if selectedAddon == nil {
					continue
				}

				normalizedAddonIDs = append(normalizedAddonIDs, selectedAddon.ID.String())
				addonNames = append(addonNames, selectedAddon.Name)
				addonPrices = append(addonPrices, selectedAddon.Price)
				addonExtra += selectedAddon.Price
			}

			if len(normalizedAddonIDs) > 0 {
				if enriched == nil {
					enriched = make(map[string]interface{})
				}
				enriched["addon_ids"] = normalizedAddonIDs
				enriched["addon_names"] = addonNames
				enriched["addon_prices"] = addonPrices
			}
		} else {
			seenAll = make(map[uuid.UUID]struct{})
			includedByGroupCount := make(map[string]int)

			normalizedIncludedIDs := make([]string, 0, len(includedAddonIDs))
			includedNames := make([]string, 0, len(includedAddonIDs))

			normalizedExtraIDs := make([]string, 0, len(extraAddonIDs)+len(legacyAddonIDs))
			extraNames := make([]string, 0, len(extraAddonIDs)+len(legacyAddonIDs))
			extraPrices := make([]float64, 0, len(extraAddonIDs)+len(legacyAddonIDs))

			allSelectedIDs := make([]string, 0, len(includedAddonIDs)+len(extraAddonIDs))
			allSelectedNames := make([]string, 0, len(includedAddonIDs)+len(extraAddonIDs))

			for _, addonID := range includedAddonIDs {
				selectedAddon, resolveErr := resolveAddon(addonID)
				if resolveErr != nil {
					return nil, 0, resolveErr
				}
				if selectedAddon == nil {
					continue
				}

				groupName := normalizeAddonGroupName(selectedAddon.GroupName)
				allowance := freeAddonAllowanceByGroup[groupName]
				if allowance > 0 && includedByGroupCount[groupName] < allowance {
					includedByGroupCount[groupName]++
					normalizedIncludedIDs = append(normalizedIncludedIDs, selectedAddon.ID.String())
					includedNames = append(includedNames, selectedAddon.Name)
					allSelectedIDs = append(allSelectedIDs, selectedAddon.ID.String())
					allSelectedNames = append(allSelectedNames, selectedAddon.Name)
					continue
				}

				normalizedExtraIDs = append(normalizedExtraIDs, selectedAddon.ID.String())
				extraNames = append(extraNames, selectedAddon.Name)
				extraPrices = append(extraPrices, selectedAddon.Price)
				addonExtra += selectedAddon.Price
				allSelectedIDs = append(allSelectedIDs, selectedAddon.ID.String())
				allSelectedNames = append(allSelectedNames, selectedAddon.Name)
			}

			for _, addonID := range extraAddonIDs {
				selectedAddon, resolveErr := resolveAddon(addonID)
				if resolveErr != nil {
					return nil, 0, resolveErr
				}
				if selectedAddon == nil {
					continue
				}

				normalizedExtraIDs = append(normalizedExtraIDs, selectedAddon.ID.String())
				extraNames = append(extraNames, selectedAddon.Name)
				extraPrices = append(extraPrices, selectedAddon.Price)
				addonExtra += selectedAddon.Price
				allSelectedIDs = append(allSelectedIDs, selectedAddon.ID.String())
				allSelectedNames = append(allSelectedNames, selectedAddon.Name)
			}

			if len(allSelectedIDs) > 0 {
				if enriched == nil {
					enriched = make(map[string]interface{})
				}
				enriched["addon_ids"] = allSelectedIDs
				enriched["addon_names"] = allSelectedNames
				enriched["included_addon_ids"] = normalizedIncludedIDs
				enriched["included_addon_names"] = includedNames
				enriched["extra_addon_ids"] = normalizedExtraIDs
				enriched["extra_addon_names"] = extraNames
				enriched["extra_addon_prices"] = extraPrices
				enriched["addon_prices"] = extraPrices
			}
		}
	}

	if enriched != nil {
		if notes, ok := enriched["notes"].(string); ok {
			trimmed := strings.TrimSpace(notes)
			if trimmed == "" {
				delete(enriched, "notes")
			} else {
				enriched["notes"] = trimmed
			}
		}
	}

	if len(enriched) == 0 {
		return nil, addonExtra, nil
	}

	return enriched, addonExtra, nil
}

func (s *OrderService) Create(ctx context.Context, input CreateOrderInput) (*ent.Order, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}

	txProductRepo := repository.NewProductRepository(tx.Client())
	total := 0.0

	type snapshot struct {
		productID      uuid.UUID
		productName    string
		quantity       int
		unitPrice      float64
		customizations map[string]interface{}
	}
	items := make([]snapshot, 0, len(input.Items))

	for _, item := range input.Items {
		p, getErr := txProductRepo.FindByIDWithCategory(ctx, item.ProductID)
		if getErr != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("%w: %s", ErrProductNotFound, item.ProductID)
		}
		if !p.IsAvailable {
			_ = tx.Rollback()
			return nil, fmt.Errorf("%w: %s", ErrProductUnavailable, p.Name)
		}

		enriched, addonExtra, enrichErr := s.enrichCustomizations(p, item.Customizations)
		if enrichErr != nil {
			_ = tx.Rollback()
			return nil, enrichErr
		}
		effectivePrice := p.Price + addonExtra

		total += effectivePrice * float64(item.Quantity)
		items = append(items, snapshot{
			productID:      item.ProductID,
			productName:    p.Name,
			quantity:       item.Quantity,
			unitPrice:      effectivePrice,
			customizations: enriched,
		})
	}

	order, err := tx.Order.Create().
		SetOrderNumber(generateOrderNumber()).
		SetCustomerName(input.CustomerName).
		SetCustomerPhone(input.CustomerPhone).
		SetNillableCustomerEmail(input.CustomerEmail).
		SetNillableCustomerIP(input.CustomerIP).
		SetTotalAmount(total).
		SetStatus(entOrder.StatusPendienteRevision).
		SetPaymentMethod(entOrder.PaymentMethod(input.PaymentMethod)).
		SetOrderType(entOrder.OrderType(input.OrderType)).
		SetNillableTableNumber(input.TableNumber).
		SetNillableNotes(input.Notes).
		SetNillableCreatedByUserID(input.CreatedByID).
		SetCreatedAt(time.Now()).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	for _, item := range items {
		create := tx.OrderItem.Create().
			SetOrderID(order.ID).
			SetProductID(item.productID).
			SetProductName(item.productName).
			SetQuantity(item.quantity).
			SetUnitPrice(item.unitPrice).
			SetSubtotal(item.unitPrice * float64(item.quantity))
		if item.customizations != nil {
			create = create.SetCustomizations(item.customizations)
		}
		if _, itemErr := create.Save(ctx); itemErr != nil {
			_ = tx.Rollback()
			return nil, itemErr
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	go s.whatsapp.SendOrderCreated(input.CustomerPhone, order.OrderNumber, total, s.buildTrackingURL(context.Background(), order.OrderNumber))
	return order, nil
}

func (s *OrderService) Approve(ctx context.Context, id uuid.UUID, reviewerID string, allowOverride bool) (*ent.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	if order.Status != entOrder.StatusPendienteRevision && !(allowOverride && order.Status == entOrder.StatusCancelada) {
		return nil, ErrInvalidOrderState
	}

	updated, err := s.orderRepo.Approve(ctx, id, &reviewerID)
	if err != nil {
		return nil, err
	}

	go s.whatsapp.SendOrderApproved(order.CustomerPhone, order.OrderNumber, s.buildTrackingURL(context.Background(), order.OrderNumber))
	return updated, nil
}

func (s *OrderService) Reject(ctx context.Context, id uuid.UUID, reason, reviewerID string) (*ent.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	updated, err := s.orderRepo.Reject(ctx, id, reason, &reviewerID)
	if err != nil {
		return nil, err
	}

	go s.whatsapp.SendOrderRejected(order.CustomerPhone, order.OrderNumber, reason, s.buildTrackingURL(context.Background(), order.OrderNumber))
	return updated, nil
}

func (s *OrderService) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*ent.Order, error) {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	updated, err := s.orderRepo.UpdateStatus(ctx, id, status)
	if err != nil {
		return nil, err
	}

	if status == "lista" {
		go s.whatsapp.SendOrderReady(order.CustomerPhone, order.OrderNumber, s.buildTrackingURL(context.Background(), order.OrderNumber))
	}

	return updated, nil
}

func (s *OrderService) Update(ctx context.Context, id uuid.UUID, input UpdateOrderInput) (*ent.Order, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}

	order, err := tx.Order.Get(ctx, id)
	if err != nil {
		_ = tx.Rollback()
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	update := tx.Order.UpdateOneID(id).
		SetCustomerName(input.CustomerName).
		SetCustomerPhone(input.CustomerPhone).
		SetPaymentMethod(entOrder.PaymentMethod(input.PaymentMethod)).
		SetOrderType(entOrder.OrderType(input.OrderType)).
		SetNillableCustomerEmail(input.CustomerEmail).
		SetNillableNotes(input.Notes).
		SetUpdatedAt(time.Now())

	if input.TableNumber == nil {
		update = update.ClearTableNumber()
	} else {
		update = update.SetTableNumber(*input.TableNumber)
	}

	if len(input.Items) == 0 {
		if _, err := update.Save(ctx); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return s.orderRepo.FindByIDWithItems(ctx, id)
	}

	txProductRepo := repository.NewProductRepository(tx.Client())
	total := 0.0

	type snapshot struct {
		productID      uuid.UUID
		productName    string
		quantity       int
		unitPrice      float64
		customizations map[string]interface{}
	}
	items := make([]snapshot, 0, len(input.Items))

	for _, item := range input.Items {
		p, getErr := txProductRepo.FindByIDWithCategory(ctx, item.ProductID)
		if getErr != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("%w: %s", ErrProductNotFound, item.ProductID)
		}
		if !p.IsAvailable {
			_ = tx.Rollback()
			return nil, fmt.Errorf("%w: %s", ErrProductUnavailable, p.Name)
		}

		enriched, addonExtra, enrichErr := s.enrichCustomizations(p, item.Customizations)
		if enrichErr != nil {
			_ = tx.Rollback()
			return nil, enrichErr
		}
		effectivePrice := p.Price + addonExtra

		total += effectivePrice * float64(item.Quantity)
		items = append(items, snapshot{
			productID:      item.ProductID,
			productName:    p.Name,
			quantity:       item.Quantity,
			unitPrice:      effectivePrice,
			customizations: enriched,
		})
	}

	if _, err := update.SetTotalAmount(total).Save(ctx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if _, err := tx.OrderItem.Delete().Where(entOrderItem.HasOrderWith(entOrder.IDEQ(order.ID))).Exec(ctx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	for _, item := range items {
		create := tx.OrderItem.Create().
			SetOrderID(order.ID).
			SetProductID(item.productID).
			SetProductName(item.productName).
			SetQuantity(item.quantity).
			SetUnitPrice(item.unitPrice).
			SetSubtotal(item.unitPrice * float64(item.quantity))
		if item.customizations != nil {
			create = create.SetCustomizations(item.customizations)
		}
		if _, itemErr := create.Save(ctx); itemErr != nil {
			_ = tx.Rollback()
			return nil, itemErr
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.orderRepo.FindByIDWithItems(ctx, id)
}

func (s *OrderService) UpdateNotes(ctx context.Context, id uuid.UUID, notes *string) (*ent.Order, error) {
	if _, err := s.orderRepo.FindByID(ctx, id); err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	return s.orderRepo.UpdateNotes(ctx, id, notes)
}

func (s *OrderService) Delete(ctx context.Context, id uuid.UUID) error {
	order, err := s.orderRepo.FindByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrOrderNotFound
		}
		return err
	}
	if s.tracker != nil {
		if revokeErr := s.tracker.RevokeAllByOrder(ctx, order.OrderNumber); revokeErr != nil {
			return revokeErr
		}
	}

	return s.orderRepo.Delete(ctx, id)
}

func generateOrderNumber() string {
	randomNum, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		// Fallback: use sequential number based on time
		randomNum = big.NewInt(int64(time.Now().Nanosecond() % 9000))
	}
	return fmt.Sprintf("ORD-%s-%04d", time.Now().Format("20060102"), randomNum.Int64()+1000)
}
