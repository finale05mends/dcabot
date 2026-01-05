package engine

import (
	"context"
	"dcabot/internal/exchange"
	"dcabot/internal/models"
	"fmt"
	"strings"
	"time"
)

func (e *Engine) runDry(ctx context.Context) error {
	e.logEntry().Info("Dry-run режим активен: построение плана без торговли.")

	events, err := e.client.Subscribe(ctx, e.cfg.Bot.Symbol)
	if err != nil {
		return err
	}

	price, err := e.waitForDryTicker(ctx, events, 10*time.Second)
	if err != nil {
		return err
	}

	if err := e.printDryPlan(price); err != nil {
		return err
	}

	e.logEntry().Info("Dry-run план готов. Ожидание остановки.")
	e.drainEvents(ctx, events)
	return nil
}

func (e *Engine) waitForDryTicker(ctx context.Context, events <-chan exchange.Event, timeout time.Duration) (float64, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-timer.C:
			return 0, fmt.Errorf("Не удалось получить цену тикера для dry-run.")
		case event, ok := <-events:
			if !ok {
				return 0, fmt.Errorf("Канал событий WS закрыт до получения тикера.")
			}
			if event.Type != exchange.EventTypeTicker || event.Ticker == nil {
				continue
			}
			if event.Ticker.LastPrice > 0 {
				return event.Ticker.LastPrice, nil
			}
		}
	}
}

func (e *Engine) drainEvents(ctx context.Context, events <-chan exchange.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-events:
			if !ok {
				return
			}
		}
	}
}

