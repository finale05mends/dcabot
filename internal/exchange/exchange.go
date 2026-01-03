package exchange

import (
	"context"
	"dcabot/internal/models"
)

type EventType string

const (
	EventTypeOrder     EventType = "Order"
	EventTypeFill      EventType = "Fill"
	EventTypeTicker    EventType = "Ticker"
	EventTypeReconnect EventType = "Reconnect"
)

type Event struct {
	Type   EventType
	Order  *models.Order
	Fill   *models.Fill
	Ticker *models.Ticker
}

type InstrumentRules struct {
	TickSize    float64
	LotSize     float64
	MinQty      float64
	MinNotional float64
	BaseCoin    string
	QuoteCoin   string
}

type Client interface {
	GetInstrumentRules(ctx context.Context, symbol string) (InstrumentRules, error)
	Subscribe(ctx context.Context, symbol string) (<-chan Event, error)
	CancelOrder(ctx context.Context, symbol, orderID string) error
	GetOpenOrders(ctx context.Context, symbol string) ([]models.Order, error)
	PlaceOrder(ctx context.Context, order models.Order) (models.Order, error)
	GetFills(ctx context.Context, symbol string) ([]models.Fill, error)
	GetBalances(ctx context.Context, coins []string) (map[string]Balance, error)
}

type Balance struct {
	Coin      string
	Wallet    float64
	Available float64
}
