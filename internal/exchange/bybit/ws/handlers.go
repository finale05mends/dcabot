package ws

import (
	"dcabot/internal/exchange"
	"dcabot/internal/models"
	"encoding/json"
	"strconv"
	"time"
)

func (w *Client) handleExecution(msg Message) {
	var data []struct {
		OrderID   string `json:"orderId"`
		OrderLink string `json:"orderLinkId"`
		ExecID    string `json:"execId"`
		Symbol    string `json:"symbol"`
		Side      string `json:"side"`
		ExecPrice string `json:"execPrice"`
		ExecQty   string `json:"execQty"`
		ExecTime  string `json:"execTime"`
		Seq       int64  `json:"seq"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		w.logEntry().WithError(err).Warn("Не удалось разобрать execution.")
		return
	}

	for _, item := range data {
		w.logEntry().WithFields(map[string]interface{}{
			"symbol":        item.Symbol,
			"side":          item.Side,
			"exec_id":       item.ExecID,
			"order_id":      item.OrderID,
			"order_link_id": item.OrderLink,
			"price":         item.ExecPrice,
			"qty":           item.ExecQty,
			"ts":            item.ExecTime,
			"seq":           item.Seq,
		}).Debug("execution")

		price, _ := strconv.ParseFloat(item.ExecPrice, 64)
		qty, _ := strconv.ParseFloat(item.ExecQty, 64)
		tsMs, _ := strconv.ParseInt(item.ExecTime, 10, 64)

		w.events <- exchange.Event{
			Type: exchange.EventTypeFill,
			Fill: &models.Fill{
				OrderID:   item.OrderID,
				LinkID:    item.OrderLink,
				ExecID:    item.ExecID,
				Symbol:    item.Symbol,
				Side:      models.OrderSide(item.Side),
				Price:     price,
				Qty:       qty,
				Timestamp: time.UnixMilli(tsMs),
				Sequence:  item.Seq,
			},
		}
	}
}

func (w *Client) handleOrder(msg Message) {
	var data []struct {
		OrderID      string `json:"orderId"`
		OrderLink    string `json:"orderLinkId"`
		Symbol       string `json:"symbol"`
		Side         string `json:"side"`
		OrderType    string `json:"orderType"`
		Price        string `json:"price"`
		Qty          string `json:"qty"`
		LeavesQty    string `json:"leavesQty"`
		OrderStatus  string `json:"orderStatus"`
		CancelType   string `json:"cancelType"`
		RejectReason string `json:"rejectReason"`
		Seq          int64  `json:"seq"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		w.logEntry().WithError(err).Warn("Не удалось разобрать order.")
		return
	}

	for _, item := range data {
		w.logEntry().WithFields(map[string]interface{}{
			"symbol":        item.Symbol,
			"side":          item.Side,
			"order_id":      item.OrderID,
			"order_link_id": item.OrderLink,
			"type":          item.OrderType,
			"status":        item.OrderStatus,
			"cancel_type":   item.CancelType,
			"reject_reason": item.RejectReason,
			"price":         item.Price,
			"qty":           item.Qty,
			"leaves_qty":    item.LeavesQty,
			"seq":           item.Seq,
		}).Debug("order")

		price, _ := strconv.ParseFloat(item.Price, 64)
		qty, _ := strconv.ParseFloat(item.Qty, 64)
		leaves, _ := strconv.ParseFloat(item.LeavesQty, 64)

		w.events <- exchange.Event{
			Type: exchange.EventTypeOrder,
			Order: &models.Order{
				ID:        item.OrderID,
				LinkID:    item.OrderLink,
				Symbol:    item.Symbol,
				Side:      models.OrderSide(item.Side),
				Type:      models.OrderType(item.OrderType),
				Price:     price,
				Qty:       qty,
				FilledQty: qty - leaves,
				Status:    models.OrderStatus(item.OrderStatus),
				Sequence:  item.Seq,
			},
		}
	}
}

func (w *Client) handleTicker(msg Message) {
	var data []struct {
		Symbol    string `json:"symbol"`
		LastPrice string `json:"lastPrice"`
		Seq       int64  `json:"seq"`
		TS        int64  `json:"ts"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		var single struct {
			Symbol    string `json:"symbol"`
			LastPrice string `json:"lastPrice"`
			Seq       int64  `json:"seq"`
			TS        int64  `json:"ts"`
		}
		if err := json.Unmarshal(msg.Data, &single); err != nil {
			w.logEntry().WithError(err).Warn("Не удалось разобрать ticker.")
			return
		}
		data = append(data, single)
	}

	for _, item := range data {
		price, _ := strconv.ParseFloat(item.LastPrice, 64)

		seq := item.Seq
		if seq == 0 {
			if item.TS > 0 {
				seq = item.TS
			} else {
				seq = msg.TS
			}
		}

		w.events <- exchange.Event{
			Type: exchange.EventTypeTicker,
			Ticker: &models.Ticker{
				Symbol:    item.Symbol,
				LastPrice: price,
				Timestamp: time.UnixMilli(msg.TS),
				Sequence:  seq,
			},
		}
	}
}
