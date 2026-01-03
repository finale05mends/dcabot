package engine

import (
	"context"
	"dcabot/internal/models"
	"fmt"
	"strings"
	"sync"
	"time"
)

func (e *Engine) placeSafetyOrders(ctx context.Context, entryPrice float64) error {
	orders := CalcSafetyOrders(entryPrice, e.cfg.Bot.SOCount, e.cfg.Bot.SOStepPercent, e.cfg.Bot.SOStepMultiplier, e.cfg.Bot.SOBaseQty, e.cfg.Bot.SOQtyMultiplier, e.state.Side)
	qtyUnit := e.qtyUnit()

	e.logEntry().WithField("count", len(orders)).Info("План сетки страховочных ордеров.")

	for i, so := range orders {
		price := e.roundPrice(so.Price)
		qty := so.Qty

		e.logEntry().WithFields(map[string]interface{}{
			"index":         i + 1,
			"entry_price":   entryPrice,
			"total_percent": so.TotalPercent,
			"price":         price,
			"qty":           qty,
			"so_count":      len(orders),
		}).Debug("safety_order_raw")

		if strings.EqualFold(qtyUnit, "quoteCoin") {
			if price <= 0 {
				e.logEntry().WithFields(map[string]interface{}{
					"index":         i + 1,
					"so_count":      len(orders),
					"total_percent": so.TotalPercent,
					"entry_price":   entryPrice,
					"price":         price,
				}).Warn("Страховочный ордер пропущен, нет цены для пересчёта объёма.")
				continue
			}
			qty = qty / price
		}
		qty = e.roundQty(qty)
		linkID := e.linkID(fmt.Sprintf("so-%d", i+1))
		notional := price * qty

		e.logEntry().WithFields(map[string]interface{}{
			"index":    i + 1,
			"price":    price,
			"qty":      qty,
			"notional": notional,
		}).Debug("safety_order_plan")

		if qty < e.rules.MinQty {
			e.logEntry().WithFields(map[string]interface{}{
				"qty":     qty,
				"min_qty": e.rules.MinQty,
			}).Warn("Страховочный ордер пропущен, объём меньше минимального.")
			continue
		}

		if orderID, exists := e.state.SafetyOrders[linkID]; exists && orderID != "" {
			e.logEntry().WithField("link_id", linkID).Debug("Страховочный ордер уже зарегистрирован, пропуск.")
			continue
		}

		if existing, err := e.findOpenOrderByLinkID(ctx, e.cfg.Bot.Symbol, linkID); err == nil && existing.ID != "" {
			e.logEntry().WithField("link_id", linkID).Info("Страховочный ордер уже открыт, пропуск.")
			e.mu.Lock()
			e.state.SafetyOrders[linkID] = existing.ID
			e.mu.Unlock()
			continue
		}

		if err := e.validateMinNotional(models.Order{Price: price, Qty: qty, Type: models.OrderTypeLimit}, price); err != nil {
			e.logEntry().WithError(err).Warn("Страховочный ордер пропущен из-за min notional.")
			continue
		}

		order := models.Order{
			Symbol:      e.cfg.Bot.Symbol,
			Side:        e.state.Side,
			Type:        models.OrderTypeLimit,
			Kind:        models.OrderKindSafety,
			Price:       price,
			Qty:         qty,
			LinkID:      linkID,
			TimeInForce: "GTC",
			PriceStep:   e.rules.TickSize,
			QtyStep:     e.rules.LotSize,
		}

		e.logEntry().WithFields(map[string]interface{}{
			"link_id": linkID,
			"price":   price,
			"qty":     qty,
		}).Info("Постановка страховочного ордера.")
		placed, err := e.placeOrderIdempotent(ctx, order)
		if err != nil {
			return err
		}

		e.mu.Lock()
		e.state.SafetyOrders[linkID] = placed.ID
		e.mu.Unlock()
		e.log.WithOrderID(placed.ID).WithField("component", "engine").WithField("symbol", e.cfg.Bot.Symbol).Info("Страховочный ордер поставлен.")

		if i < len(orders)-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
	}

	return nil
}

