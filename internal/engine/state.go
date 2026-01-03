package engine

import (
	"dcabot/internal/models"
	"time"
)

type DealState struct {
	Active           bool               `json:"active"`
	Closing          bool               `json:"closing"`
	CloseRequested   bool               `json:"close_requested"`
	CloseReason      string             `json:"close_reason"`
	DealID           string             `json:"deal_id"`
	Symbol           string             `json:"symbol"`
	Side             models.OrderSide   `json:"side"`
	EntryPrice       float64            `json:"entry_price"`
	EntryLinkID      string             `json:"entry_link_id"`
	AvgPrice         float64            `json:"avg_price"`
	TotalQty         float64            `json:"total_qty"`
	FilledByLink     map[string]float64 `json:"filled_by_link"`
	ProcessedExecIDs map[string]bool    `json:"processed_exec_ids"`
	TPFilledQty      float64            `json:"tp_filled_qty"`
	TPOrderID        string             `json:"tp_order_id"`
	TPlinkID         string             `json:"tp_link_id"`
	PlannedTPQty     float64            `json:"planned_tp_qty"`
	SafetyOrders     map[string]string  `json:"safety_orders"`
	LastFillSeq      int64              `json:"last_fill_seq"`
	LastFillAt       time.Time          `json:"last_fill_at"`
	LastOrderSeq     int64              `json:"last_order_seq"`
	LastTickerSeq    int64              `json:"last_ticker_seq"`
	LastTicker       models.Ticker      `json:"last_ticker"`
	UpdatedAt        time.Time          `json:"updated_at"`
	ClosedAt         *time.Time         `json:"closed_at,omitempty"`
	PlannedTPPrice   float64            `json:"planned_tp_price"`
}
