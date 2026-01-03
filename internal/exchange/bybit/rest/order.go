package rest

import (
	"context"
	"dcabot/internal/models"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func (c *Client) PlaceOrder(ctx context.Context, order models.Order) (models.Order, error) {

	body := map[string]any{
		"category":    "spot",
		"symbol":      order.Symbol,
		"side":        order.Side,
		"orderType":   order.Type,
		"qty":         formatWithStep(order.Qty, order.QtyStep),
		"price":       formatWithStep(order.Price, order.PriceStep),
		"timeInForce": order.TimeInForce,
		"orderLinkId": order.LinkID,
	}

	if order.Type == models.OrderTypeMarket {
		delete(body, "price")
		if order.MarketUnit != "" {
			body["marketUnit"] = order.MarketUnit
		}
	}

	var resp bybitResponse[struct {
		OrderID string `json:"orderId"`
	}]

	if err := c.doRequest(ctx, http.MethodPost, "/v5/order/create", nil, body, true, &resp); err != nil {
		return models.Order{}, err
	}

	order.ID = resp.Result.OrderID
	return order, nil
}

func (c *Client) CancelOrder(ctx context.Context, symbol, orderID string) error {
	body := map[string]any{
		"category": "spot",
		"symbol":   symbol,
		"orderId":  orderID,
	}

	var resp bybitResponse[struct{}]

	return c.doRequest(ctx, http.MethodPost, "/v5/order/cancel", nil, body, true, &resp)
}

func (c *Client) GetOpenOrders(ctx context.Context, symbol string) ([]models.Order, error) {
	params := url.Values{}
	params.Set("category", "spot")
	params.Set("symbol", symbol)

	var resp bybitResponse[struct {
		List []struct {
			OrderID      string `json:"orderId"`
			OrderLink    string `json:"orderLinkId"`
			Side         string `json:"side"`
			OrderType    string `json:"orderType"`
			Price        string `json:"price"`
			Qty          string `json:"qty"`
			LeavesQty    string `json:"leavesQty"`
			OrderStatus  string `json:"orderStatus"`
			IsReduceOnly bool   `json:"reduceOnly"`
		} `json:"list"`
	}]

	if err := c.doRequest(ctx, http.MethodGet, "/v5/order/realtime", params, nil, true, &resp); err != nil {
		return nil, err
	}

	var orders []models.Order
	for _, item := range resp.Result.List {
		price, _ := strconv.ParseFloat(item.Price, 64)
		qty, _ := strconv.ParseFloat(item.Qty, 64)
		leaves, _ := strconv.ParseFloat(item.LeavesQty, 64)

		filled := qty - leaves

		orders = append(orders, models.Order{
			ID:        item.OrderID,
			LinkID:    item.OrderLink,
			Symbol:    symbol,
			Side:      models.OrderSide(item.Side),
			Type:      models.OrderType(item.OrderType),
			Price:     price,
			Qty:       qty,
			FilledQty: filled,
			Status:    models.OrderStatus(item.OrderStatus),
			IsReduce:  item.IsReduceOnly,
		})
	}
	return orders, nil
}

func (c *Client) GetFills(ctx context.Context, symbol string) ([]models.Fill, error) {
	params := url.Values{}
	params.Set("category", "spot")
	params.Set("symbol", symbol)

	var resp bybitResponse[struct {
		List []struct {
			OrderID   string `json:"orderId"`
			OrderLink string `json:"orderLinkId"`
			ExecID    string `json:"execId"`
			Side      string `json:"side"`
			ExecPrice string `json:"execPrice"`
			ExecQty   string `json:"execQty"`
			ExecTime  string `json:"execTime"`
		} `json:"list"`
	}]

	if err := c.doRequest(ctx, http.MethodGet, "/v5/execution/list", params, nil, true, &resp); err != nil {
		return nil, err
	}

	var fills []models.Fill
	for _, item := range resp.Result.List {
		price, _ := strconv.ParseFloat(item.ExecPrice, 64)
		qty, _ := strconv.ParseFloat(item.ExecQty, 64)
		tsMs, _ := strconv.ParseInt(item.ExecTime, 10, 64)

		fills = append(fills, models.Fill{
			OrderID:   item.OrderID,
			LinkID:    item.OrderLink,
			ExecID:    item.ExecID,
			Symbol:    symbol,
			Side:      models.OrderSide(item.Side),
			Price:     price,
			Qty:       qty,
			Timestamp: time.UnixMilli(tsMs),
		})
	}
	return fills, nil
}
