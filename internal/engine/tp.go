package engine

import (
	"context"
	"dcabot/internal/models"
	"fmt"
	"time"
)

func (e *Engine) placeTP(ctx context.Context, tpPrice, qty float64, linkSuffix string) error {
	if qty < e.rules.MinQty {
		e.logEntry().WithFields(map[string]interface{}{
			"qty":     qty,
			"min_qty": e.rules.MinQty,
		}).Warn("Объём TP меньше минимального, пропуск постановки.")
		return nil
	}
	if err := e.waitNoOpenTPOrders(ctx, 5, 500*time.Millisecond); err != nil {
		return err
	}
	adjustedQty, err := e.resolveTPQty(ctx, qty)
	if err != nil {
		return err
	}
	qty = adjustedQty
	tpLinkID := e.linkID(linkSuffix)
	if err := e.validateMinNotional(models.Order{Price: tpPrice, Qty: qty, Type: models.OrderTypeLimit}, tpPrice); err != nil {
		return err
	}
	tpOrder := models.Order{
		Symbol:      e.cfg.Bot.Symbol,
		Side:        oppositeSide(e.state.Side),
		Type:        models.OrderTypeLimit,
		Kind:        models.OrderKindTP,
		Price:       tpPrice,
		Qty:         qty,
		LinkID:      tpLinkID,
		TimeInForce: "GTC",
		IsReduce:    true,
		PriceStep:   e.rules.TickSize,
		QtyStep:     e.rules.LotSize,
	}

	e.mu.Lock()
	e.state.TPlinkID = tpLinkID
	e.state.PlannedTPPrice = tpPrice
	e.state.PlannedTPQty = qty
	e.mu.Unlock()

	e.logEntry().WithFields(map[string]interface{}{
		"link_id": tpLinkID,
		"price":   tpPrice,
		"qty":     qty,
	}).Info("Постановка TP.")
	order, err := e.placeOrderIdempotent(ctx, tpOrder)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.state.TPOrderID = order.ID
	e.mu.Unlock()
	e.log.WithOrderID(order.ID).WithField("component", "engine").WithField("symbol", e.cfg.Bot.Symbol).Info("TP поставлен.")
	e.confirmTPStatus(ctx, tpOrder, order.ID)
	return nil
}

