package engine

import (
	"context"
	"dcabot/internal/models"
	"fmt"
	"strings"
	"time"
)

func (e *Engine) restoreActiveOrders(ctx context.Context) (bool, error) {
	e.ensureStateMaps()

	openOrders, err := e.withRetryOrders(ctx, e.cfg.Bot.Symbol)
	if err != nil {
		return false, err
	}

	deals := make(map[string][]models.Order)
	for _, ord := range openOrders {
		dealID, ok := dealIDFromLinkID(ord.LinkID)
		if !ok {
			continue
		}
		deals[dealID] = append(deals[dealID], ord)
	}

	if len(deals) == 0 {
		return false, nil
	}

	dealID := pickDealID(deals)
	orders := deals[dealID]
	for _, ord := range orders {
		role := "other"
		switch {
		case isTPLinkID(ord.LinkID):
			role = "tp"
		case isSafetyLinkID(ord.LinkID):
			role = "safety"
		case isEntryLinkID(ord.LinkID):
			role = "entry"
		}
		e.logEntry().WithFields(map[string]interface{}{
			"deal_id":  dealID,
			"order_id": ord.ID,
			"link_id":  ord.LinkID,
			"side":     ord.Side,
			"type":     ord.Type,
			"status":   ord.Status,
			"price":    ord.Price,
			"qty":      ord.Qty,
			"role":     role,
		}).Debug("Ордер восстановлен.")
	}

	var tpOrder *models.Order
	safetyOrders := map[string]string{}
	side := models.OrderSide("")
	for i := range orders {
		ord := orders[i]
		if isTPLinkID(ord.LinkID) {
			tmp := ord
			tpOrder = &tmp
		}
		if isSafetyLinkID(ord.LinkID) {
			safetyOrders[ord.LinkID] = ord.ID
			if side == "" {
				side = ord.Side
			}
		}
		if isEntryLinkID(ord.LinkID) && side == "" {
			side = ord.Side
		}
	}

	if side == "" && tpOrder != nil {
		side = oppositeSide(tpOrder.Side)
	}
	if side == "" {
		parsedSide, err := normalizeSide(e.cfg.Bot.Side)
		if err != nil {
			return false, err
		}
		side = parsedSide
	}

	fills, err := e.withRetryFills(ctx, e.cfg.Bot.Symbol)
	if err != nil {
		return false, err
	}

	var buyQty, buyCost, sellQty float64
	var entryPrice float64
	var entryTS time.Time
	filledByLink := map[string]float64{}
	processedExecIDs := map[string]bool{}
	for _, fill := range fills {
		if !strings.HasPrefix(fill.LinkID, dealID+"-") {
			continue
		}
		if fill.ExecID != "" {
			processedExecIDs[fill.ExecID] = true
		}
		filledByLink[fill.LinkID] += fill.Qty
		if fill.Side == models.OrderSideBuy {
			buyQty += fill.Qty
			buyCost += fill.Qty * fill.Price
			if entryPrice == 0 || fill.Timestamp.Before(entryTS) {
				entryPrice = fill.Price
				entryTS = fill.Timestamp
			}
		} else {
			sellQty += fill.Qty
		}
	}

	totalQty := buyQty - sellQty
	avgPrice := 0.0
	if buyQty > 0 {
		avgPrice = buyCost / buyQty
	}
	if entryPrice == 0 {
		entryPrice = avgPrice
	}

	if tpOrder != nil && totalQty <= 0 {
		totalQty = tpOrder.Qty
	}

	if tpOrder == nil && totalQty <= 0 {
		e.logEntry().WithFields(map[string]interface{}{
			"deal_id": dealID,
			"orders":  len(orders),
		}).Info("Восстановление пропущено: позиции нет и TP нет.")
		return false, nil
	}

	if tpOrder == nil && totalQty > 0 && side == models.OrderSideBuy {
		if baseQty, err := e.baseAvailable(ctx); err == nil {
			rounded := e.roundQty(baseQty)
			if e.isQtyZero(rounded) {
				e.logEntry().WithFields(map[string]interface{}{
					"deal_id":    dealID,
					"orders":     len(orders),
					"fills_qty":  totalQty,
					"base_qty":   rounded,
					"base_coin":  e.rules.BaseCoin,
					"tp_missing": true,
				}).Info("Восстановление пропущено: позиции нет по балансу.")
				return false, nil
			}
		}
	}

	entryLinkID := fmt.Sprintf("%s-entry", dealID)

	plannedTPPrice := 0.0
	plannedTPQty := 0.0
	tpOrderID := ""
	tpLinkID := ""
	if tpOrder != nil {
		tpOrderID = tpOrder.ID
		tpLinkID = tpOrder.LinkID
		plannedTPPrice = tpOrder.Price
		plannedTPQty = tpOrder.Qty
	}

	e.mu.Lock()
	lastTicker := e.state.LastTicker
	lastTickerSeq := e.state.LastTickerSeq
	e.state = DealState{
		Active:           true,
		DealID:           dealID,
		Symbol:           e.cfg.Bot.Symbol,
		Side:             side,
		EntryPrice:       entryPrice,
		EntryLinkID:      entryLinkID,
		AvgPrice:         avgPrice,
		TotalQty:         totalQty,
		FilledByLink:     filledByLink,
		TPFilledQty:      0,
		TPOrderID:        tpOrderID,
		TPlinkID:         tpLinkID,
		PlannedTPQty:     plannedTPQty,
		PlannedTPPrice:   plannedTPPrice,
		ProcessedExecIDs: processedExecIDs,
		SafetyOrders:     safetyOrders,
		LastTicker:       lastTicker,
		LastTickerSeq:    lastTickerSeq,
		UpdatedAt:        time.Now(),
	}
	e.mu.Unlock()

	e.logEntry().WithFields(map[string]interface{}{
		"deal_id":     dealID,
		"orders":      len(orders),
		"tp_id":       tpOrderID,
		"safety":      len(safetyOrders),
		"qty":         totalQty,
		"avg_price":   avgPrice,
		"entry_price": entryPrice,
	}).Info("Восстановление ордеров.")
	if totalQty <= 0 {
		e.logEntry().Warn("Восстановление: позиция не найдена по fills, возможно TP уже закрылся.")
	}

	if tpOrder == nil && totalQty > 0 {
		tpBase := avgPrice
		if tpBase == 0 {
			tpBase = entryPrice
		}
		if tpBase > 0 {
			tpPrice := CalcTPPrice(tpBase, e.cfg.Bot.TPPercent, side)
			if err := e.placeTP(ctx, e.roundPrice(tpPrice), e.roundQty(totalQty), e.nextTPSuffix()); err != nil {
				return true, err
			}
		}
	}

	if err := e.rebuildMissingSafetyOrders(ctx); err != nil {
		return true, err
	}

	return true, nil
}

