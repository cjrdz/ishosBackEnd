package events

import "sync"

type OrderStatus string

const (
	OrderStatusCreated  OrderStatus = "created"
	OrderStatusApproved OrderStatus = "approved"
	OrderStatusRejected OrderStatus = "rejected"
	OrderStatusReady    OrderStatus = "ready"
)

type OrderStatusChanged struct {
	Status      OrderStatus
	OrderNumber string
	Phone       string
	Total       float64
	Reason      string
	TrackingURL string
}

type Dispatcher struct {
	mu                     sync.RWMutex
	orderStatusSubscribers []func(OrderStatusChanged)
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{}
}

func (d *Dispatcher) SubscribeOrderStatusChanged(fn func(OrderStatusChanged)) {
	if fn == nil {
		return
	}
	d.mu.Lock()
	d.orderStatusSubscribers = append(d.orderStatusSubscribers, fn)
	d.mu.Unlock()
}

func (d *Dispatcher) PublishOrderStatusChanged(evt OrderStatusChanged) {
	d.mu.RLock()
	subscribers := make([]func(OrderStatusChanged), len(d.orderStatusSubscribers))
	copy(subscribers, d.orderStatusSubscribers)
	d.mu.RUnlock()

	for _, fn := range subscribers {
		go fn(evt)
	}
}
