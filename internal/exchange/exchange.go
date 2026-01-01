package exchange

import (
	"context"
	"dcabot/internal/models"
)

type EventType string

const (
	EventTypeOrder     EventType = "ORDER"
	EventTypeFill      EventType = "FILL"
	EventTypeTicker    EventType = "TICKER"
	EventTypeReconnect EventType = "RECONNECT"
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
}

type Client interface {
	GetInstrumentRules(ctx context.Context, symbol string) (InstrumentRules, error)
}
