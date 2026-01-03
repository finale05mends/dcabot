package engine

import (
	"context"
	"dcabot/internal/models"
	"fmt"
	"strings"
	"time"
)

func (e *Engine) openDeal(ctx context.Context) error {
	if e.state.Active {
		return nil
	}

	e.ensureDealID()
	e.ensureStateMaps()
	side, err := normalizeSide(e.cfg.Bot.Side)
	if err != nil {
		return err
	}
	entryLinkID := e.linkID("entry")
	qtyUnit := e.qtyUnit()

	entryQty := e.cfg.Bot.BaseOrderQty
	if !strings.EqualFold(qtyUnit, "quoteCoin") {
		entryQty = e.roundQty(entryQty)
	}

	entryOrder := models.Order{
		Symbol:      e.cfg.Bot.Symbol,
		Side:        side,
		Type:        models.OrderTypeMarket,
		Kind:        models.OrderKindEntry,
		Qty:         entryQty,
		LinkID:      entryLinkID,
		TimeInForce: "IOC",
		MarketUnit:  qtyUnit,
		QtyStep:     e.rules.LotSize,
	}

	if strings.EqualFold(qtyUnit, "baseCoin") && entryOrder.Qty < e.rules.MinQty {
		return fmt.Errorf("Объём входа меньше минимального: %f", entryOrder.Qty)
	}

	priceHint := e.state.LastTicker.LastPrice
	if priceHint == 0 && e.rules.MinNotional > 0 {
		var err error
		priceHint, err = e.waitForTickerPrice(ctx, 10*time.Second)
		if err != nil {
			return err
		}
	}
	if err := e.validateMinNotional(entryOrder, priceHint); err != nil {
		return err
	}

	e.logEntry().WithFields(map[string]interface{}{
		"side": entryOrder.Side,
		"type": entryOrder.Type,
		"qty":  entryOrder.Qty,
	}).Info("Входной ордер.")

	_, err = e.placeOrderIdempotent(ctx, entryOrder)
	if err != nil {
		return err
	}

	e.logEntry().Info("Отправка market ордер на вход.")

	fill, execIDs, err := e.waitEntryFill(ctx, entryLinkID)
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.state = DealState{
		Active:      true,
		DealID:      e.state.DealID,
		Symbol:      entryOrder.Symbol,
		Side:        entryOrder.Side,
		EntryPrice:  fill.Price,
		EntryLinkID: entryLinkID,
		AvgPrice:    fill.Price,
		TotalQty:    fill.Qty,
		FilledByLink: map[string]float64{
			entryLinkID: fill.Qty,
		},
		TPFilledQty:      0,
		ProcessedExecIDs: map[string]bool{},
		PlannedTPQty:     0,
		SafetyOrders:     map[string]string{},
		UpdatedAt:        time.Now(),
	}
	for _, execID := range execIDs {
		if execID != "" {
			e.state.ProcessedExecIDs[execID] = true
		}
	}
	e.mu.Unlock()

	return e.placeTPAndSafety(ctx, fill.Price)
}

func (e *Engine) waitEntryFill(ctx context.Context, linkID string) (models.Fill, []string, error) {
	timeout := time.NewTimer(20 * time.Second)
	defer timeout.Stop()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return models.Fill{}, nil, ctx.Err()
		case <-timeout.C:
			return models.Fill{}, nil, fmt.Errorf("Не дождались исполнения входа.")
		case <-ticker.C:
			fills, err := e.withRetryFills(ctx, e.cfg.Bot.Symbol)
			if err != nil {
				continue
			}
			var totalQty float64
			var totalCost float64
			var lastFill models.Fill
			var execIDs []string
			for _, fill := range fills {
				if fill.LinkID != linkID {
					continue
				}
				totalQty += fill.Qty
				totalCost += fill.Price * fill.Qty
				lastFill = fill
				if fill.ExecID != "" {
					execIDs = append(execIDs, fill.ExecID)
				}
			}
			if totalQty > 0 {
				return models.Fill{
					OrderID:   lastFill.OrderID,
					LinkID:    linkID,
					ExecID:    lastFill.ExecID,
					Symbol:    lastFill.Symbol,
					Side:      lastFill.Side,
					Price:     CalcAvgPrice(totalCost, totalQty),
					Qty:       totalQty,
					Timestamp: lastFill.Timestamp,
					Sequence:  lastFill.Sequence,
				}, execIDs, nil
			}
		}
	}
}

func (e *Engine) placeTPAndSafety(ctx context.Context, entryPrice float64) error {
	tpPrice := CalcTPPrice(entryPrice, e.cfg.Bot.TPPercent, e.state.Side)
	tpPrice = e.roundPrice(tpPrice)
	totalQty := e.roundQty(e.state.TotalQty)

	if err := e.placeTP(ctx, tpPrice, totalQty, e.nextTPSuffix()); err != nil {
		return err
	}

	if err := e.placeSafetyOrders(ctx, entryPrice); err != nil {
		return err
	}

	return nil
}