func (e *Engine) buildSafetyOrders(entryPrice float64) map[string]models.Order {
	orders := CalcSafetyOrders(entryPrice, e.cfg.Bot.SOCount, e.cfg.Bot.SOStepPercent, e.cfg.Bot.SOStepMultiplier, e.cfg.Bot.SOBaseQty, e.cfg.Bot.SOQtyMultiplier, e.state.Side)
	result := make(map[string]models.Order, len(orders))
	for i, so := range orders {
		price := e.roundPrice(so.Price)
		qty := e.roundQty(so.Qty)
		linkID := e.linkID(fmt.Sprintf("so-%d", i+1))
		result[linkID] = models.Order{
			Symbol:      e.cfg.Bot.Symbol,
			Side:        e.state.Side,
			Type:        models.OrderTypeLimit,
			Kind:        models.OrderKindSafety,
			Price:       price,
			Qty:         qty,
			LinkID:      linkID,
			TimeInForce: "GTC",
			PriceStep:   e.rules.TickSize,
			QtyStep:     e.rules.LotSize,
		}
	}
	return result
}

func (e *Engine) rebuildMissingSafetyOrders(ctx context.Context) error {
	if e.state.EntryPrice == 0 {
		return nil
	}
	expected := e.buildSafetyOrders(e.state.EntryPrice)
	for linkID, order := range expected {
		if orderID, exists := e.state.SafetyOrders[linkID]; exists && orderID != "" {
			continue
		}
		if order.Qty < e.rules.MinQty {
			continue
		}
		if err := e.validateMinNotional(order, order.Price); err != nil {
			continue
		}
		placed, err := e.placeOrderIdempotent(ctx, order)
		if err != nil {
			return err
		}
		e.state.SafetyOrders[linkID] = placed.ID
		e.log.WithOrderID(placed.ID).WithField("component", "engine").WithField("symbol", e.cfg.Bot.Symbol).Info("Страховочный ордер переставлен.")
	}
	return nil
}

func (e *Engine) cancelSafetyOrders(ctx context.Context) error {
	e.mu.Lock()
	orderIDs := make([]string, 0, len(e.state.SafetyOrders))
	for _, orderID := range e.state.SafetyOrders {
		if orderID == "" {
			continue
		}
		orderIDs = append(orderIDs, orderID)
	}
	e.mu.Unlock()

	if len(orderIDs) == 0 {
		return nil
	}

	const workers = 3
	jobs := make(chan string, len(orderIDs))
	errCh := make(chan error, len(orderIDs))
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for orderID := range jobs {
			if ctx.Err() != nil {
				return
			}
			if err := e.withRetryVoid(ctx, func() error {
				return e.client.CancelOrder(ctx, e.cfg.Bot.Symbol, orderID)
			}); err != nil {
				if !isOrderNotExistError(err) {
					errCh <- err
				}
			}
		}
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker()
	}
	for _, orderID := range orderIDs {
		jobs <- orderID
	}
	close(jobs)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	e.mu.Lock()
	e.state.SafetyOrders = map[string]string{}
	e.mu.Unlock()
	return nil
}

func (e *Engine) cancelOpenBotOrders(ctx context.Context) (int, error) {
	openOrders, err := e.withRetryOrders(ctx, e.cfg.Bot.Symbol)
	if err != nil {
		return 0, err
	}
	orderIDs := make([]string, 0, len(openOrders))
	for _, ord := range openOrders {
		if _, ok := dealIDFromLinkID(ord.LinkID); !ok {
			continue
		}
		if ord.ID == "" {
			continue
		}
		orderIDs = append(orderIDs, ord.ID)
	}
	if len(orderIDs) == 0 {
		return 0, nil
	}

	const workers = 3
	jobs := make(chan string, len(orderIDs))
	errCh := make(chan error, len(orderIDs))
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for orderID := range jobs {
			if ctx.Err() != nil {
				return
			}
			if err := e.withRetryVoid(ctx, func() error {
				return e.client.CancelOrder(ctx, e.cfg.Bot.Symbol, orderID)
			}); err != nil {
				if !isOrderNotExistError(err) {
					errCh <- err
				}
			}
		}
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker()
	}
	for _, orderID := range orderIDs {
		jobs <- orderID
	}
	close(jobs)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return len(orderIDs), err
		}
	}
	return len(orderIDs), nil
}