func (e *Engine) rebuildTP(ctx context.Context) error {
	e.mu.Lock()
	if !e.state.Active {
		e.mu.Unlock()
		return nil
	}
	tpPrice := CalcTPPrice(e.state.AvgPrice, e.cfg.Bot.TPPercent, e.state.Side)
	tpPrice = e.roundPrice(tpPrice)
	qty := e.roundQty(e.state.TotalQty)
	oldOrderID := e.state.TPOrderID
	oldTPPrice := e.state.PlannedTPPrice
	e.mu.Unlock()

	if oldOrderID != "" {
		e.logEntry().WithFields(map[string]interface{}{
			"old_id":    oldOrderID,
			"old_price": oldTPPrice,
			"new_price": tpPrice,
			"qty":       qty,
		}).Info("Перестановка TP.")
		if err := e.withRetryVoid(ctx, func() error {
			return e.client.CancelOrder(ctx, e.cfg.Bot.Symbol, oldOrderID)
		}); err != nil {
			if !isOrderNotExistError(err) {
				return err
			}
			e.mu.Lock()
			e.state.TPOrderID = ""
			e.mu.Unlock()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return e.placeTP(ctx, tpPrice, qty, e.nextTPSuffix())
}

func (e *Engine) scheduleTPRebuild(ctx context.Context) {
	const debounce = 700 * time.Millisecond
	const retryDelay = 1 * time.Second

	e.mu.Lock()
	if !e.state.Active {
		e.mu.Unlock()
		return
	}
	e.tpRebuildAt = time.Now().Add(debounce)
	if e.tpRebuildScheduled {
		e.mu.Unlock()
		return
	}
	e.tpRebuildScheduled = true
	e.mu.Unlock()

	go func() {
		for {
			e.mu.Lock()
			dueAt := e.tpRebuildAt
			e.mu.Unlock()

			wait := time.Until(dueAt)
			if wait > 0 {
				select {
				case <-ctx.Done():
					e.mu.Lock()
					e.tpRebuildScheduled = false
					e.mu.Unlock()
					return
				case <-time.After(wait):
				}
			}

			e.mu.Lock()
			if time.Now().Before(e.tpRebuildAt) {
				e.mu.Unlock()
				continue
			}
			e.tpRebuildScheduled = false
			e.mu.Unlock()

			if err := e.rebuildTP(ctx); err != nil {
				e.logEntry().WithError(err).Warn("Не удалось переставить TP.")
				e.mu.Lock()
				e.tpRebuildAt = time.Now().Add(retryDelay)
				e.tpRebuildScheduled = true
				e.mu.Unlock()
				continue
			}
			return
		}
	}()
}

func (e *Engine) resolveTPQty(ctx context.Context, qty float64) (float64, error) {
	if e.state.Side != models.OrderSideBuy {
		return qty, nil
	}
	base := e.rules.BaseCoin
	if base == "" {
		return qty, nil
	}
	qty = e.roundQty(qty)
	minAvailable := qty - (e.rules.LotSize / 2)
	if minAvailable < 0 {
		minAvailable = 0
	}

	var lastErr error
	var lastAvailable float64
	var lastWallet float64
	const attempts = 8
	const delay = 500 * time.Millisecond
	const settleDelay = 2 * time.Second
	const fullRatioThreshold = 0.99

	e.mu.Lock()
	lastFillAt := e.state.LastFillAt
	e.mu.Unlock()
	if !lastFillAt.IsZero() && time.Since(lastFillAt) < settleDelay {
		wait := settleDelay - time.Since(lastFillAt)
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(wait):
		}
	}

	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		balances, err := e.client.GetBalances(ctx, []string{base})
		if err != nil {
			lastErr = err
		} else if bal, ok := balances[base]; ok {
			lastAvailable = bal.Available
			lastWallet = bal.Wallet
			balance := lastWallet
			if balance <= 0 {
				balance = lastAvailable
			}
			if balance >= minAvailable {
				return qty, nil
			}
		}
		if i < attempts-1 {
			if lastErr != nil {
				e.logEntry().WithError(lastErr).Warn("Ожилание доступного баланса для TP.")
			} else {
				e.logEntry().WithFields(map[string]interface{}{
					"need":      qty,
					"available": lastAvailable,
					"wallet":    lastWallet,
				}).Debug("Ожидание доступного баланса для TP.")
			}
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	if lastErr != nil {
		return 0, fmt.Errorf("Не удалось получить баланс для TP: %w", lastErr)
	}

	balance := lastWallet
	if balance <= 0 {
		balance = lastAvailable
	}
	if balance > 0 && qty > 0 && (balance/qty) < fullRatioThreshold {
		return 0, fmt.Errorf("Баланс для TP не обновился: need=%f available=%f wallet=%f", qty, lastAvailable, lastWallet)
	}
	adjusted := e.roundQty(balance)
	if adjusted < e.rules.MinQty {
		e.logEntry().WithFields(map[string]interface{}{
			"need":      qty,
			"available": lastAvailable,
			"wallet":    lastWallet,
			"min_qty":   e.rules.MinQty,
		}).Warn("Недостаточный баланс для TP.")
		return 0, nil
	}
	if adjusted < qty {
		e.logEntry().WithFields(map[string]interface{}{
			"was":       qty,
			"now":       adjusted,
			"wallet":    lastWallet,
			"available": lastAvailable,
		}).Info("Корректировка TP по балансу.")
		e.mu.Lock()
		if adjusted < e.state.TotalQty && (adjusted/qty) >= fullRatioThreshold {
			e.state.TotalQty = adjusted
		}
		e.mu.Unlock()
	}
	return adjusted, nil
}

func (e *Engine) waitNoOpenTPOrders(ctx context.Context, attempts int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		hasOpen, err := e.hasOpenTPOrders(ctx)
		if err != nil {
			lastErr = err
		} else if !hasOpen {
			return nil
		} else {
			lastErr = nil
		}
		if i < attempts-1 {
			if lastErr != nil {
				e.logEntry().WithError(lastErr).Warn("Ожидание закрытия активного TP перед постановкой.")
			} else {
				e.logEntry().Debug("Ожидание закрытия активного TP перед постановкой.")
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("Есть открытый TP, отложена постановка нового.")
}

func (e *Engine) confirmTPStatus(ctx context.Context, tpOrder models.Order, orderID string) {
	openOrders, err := e.withRetryOrders(ctx, tpOrder.Symbol)
	if err != nil {
		e.logEntry().WithError(err).Warn("Не удалось проверить TP через open orders.")
		return
	}
	for _, ord := range openOrders {
		if ord.ID == orderID || ord.LinkID == tpOrder.LinkID {
			leavesQty := ord.Qty - ord.FilledQty
			e.logEntry().WithFields(map[string]interface{}{
				"order_id":   ord.ID,
				"status":     ord.Status,
				"leaves_qty": leavesQty,
				"price":      ord.Price,
				"qty":        ord.Qty,
			}).Info("TP подтверждён в open orders.")
			return
		}
	}

	fills, err := e.withRetryFills(ctx, tpOrder.Symbol)
	if err != nil {
		e.logEntry().WithError(err).Warn("Не удалось проверить исполнения TP.")
		return
	}
	var filledQty float64
	found := false
	for _, fill := range fills {
		if fill.OrderID == orderID || fill.LinkID == tpOrder.LinkID {
			found = true
			filledQty += fill.Qty
		}
	}
	if found {
		e.logEntry().WithFields(map[string]interface{}{
			"order_id":   orderID,
			"filled_qty": filledQty,
		}).Warn("TP не найден в open orders, но есть исполнения.")
		return
	}
	e.logEntry().WithField("order_id", orderID).Warn("TP не найден в open orders и в исполнениях.")
}
