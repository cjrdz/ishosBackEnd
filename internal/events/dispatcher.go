// dispatcher.go — In-process pub/sub for order lifecycle events.
// Events are queued into a bounded channel and processed by a worker pool to
// avoid unbounded goroutine growth under burst traffic.
package events

import (
	"context"
	"sync"
)

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

type DispatcherConfig struct {
	QueueSize int
	Workers   int
}

type Dispatcher struct {
	mu                     sync.RWMutex
	orderStatusSubscribers []func(OrderStatusChanged)
	queue                  chan OrderStatusChanged
	once                   sync.Once
	wg                     sync.WaitGroup
}

func NewDispatcher() *Dispatcher {
	return NewDispatcherWithConfig(DispatcherConfig{QueueSize: 256, Workers: 4})
}

func NewDispatcherWithConfig(cfg DispatcherConfig) *Dispatcher {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 256
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}

	d := &Dispatcher{queue: make(chan OrderStatusChanged, cfg.QueueSize)}
	for i := 0; i < cfg.Workers; i++ {
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			for evt := range d.queue {
				d.dispatch(evt)
			}
		}()
	}

	return d
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
	if d == nil || d.queue == nil {
		return
	}

	// Never block request paths: if queue is full, dispatch in a short-lived goroutine.
	select {
	case d.queue <- evt:
	default:
		go d.dispatch(evt)
	}
}

func (d *Dispatcher) Close(ctx context.Context) error {
	if d == nil || d.queue == nil {
		return nil
	}

	d.once.Do(func() {
		close(d.queue)
	})

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	if ctx == nil {
		<-done
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *Dispatcher) dispatch(evt OrderStatusChanged) {
	d.mu.RLock()
	// Snapshot the slice under RLock so subscribers added during dispatch don't receive this event
	// and we don't hold the lock while executing (potentially slow) callbacks.
	subscribers := make([]func(OrderStatusChanged), len(d.orderStatusSubscribers))
	copy(subscribers, d.orderStatusSubscribers)
	d.mu.RUnlock()

	for _, fn := range subscribers {
		fn(evt)
	}
}
