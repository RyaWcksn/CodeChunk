//go:build ignore
// example complex Go file for chunker demos
package order

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusPaid      Status = "paid"
	StatusShipped   Status = "shipped"
	StatusCancelled Status = "cancelled"
)

var (
	ErrNotFound     = errors.New("order not found")
	ErrInvalidState = errors.New("invalid state transition")
	ErrPayment      = errors.New("payment failed")
)

const maxRetries = 3
const DefaultTimeout = 30 * time.Second

type Money struct {
	Amount   int64
	Currency string
}

func (m Money) String() string {
	return fmt.Sprintf("%s %d.%02d", m.Currency, m.Amount/100, m.Amount%100)
}

type Item struct {
	SKU      string
	Qty      int
	UnitCost Money
}

type Line struct {
	Item     Item
	Subtotal Money
}

type Address struct {
	Street  string
	City    string
	Country string
	Zip     string
}

type Customer struct {
	ID      string
	Name    string
	Email   string
	Address Address
}

type Order struct {
	ID         string
	Customer   Customer
	Lines      []Line
	Total      Money
	Status     Status
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Version    int64
	privateTag string
}

type Repository interface {
	Save(ctx context.Context, o *Order) error
	Get(ctx context.Context, id string) (*Order, error)
	List(ctx context.Context, c Customer) ([]*Order, error)
	Delete(ctx context.Context, id string) error
}

type PaymentGateway interface {
	Charge(ctx context.Context, m Money, token string) error
	Refund(ctx context.Context, m Money, token string) error
}

type EventBus interface {
	Publish(ctx context.Context, topic string, payload any) error
	Subscribe(ctx context.Context, topic string, handler func(any)) error
}

type Service struct {
	repo  Repository
	pay   PaymentGateway
	bus   EventBus
	mu    sync.RWMutex
	cache map[string]*Order
}

func NewService(r Repository, p PaymentGateway, b EventBus) *Service {
	return &Service{repo: r, pay: p, bus: b, cache: make(map[string]*Order)}
}

func (s *Service) Place(ctx context.Context, cust Customer, items []Item, token string) (*Order, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: no items", ErrInvalidState)
	}
	lines := make([]Line, len(items))
	var total int64
	for i, it := range items {
		sub := it.UnitCost.Amount * int64(it.Qty)
		lines[i] = Line{Item: it, Subtotal: Money{Amount: sub, Currency: it.UnitCost.Currency}}
		total += sub
	}
	money := Money{Amount: total, Currency: items[0].UnitCost.Currency}
	if err := s.pay.Charge(ctx, money, token); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPayment, err)
	}
	o := &Order{
		ID:        newID(),
		Customer:  cust,
		Lines:     lines,
		Total:     money,
		Status:    StatusPaid,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	if err := s.repo.Save(ctx, o); err != nil {
		return nil, err
	}
	_ = s.bus.Publish(ctx, "order.placed", o)
	return o, nil
}

func (s *Service) Cancel(ctx context.Context, id, reason string) error {
	o, err := s.get(ctx, id)
	if err != nil {
		return err
	}
	switch o.Status {
	case StatusPaid, StatusShipped:
		// ok
	default:
		return fmt.Errorf("%w: cannot cancel %s", ErrInvalidState, o.Status)
	}
	o.Status = StatusCancelled
	o.UpdatedAt = time.Now()
	o.Version++
	if err := s.repo.Save(ctx, o); err != nil {
		return err
	}
	_ = s.bus.Publish(ctx, "order.cancelled", map[string]string{"id": id, "reason": reason})
	return nil
}

func (s *Service) get(ctx context.Context, id string) (*Order, error) {
	s.mu.RLock()
	if o, ok := s.cache[id]; ok {
		s.mu.RUnlock()
		return o, nil
	}
	s.mu.RUnlock()
	o, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	s.cache[id] = o
	s.mu.Unlock()
	return o, nil
}

func (s *Service) BulkImport(ctx context.Context, orders []*Order) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, o := range orders {
		o := o
		g.Go(func() error {
			return s.repo.Save(gctx, o)
		})
	}
	return g.Wait()
}

func ComputeTotals[T Numberish](items []T, currency string) []Money {
	out := make([]Money, len(items))
	for i, it := range items {
		out[i] = Money{Amount: int64(it), Currency: currency}
	}
	return out
}

type Numberish interface {
	~int | ~int32 | ~int64 | ~float32 | ~float64
}

func newID() string {
	return fmt.Sprintf("ord_%d", time.Now().UnixNano())
}