func pickDealID(deals map[string][]models.Order) string {
	var best string
	bestCount := -1
	bestHasTP := false
	for dealID, orders := range deals {
		hasTP := false
		for _, ord := range orders {
			if isTPLinkID(ord.LinkID) {
				hasTP = true
				break
			}
		}
		if best == "" || (hasTP && !bestHasTP) || (hasTP == bestHasTP && len(orders) > bestCount) {
			best = dealID
			bestCount = len(orders)
			bestHasTP = hasTP
		}
	}
	return best
}

func dealIDFromLinkID(linkID string) (string, bool) {
	if strings.HasSuffix(linkID, "-entry") {
		return strings.TrimSuffix(linkID, "-entry"), true
	}
	if strings.HasSuffix(linkID, "-tp") {
		return strings.TrimSuffix(linkID, "-tp"), true
	}
	if idx := strings.LastIndex(linkID, "-tp-"); idx != -1 {
		return linkID[:idx], true
	}
	if idx := strings.LastIndex(linkID, "-so-"); idx != -1 {
		return linkID[:idx], true
	}
	return "", false
}

func isEntryLinkID(linkID string) bool {
	return strings.HasSuffix(linkID, "-entry")
}

func isTPLinkID(linkID string) bool {
	return strings.HasSuffix(linkID, "-tp") || strings.Contains(linkID, "-tp-")
}

func isSafetyLinkID(linkID string) bool {
	return strings.Contains(linkID, "-so-")
}
