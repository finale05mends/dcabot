package models

import "time"

type OrderSide string

type OrderType string

type OrderStatus string

type OrderKind string

const (
	OrderSideBuy  OrderSide = "Buy"
	OrderSideSell OrderSide = "Sell"
)

const (
	OrderTypeMarket OrderType = "Market"
	OrderTypeLimit  OrderType = "Limit"
)

const (
	OrderStatusNew             OrderStatus = "New"
	OrderStatusPartiallyFilled OrderStatus = "PartiallyFilled"
	OrderStatusFilled          OrderStatus = "Filled"
	OrderStatusCanceled        OrderStatus = "Canceled"
)

const (
	OrderKindEntry  OrderKind = "Entry"
	OrderKindTP     OrderKind = "TakeProfit"
	OrderKindSafety OrderKind = "Safety"
)

type Order struct {
	ID          string      `json:"id"`
	LinkID      string      `json:"link_id"`
	Symbol      string      `json:"symbol"`
	Side        OrderSide   `json:"side"`
	Type        OrderType   `json:"type"`
	Kind        OrderKind   `json:"kind"`
	Price       float64     `json:"price"`
	Qty         float64     `json:"qty"`
	FilledQty   float64     `json:"filled_qty"`
	Status      OrderStatus `json:"status"`
	Sequence    int64       `json:"sequence"`
	CreateTime  time.Time   `json:"create_time"`
	UpdateTime  time.Time   `json:"update_time"`
	IsReduce    bool        `json:"is_reduce"`
	TimeInForce string      `json:"time_in_force"`
	MarketUnit  string      `json:"market_unit"`
	PriceStep   float64     `json:"price_step"`
	QtyStep     float64     `json:"qty_step"`
}

type Fill struct {
	OrderID   string    `json:"order_id"`
	LinkID    string    `json:"link_id"`
	ExecID    string    `json:"exec_id"`
	Symbol    string    `json:"symbol"`
	Side      OrderSide `json:"side"`
	Price     float64   `json:"price"`
	Qty       float64   `json:"qty"`
	Timestamp time.Time `json:"timestamp"`
	Sequence  int64     `json:"sequence"`
}

type Ticker struct {
	Symbol    string    `json:"symbol"`
	LastPrice float64   `json:"last_price"`
	Timestamp time.Time `json:"timestamp"`
	Sequence  int64     `json:"sequence"`
}
