package models

import "time"

type OrderSide string
type OrderType string
type OrderStatus string
type OrderKind string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"

	OrderTypeMarket OrderType = "MARKET"
	OrderTypeLimit  OrderType = "LIMIT"

	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCanceled        OrderStatus = "CANCELED"

	OrderKindEntry  OrderKind = "ENTRY"
	OrderKindTP     OrderKind = "TAKE_PROFIT"
	OrderKindSafety OrderKind = "SAFETY"
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
