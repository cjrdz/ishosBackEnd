package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cjrdz/ishosBackEnd/internal/apperror"
	"github.com/cjrdz/ishosBackEnd/internal/dto"
	"github.com/cjrdz/ishosBackEnd/internal/repository"
)

func TestActiveOfferPrices(t *testing.T) {
	cone := uuid.New()
	bowl := uuid.New()
	expired := uuid.New()
	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	past := time.Now().Add(-time.Hour).Format(time.RFC3339)

	repo := &adminSettingRepoStub{getByKey: func(_ context.Context, key string) (dto.AdminSetting, error) {
		if key != StoreOffersKey {
			return dto.AdminSetting{Value: map[string]any{}}, nil
		}
		return dto.AdminSetting{Value: map[string]any{"items": []any{
			map[string]any{"product_id": cone.String(), "discount_price": 0.75, "expires_at": future},
			map[string]any{"product_id": cone.String(), "discount_price": 0.60, "expires_at": future}, // lowest wins
			map[string]any{"product_id": bowl.String(), "discount_price": 0.0, "expires_at": future},  // label-only offer
			map[string]any{"product_id": expired.String(), "discount_price": 0.50, "expires_at": past},
			map[string]any{"product_id": "not-a-uuid", "discount_price": 1.0, "expires_at": future},
			map[string]any{"product_id": bowl.String(), "discount_price": 0.40}, // no expiry: storefront hides it
		}}}, nil
	}}

	prices, err := NewAdminSettingService(repo).ActiveOfferPrices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(prices) != 1 || prices[cone] != 0.60 {
		t.Fatalf("prices = %v, want only %s → 0.60", prices, cone)
	}
}

func TestActiveOffersFiltersExpired(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	offers := []OfferItem{
		{ProductID: "a", ExpiresAt: now.Add(time.Minute).Format(time.RFC3339)},
		{ProductID: "b", ExpiresAt: now.Add(-time.Minute).Format(time.RFC3339)},
		{ProductID: "c"}, // no expiry → active
	}
	active := ActiveOffers(offers, now)
	if len(active) != 2 || active[0].ProductID != "a" || active[1].ProductID != "c" {
		t.Fatalf("active = %+v", active)
	}
}

func TestUpdateStoreSettingsRejectsUnpriceableOffers(t *testing.T) {
	svc := NewAdminSettingService(&adminSettingRepoStub{})
	good := OfferItem{ProductID: uuid.NewString(), DiscountPrice: 1.5, ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}

	if _, err := svc.UpdateStoreSettings(context.Background(), StoreSettings{OrdersEnabled: true, Offers: []OfferItem{good}}, "admin"); err != nil {
		t.Fatalf("valid offer rejected: %v", err)
	}

	for name, bad := range map[string]OfferItem{
		"local datetime without zone": {ProductID: good.ProductID, DiscountPrice: 1, ExpiresAt: "2026-11-01T10:00"},
		"missing expiry":              {ProductID: good.ProductID, DiscountPrice: 1},
		"negative price":              {ProductID: good.ProductID, DiscountPrice: -1, ExpiresAt: good.ExpiresAt},
		"not a product id":            {ProductID: "cono", DiscountPrice: 1, ExpiresAt: good.ExpiresAt},
	} {
		_, err := svc.UpdateStoreSettings(context.Background(), StoreSettings{OrdersEnabled: true, Offers: []OfferItem{bad}}, "admin")
		if !errors.Is(err, apperror.ErrValidation) {
			t.Errorf("%s: expected validation error, got %v", name, err)
		}
	}
}

type offerSourceStub struct {
	prices map[uuid.UUID]float64
	err    error
}

func (s offerSourceStub) ActiveOfferPrices(context.Context) (map[uuid.UUID]float64, error) {
	return s.prices, s.err
}

func TestOrderCommandService_Create_PassesOfferPricesToStore(t *testing.T) {
	product := uuid.New()
	var got map[uuid.UUID]float64
	store := &orderStoreStub{createFn: func(_ context.Context, in repository.StoreOrderInput) (dto.Order, error) {
		got = in.BasePrices
		return dto.Order{OrderNumber: "ORD-1"}, nil
	}}
	svc := NewOrderCommandService(&orderRepoStub{}, store, &customerRepoStub{}, &productRepoStub{}, nil, nil, nil)
	svc.offers = offerSourceStub{prices: map[uuid.UUID]float64{product: 1.99}}

	_, err := svc.Create(context.Background(), CreateOrderInput{
		LocationID:   testLocation,
		CustomerName: "Ana", CustomerPhone: "71234567", PaymentMethod: "efectivo", OrderType: "para_llevar",
		Items: []OrderItemInput{{ProductID: product, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got[product] != 1.99 {
		t.Fatalf("store got base prices %v, want %s → 1.99", got, product)
	}
}

func TestOrderCommandService_Create_OfferOutageFallsBackToRegularPrices(t *testing.T) {
	called := false
	store := &orderStoreStub{createFn: func(_ context.Context, in repository.StoreOrderInput) (dto.Order, error) {
		called = true
		if len(in.BasePrices) != 0 {
			t.Errorf("expected no offer prices during an outage, got %v", in.BasePrices)
		}
		return dto.Order{OrderNumber: "ORD-2"}, nil
	}}
	svc := NewOrderCommandService(&orderRepoStub{}, store, &customerRepoStub{}, &productRepoStub{}, nil, nil, nil)
	svc.offers = offerSourceStub{err: errors.New("settings unavailable")}

	if _, err := svc.Create(context.Background(), CreateOrderInput{
		LocationID:   testLocation,
		CustomerName: "Ana", CustomerPhone: "71234567", PaymentMethod: "efectivo", OrderType: "para_llevar",
		Items: []OrderItemInput{{ProductID: uuid.New(), Quantity: 1}},
	}); err != nil {
		t.Fatalf("an offers outage must not block orders: %v", err)
	}
	if !called {
		t.Fatal("order was not created")
	}
}