func (e *Engine) printDryPlan(entryPrice float64) error {
	e.ensureDealID()
	e.ensureStateMaps()

	side, err := normalizeSide(e.cfg.Bot.Side)
	if err != nil {
		return err
	}
	qtyUnit := e.qtyUnit()

	entryPrice = e.roundPrice(entryPrice)
	if entryPrice <= 0 {
		return fmt.Errorf("Некорректная цена входа для dry-run: %f", entryPrice)
	}

	entryQty := e.cfg.Bot.BaseOrderQty
	baseQty := entryQty
	quoteQty := 0.0
	if strings.EqualFold(qtyUnit, "quoteCoin") {
		quoteQty = entryQty
		baseQty = entryQty / entryPrice
	} else {
		quoteQty = entryQty * entryPrice
	}
	baseQty = e.roundQty(baseQty)
	quoteQty = baseQty * entryPrice

	if strings.EqualFold(qtyUnit, "baseCoin") && baseQty < e.rules.MinQty {
		return fmt.Errorf("Объём входа меньше минимального: %f", baseQty)
	}
	if err := e.validateMinNotional(models.Order{
		Type:       models.OrderTypeMarket,
		Qty:        entryQty,
		MarketUnit: qtyUnit,
	}, entryPrice); err != nil {
		return err
	}

	fields := map[string]interface{}{
		"deal_id":     e.state.DealID,
		"side":        side,
		"entry_price": formatFloatPlain(entryPrice),
		"qty_unit":    qtyUnit,
		"qty_base":    formatFloatPlain(baseQty),
		"qty_quote":   formatFloatPlain(quoteQty),
	}
	if strings.EqualFold(qtyUnit, "quoteCoin") {
		fields["quote_requested"] = formatFloatPlain(entryQty)
	}
	fields["type"] = models.OrderTypeMarket
	fields["qty"] = formatFloatPlain(baseQty)
	e.logEntry().WithFields(fields).Info("Входной ордер. (dry-run)")
	e.logEntry().Info("Отправка market ордер на вход. (dry-run)")

	tpPrice := CalcTPPrice(entryPrice, e.cfg.Bot.TPPercent, side)
	tpPrice = e.roundPrice(tpPrice)
	tpQty := e.roundQty(baseQty)
	tpSuffix := e.nextTPSuffix()
	tpLinkID := e.linkID(tpSuffix)

	if tpQty < e.rules.MinQty {
		e.logEntry().WithFields(map[string]interface{}{
			"tp_price": formatFloatPlain(tpPrice),
			"tp_qty":   formatFloatPlain(tpQty),
			"min_qty":  formatFloatPlain(e.rules.MinQty),
		}).Warn("Объём TP меньше минимального, пропуск постановки. (dry-run)")
	} else if err := e.validateMinNotional(models.Order{Price: tpPrice, Qty: tpQty, Type: models.OrderTypeLimit}, tpPrice); err != nil {
		e.logEntry().WithFields(map[string]interface{}{
			"tp_price": formatFloatPlain(tpPrice),
			"tp_qty":   formatFloatPlain(tpQty),
			"error":    err.Error(),
		}).Warn("TP пропущен из-за min notional. (dry-run)")
	} else {
		e.logEntry().WithFields(map[string]interface{}{
			"link_id":  tpLinkID,
			"tp_price": formatFloatPlain(tpPrice),
			"tp_qty":   formatFloatPlain(tpQty),
		}).Info("Постановка TP. (dry-run)")
	}

	safetyOrders := CalcSafetyOrders(entryPrice, e.cfg.Bot.SOCount, e.cfg.Bot.SOStepPercent, e.cfg.Bot.SOStepMultiplier, e.cfg.Bot.SOBaseQty, e.cfg.Bot.SOQtyMultiplier, side)
	e.logEntry().WithField("count", len(safetyOrders)).Info("План сетки страховочных ордеров. (dry-run)")

	totalQty := baseQty
	avgPrice := entryPrice
	lastTPPrice := tpPrice
	lastTPLinkID := tpLinkID
	for i, so := range safetyOrders {
		price := e.roundPrice(so.Price)
		qty := so.Qty
		if strings.EqualFold(qtyUnit, "quoteCoin") {
			if price <= 0 {
				e.logEntry().WithFields(map[string]interface{}{
					"index":         i + 1,
					"total_percent": so.TotalPercent,
					"price":         formatFloatPlain(price),
				}).Warn("Страховочный ордер пропущен, нет цены для пересчёта объёма. (dry-run)")
				continue
			}
			qty = qty / price
		}
		qty = e.roundQty(qty)

		if qty < e.rules.MinQty {
			e.logEntry().WithFields(map[string]interface{}{
				"index":   i + 1,
				"price":   formatFloatPlain(price),
				"qty":     formatFloatPlain(qty),
				"min_qty": formatFloatPlain(e.rules.MinQty),
			}).Warn("Страховочный ордер пропущен, объём меньше минимального. (dry-run)")
			continue
		}
		if err := e.validateMinNotional(models.Order{Price: price, Qty: qty, Type: models.OrderTypeLimit}, price); err != nil {
			e.logEntry().WithFields(map[string]interface{}{
				"index": i + 1,
				"price": formatFloatPlain(price),
				"qty":   formatFloatPlain(qty),
				"error": err.Error(),
			}).Warn("Страховочный ордер пропущен из-за min notional. (dry-run)")
			continue
		}

		linkID := e.linkID(fmt.Sprintf("so-%d", i+1))
		e.logEntry().WithFields(map[string]interface{}{
			"index":         i + 1,
			"link_id":       linkID,
			"price":         formatFloatPlain(price),
			"qty":           formatFloatPlain(qty),
			"total_percent": so.TotalPercent,
		}).Info("Постановка страховочного ордера. (dry-run)")
		e.logEntry().WithFields(map[string]interface{}{
			"link_id": linkID,
			"price":   formatFloatPlain(price),
			"qty":     formatFloatPlain(qty),
		}).Info("Исполнен ордер -- Планировка перестановки TP. (dry-run)")

		totalCost := avgPrice*totalQty + price*qty
		totalQty += qty
		avgPrice = CalcAvgPrice(totalCost, totalQty)
		newTP := e.roundPrice(CalcTPPrice(avgPrice, e.cfg.Bot.TPPercent, side))
		newTPQty := e.roundQty(totalQty)
		newTPSuffix := e.nextTPSuffix()
		newTPLinkID := e.linkID(newTPSuffix)

		e.logEntry().WithFields(map[string]interface{}{
			"old_id":    lastTPLinkID,
			"old_price": formatFloatPlain(lastTPPrice),
			"new_price": formatFloatPlain(newTP),
			"qty":       formatFloatPlain(newTPQty),
		}).Info("Перестановка TP. (dry-run)")

		e.logEntry().WithFields(map[string]interface{}{
			"link_id": newTPLinkID,
			"price":   formatFloatPlain(newTP),
			"qty":     formatFloatPlain(newTPQty),
		}).Info("Постановка TP. (dry-run)")

		lastTPPrice = newTP
		lastTPLinkID = newTPLinkID
	}

	return nil
}
