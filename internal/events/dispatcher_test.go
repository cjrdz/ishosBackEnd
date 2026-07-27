package events

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestDispatcherPublishesEvent(t *testing.T) {
	d := NewDispatcherWithConfig(DispatcherConfig{QueueSize: 8, Workers: 1})
	t.Cleanup(func() {
		_ = d.Close(context.Background())
	})

	got := make(chan OrderStatusChanged, 1)
	d.SubscribeOrderStatusChanged(func(evt OrderStatusChanged) {
		got <- evt
	})

	want := OrderStatusChanged{Status: OrderStatusCreated, OrderNumber: "A-1"}
	d.PublishOrderStatusChanged(want)

	select {
	case evt := <-got:
		if evt.OrderNumber != want.OrderNumber {
			t.Fatalf("OrderNumber = %q, want %q", evt.OrderNumber, want.OrderNumber)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dispatched event")
	}
}

func TestDispatcherCloseDrainsQueue(t *testing.T) {
	d := NewDispatcherWithConfig(DispatcherConfig{QueueSize: 16, Workers: 2})

	var mu sync.Mutex
	count := 0
	d.SubscribeOrderStatusChanged(func(_ OrderStatusChanged) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	for i := 0; i < 10; i++ {
		d.PublishOrderStatusChanged(OrderStatusChanged{Status: OrderStatusReady})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := d.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if count != 10 {
		t.Fatalf("processed events = %d, want %d", count, 10)
	}
}
