package engine

import (
	"context"
	"dcabot/internal/exchange"
	"dcabot/internal/models"
	"time"
)

func (e *Engine) handleEvents(ctx context.Context, events <-chan exchange.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				e.logEntry().Warn("Канал событий WS закрыт.")
				return
			}
			switch event.Type {
			case exchange.EventTypeFill:
				if event.Fill != nil {
					e.handleFill(ctx, *event.Fill)
				}
			case exchange.EventTypeOrder:
				if event.Order != nil {
					e.handleOrder(ctx, *event.Order)
				}
			case exchange.EventTypeTicker:
				if event.Ticker != nil {
					e.handleTicker(ctx, *event.Ticker)
				}
			case exchange.EventTypeReconnect:
				e.logEntry().Info("Получен сигнал реконнекта WS, сверка ордеров.")
				if err := e.syncOpenOrders(ctx); err != nil {
					e.logEntry().WithError(err).Warn("Не удалось сверить ордера после реконнекта.")
				}
			}
		}
	}
}

func (e *Engine) syncOpenOrders(ctx context.Context) error {
	if !e.state.Active {
		return nil
	}
	e.ensureStateMaps()
	e.ensureDealID()
	openOrders, err := e.withRetryOrders(ctx, e.cfg.Bot.Symbol)
	if err != nil {
		return err
	}

	tpFound := false
	safetyFound := make(map[string]bool)
	for linkID := range e.state.SafetyOrders {
		safetyFound[linkID] = false
	}

	for _, order := range openOrders {
		if order.LinkID == e.state.TPlinkID {
			e.state.TPOrderID = order.ID
			tpFound = true
		}
		if _, ok := safetyFound[order.LinkID]; ok {
			e.state.SafetyOrders[order.LinkID] = order.ID
			safetyFound[order.LinkID] = true
		}
	}

	if !tpFound && e.state.TPlinkID != "" {
		e.logEntry().Warn("TP не найден. Перестановка.")
		if err := e.rebuildTP(ctx); err != nil {
			return err
		}
	}

	for linkID, found := range safetyFound {
		if !found {
			e.logEntry().WithField("link_id", linkID).Warn("Страховочный ордер не найден, попытка перестановки.")
		}
	}

	if err := e.rebuildMissingSafetyOrders(ctx); err != nil {
		return err
	}

	return nil
}

func (e *Engine) handleFill(ctx context.Context, fill models.Fill) {
	e.mu.Lock()
	e.ensureStateMaps()

	if fill.ExecID != "" {
		if e.state.ProcessedExecIDs[fill.ExecID] {
			e.mu.Unlock()
			return
		}
		e.state.ProcessedExecIDs[fill.ExecID] = true
	}

	if fill.Sequence > 0 {
		if fill.Sequence < e.state.LastFillSeq {
			e.mu.Unlock()
			return
		}
		if fill.Sequence > e.state.LastFillSeq {
			e.state.LastFillSeq = fill.Sequence
		}
	}

	if !fill.Timestamp.IsZero() {
		e.state.LastFillAt = fill.Timestamp
	} else {
		e.state.LastFillAt = time.Now()
	}
	e.mu.Unlock()

	if isTPLinkID(fill.LinkID) || e.state.TPlinkID == fill.LinkID || (e.state.TPOrderID != "" && e.state.TPOrderID == fill.OrderID) {
		e.onTPFill(ctx, fill)
		return
	}

	if fill.Side != e.state.Side {
		return
	}

	e.onPositionIncrease(ctx, fill)
}

func (e *Engine) handleOrder(ctx context.Context, order models.Order) {
	e.mu.Lock()
	if order.Status == models.OrderStatusCanceled && order.ID == e.state.TPOrderID {
		e.state.TPOrderID = ""
	}
	if order.Sequence > 0 && order.Sequence <= e.state.LastOrderSeq {
		e.mu.Unlock()
		return
	}
	if order.Sequence > 0 {
		e.state.LastOrderSeq = order.Sequence
	}
	isTP := (e.state.TPOrderID != "" && order.ID == e.state.TPOrderID) || (e.state.TPlinkID != "" && order.LinkID == e.state.TPlinkID)
	totalQty := e.state.TotalQty
	if isTP && order.Status == models.OrderStatusFilled {
		if order.FilledQty > 0 && order.FilledQty < totalQty {
			e.state.TotalQty = totalQty - order.FilledQty
		} else {
			e.state.TotalQty = 0
		}
		totalQty = e.state.TotalQty
	}
	e.mu.Unlock()

	if isTP && order.Status == models.OrderStatusFilled {
		if e.isQtyZero(totalQty) {
			e.logEntry().WithField("order_id", order.ID).Info("TP полностью исполнен по статусу ордера.")
			e.requestClose(ctx, "TP полностью исполнен (order status).")
		} else {
			e.logEntry().WithField("order_id", order.ID).Warn("TP отмечен как исполненный, но позиция ещё есть, делка продолжается.")
		}
	}
}