func (e *Engine) requestClose(ctx context.Context, reason string) {
	e.mu.Lock()
	if !e.state.Active || e.state.Closing {
		e.mu.Unlock()
		return
	}
	e.state.Closing = true
	e.state.CloseRequested = true
	e.state.CloseReason = reason
	e.mu.Unlock()

	e.logEntry().WithField("reason", reason).Info("Закрытие цикла сделки.")

	go func() {
		const settleDelay = 1 * time.Second

		if err := e.cancelSafetyOrders(ctx); err != nil {
			e.logEntry().WithError(err).Warn("Не удалось отменить страховочные ордера.")
		} else {
			e.logEntry().Info("Страховочные ордера отменены.")
		}

		for {
			if ctx.Err() != nil {
				return
			}

			e.mu.Lock()
			active := e.state.Active
			closing := e.state.Closing
			totalQty := e.state.TotalQty
			lastFillAt := e.state.LastFillAt
			e.mu.Unlock()

			if !active || !closing {
				return
			}

			if !e.isQtyZero(totalQty) {
				e.logEntry().Warn("Закрытие отменено: есть позиция, сделка продолжается.")
				e.mu.Lock()
				e.state.Closing = false
				e.state.CloseRequested = false
				e.state.CloseReason = ""
				e.mu.Unlock()
				if err := e.rebuildTP(ctx); err != nil {
					e.logEntry().WithError(err).Warn("Не удалось переставить TP.")
				}
				return
			}

			if baseQty, err := e.baseAvailable(ctx); err == nil {
				rounded := e.roundQty(baseQty)
				if !e.isQtyZero(rounded) {
					e.logEntry().Warn("Закрытие отменено: есть позиция по балансу, сделка продолжается.")
					e.mu.Lock()
					e.state.TotalQty = rounded
					e.state.Closing = false
					e.state.CloseRequested = false
					e.state.CloseReason = ""
					e.mu.Unlock()
					if err := e.rebuildTP(ctx); err != nil {
						e.logEntry().WithError(err).Warn("Не удалось переставить TP.")
					}
					return
				}
			}

			if !lastFillAt.IsZero() && time.Since(lastFillAt) < settleDelay {
				select {
				case <-ctx.Done():
					return
				case <-time.After(settleDelay):
				}
				continue
			}

			hasOpen, err := e.hasOpenBotOrders(ctx)
			if err != nil {
				e.logEntry().WithError(err).Warn("Не удалось проверить открытые ордера перед завершением цикла.")
			} else if !hasOpen {
				e.finalizeClose(ctx)
				return
			} else {
				if canceled, cancelErr := e.cancelOpenBotOrders(ctx); cancelErr != nil {
					e.logEntry().WithError(cancelErr).Warn("Не удалось отменить открытые ордера перед завершением цикла.")
				} else if canceled > 0 {
					e.logEntry().WithField("count", canceled).Info("Отмена открытых ордеров перед завершением цикла.")
				}
				e.logEntry().Info("Ожидание закрытия открытых ордеров перед завершением цикла.")
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(settleDelay):
			}
		}
	}()
}

func (e *Engine) finalizeClose(ctx context.Context) {
	e.mu.Lock()
	if !e.state.Active {
		e.mu.Unlock()
		return
	}
	now := time.Now()
	e.state.Active = false
	e.state.Closing = false
	e.state.CloseRequested = false
	e.state.CloseReason = ""
	e.state.DealID = ""
	e.state.Symbol = ""
	e.state.Side = ""
	e.state.EntryPrice = 0
	e.state.EntryLinkID = ""
	e.state.AvgPrice = 0
	e.state.TotalQty = 0
	e.state.FilledByLink = map[string]float64{}
	e.state.ProcessedExecIDs = map[string]bool{}
	e.state.TPFilledQty = 0
	e.state.TPOrderID = ""
	e.state.TPlinkID = ""
	e.state.PlannedTPQty = 0
	e.state.SafetyOrders = map[string]string{}
	e.state.PlannedTPPrice = 0
	e.state.ClosedAt = &now
	e.state.UpdatedAt = now
	e.mu.Unlock()

	go func() {
		const restartDelay = 1 * time.Second
		const maxChecks = 5

		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(restartDelay):
		}
		for i := 0; i < maxChecks; i++ {
			if ctx.Err() != nil {
				return
			}
			hasOpen, err := e.hasOpenBotOrders(ctx)
			if err != nil {
				e.logEntry().WithError(err).Warn("Не удалось проверить открытые ордера перед новым циклом.")
			} else if !hasOpen {
				break
			} else {
				if canceled, cancelErr := e.cancelOpenBotOrders(ctx); cancelErr != nil {
					e.logEntry().WithError(cancelErr).Warn("Не удалось отменить открытые ордера перед новым циклом.")
				} else if canceled > 0 {
					e.logEntry().WithField("count", canceled).Info("Отмена открытых ордерво перед новым циклом.")
				}
				e.logEntry().Info("Ожидание закрытия открытых ордеров перед новым циклом.")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(restartDelay):
			}
		}
		if baseQty, err := e.baseAvailable(ctx); err == nil {
			if !e.isQtyZero(e.roundQty(baseQty)) {
				e.logEntry().Warn("Новый цикл не запущен: есть позиция по балансу.")
				return
			}
		}

		hasOpen, err := e.hasOpenBotOrders(ctx)
		if err != nil {
			e.logEntry().WithError(err).Warn("Не удалось проверить открытые ордера перед новым циклом.")
			return
		}
		if hasOpen {
			e.logEntry().Warn("Новый цикл не запущен: есть открытые ордера.")
			return
		}
		e.logEntry().Info("Запускаю новый цикл сделки.")
		if err := e.openDeal(ctx); err != nil {
			e.logEntry().WithError(err).Error("Не удалось открыть новый цикл.")
		}
	}()
}