func (e *Engine) handleTicker(ctx context.Context, ticker models.Ticker) {
	now := time.Now()
	e.mu.Lock()
	if ticker.Sequence > 0 && ticker.Sequence <= e.state.LastTickerSeq {
		e.mu.Unlock()
		return
	}
	if ticker.Sequence > 0 {
		e.state.LastTickerSeq = ticker.Sequence
	}
	e.state.LastTicker = ticker
	e.state.UpdatedAt = now

	active := e.state.Active
	dealID := e.state.DealID
	tpPrice := e.state.PlannedTPPrice
	tpQty := e.state.PlannedTPQty
	tpOrderID := e.state.TPOrderID
	tpLinkID := e.state.TPlinkID
	totalQty := e.state.TotalQty
	avgPrice := e.state.AvgPrice

	if now.Sub(e.lastTickerLog) < 1*time.Second {
		e.mu.Unlock()
		return
	}
	e.lastTickerLog = now
	e.mu.Unlock()

	if !active || dealID == "" {
		return
	}

	base := e.rules.BaseCoin
	quote := e.rules.QuoteCoin
	baseBal := 0.0
	quoteBal := 0.0
	balanceErr := ""
	if base != "" || quote != "" {
		coins := []string{}
		if base != "" {
			coins = append(coins, base)
		}
		if quote != "" && quote != base {
			coins = append(coins, quote)
		}
		balances, err := e.client.GetBalances(ctx, coins)
		if err != nil {
			balanceErr = err.Error()
		} else {
			if bal, ok := balances[base]; ok {
				baseBal = bal.Wallet
				if baseBal == 0 {
					baseBal = bal.Available
				}
			}
			if bal, ok := balances[quote]; ok {
				quoteBal = bal.Wallet
				if quoteBal == 0 {
					quoteBal = bal.Available
				}
			}
		}
	}

	fields := map[string]interface{}{
		"symbol":        ticker.Symbol,
		"price":         ticker.LastPrice,
		"seq":           ticker.Sequence,
		"ts":            ticker.Timestamp.UnixMilli(),
		"balance_base":  baseBal,
		"balance_quote": quoteBal,
		"tp_price":      tpPrice,
		"tp_qty":        tpQty,
		"tp_order_id":   tpOrderID,
		"tp_link_id":    tpLinkID,
		"total_qty":     totalQty,
		"avg_price":     avgPrice,
	}
	if balanceErr != "" {
		fields["balance_error"] = balanceErr
	}
	e.logEntry().WithFields(fields).Debug("ticker")
}

func (e *Engine) onTPFill(ctx context.Context, fill models.Fill) {
	e.mu.Lock()
	e.ensureStateMaps()
	e.state.TPFilledQty += fill.Qty
	e.state.TotalQty -= fill.Qty
	if e.state.TotalQty < 0 {
		e.state.TotalQty = 0
	}
	e.state.UpdatedAt = time.Now()
	totalQty := e.state.TotalQty
	e.mu.Unlock()

	e.logEntry().WithFields(map[string]interface{}{
		"order_id":  fill.OrderID,
		"link_id":   fill.LinkID,
		"qty":       fill.Qty,
		"price":     fill.Price,
		"total_qty": totalQty,
	}).Info("fill TP")

	if e.isQtyZero(totalQty) {
		e.requestClose(ctx, "TP полностью исполнен.")
	}

	if totalQty > 0 {
		e.logEntry().WithField("order_id", fill.OrderID).Info("Частичное исполнение TP/")
	}
}

func (e *Engine) onPositionIncrease(ctx context.Context, fill models.Fill) {
	e.mu.Lock()
	e.ensureStateMaps()
	prevFilled := e.state.FilledByLink[fill.LinkID]
	e.state.FilledByLink[fill.LinkID] = prevFilled + fill.Qty
	totalCost := e.state.AvgPrice*e.state.TotalQty + fill.Price*fill.Qty
	e.state.TotalQty += fill.Qty
	e.state.AvgPrice = CalcAvgPrice(totalCost, e.state.TotalQty)
	e.state.UpdatedAt = time.Now()
	newAvg := e.state.AvgPrice
	totalQty := e.state.TotalQty
	wasClosing := e.state.Closing
	if wasClosing {
		e.state.Closing = false
		e.state.CloseRequested = false
		e.state.CloseReason = ""
	}
	e.mu.Unlock()

	if prevFilled > 0 {
		e.logEntry().WithField("link_id", fill.LinkID).Info("Частичное исполнение ордера.")
	}

	fillLabel := "fill"
	if isSafetyLinkID(fill.LinkID) {
		fillLabel = "fill safety"
	} else if isEntryLinkID(fill.LinkID) {
		fillLabel = "fill entry"
	}
	e.logEntry().WithFields(map[string]interface{}{
		"label":     fillLabel,
		"link_id":   fill.LinkID,
		"order_id":  fill.OrderID,
		"qty":       fill.Qty,
		"price":     fill.Price,
		"total_qty": totalQty,
		"avg":       newAvg,
	}).Info("fill")
	e.logEntry().WithField("avg", newAvg).Debug("Средняя цена пересчитана после исполнения.")
	e.logEntry().WithFields(map[string]interface{}{
		"link_id":   fill.LinkID,
		"avg":       newAvg,
		"total_qty": totalQty,
	}).Info("Исполнен ордер -- Планировка перестановки TP.")

	if wasClosing {
		e.logEntry().Info("Закрытие отменено: поступило новое исполнение, сделка продолжается.")
	}

	e.scheduleTPRebuild(ctx)
}
