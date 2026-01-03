package engine

import (
	"context"
	"dcabot/internal/models"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (e *Engine) withRetry(ctx context.Context, fn func() (models.Order, error)) (models.Order, error) {
	var lastErr error
	var backoff time.Duration = 1 * time.Second
	for i := 0; i < 5; i++ {
		order, err := fn()
		if err == nil {
			return order, nil
		}
		lastErr = err
		wait := time.Duration(math.Min(float64(backoff), float64(backoff*30)))
		if isRateLimitError(err) {
			wait = time.Duration(math.Min(float64(backoff*4), float64(backoff*30)))
		}
		e.logEntry().WithError(lastErr).Warn("Ошибка, повторяем запрос.")
		select {
		case <-ctx.Done():
			return models.Order{}, ctx.Err()
		case <-time.After(wait):
		}
		backoff *= 2
	}
	return models.Order{}, lastErr
}

func (e *Engine) withRetryVoid(ctx context.Context, fn func() error) error {
	var lastErr error
	var backoff time.Duration = 1 * time.Second
	for i := 0; i < 5; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		wait := time.Duration(math.Min(float64(backoff), float64(backoff*30)))
		if isRateLimitError(lastErr) {
			wait = time.Duration(math.Min(float64(backoff*4), float64(backoff*30)))
		}
		e.logEntry().WithError(lastErr).Warn("Ошибка, повторяем запрос.")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		backoff *= 2
	}
	return lastErr
}

func (e *Engine) withRetryOrders(ctx context.Context, symbol string) ([]models.Order, error) {
	var lastErr error
	var backoff time.Duration = 1 * time.Second
	for i := 0; i < 5; i++ {
		orders, err := e.client.GetOpenOrders(ctx, symbol)
		if err == nil {
			return orders, nil
		}
		lastErr = err
		wait := time.Duration(math.Min(float64(backoff), float64(backoff*30)))
		if isRateLimitError(err) {
			wait = time.Duration(math.Min(float64(backoff*4), float64(backoff*30)))
		}
		e.logEntry().WithError(lastErr).Warn("Ошибка, повторяем запрос.")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		backoff *= 2
	}
	return nil, lastErr
}

func (e *Engine) withRetryFills(ctx context.Context, symbol string) ([]models.Fill, error) {
	var lastErr error
	var backoff time.Duration = 1 * time.Second
	for i := 0; i < 5; i++ {
		fills, err := e.client.GetFills(ctx, symbol)
		if err == nil {
			return fills, nil
		}
		lastErr = err
		wait := time.Duration(math.Min(float64(backoff), float64(backoff*30)))
		if isRateLimitError(err) {
			wait = time.Duration(math.Min(float64(backoff*4), float64(backoff*30)))
		}
		e.logEntry().WithError(lastErr).Warn("Ошибка, повторяем запрос.")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		backoff *= 2
	}
	return nil, lastErr
}

func (e *Engine) placeOrderIdempotent(ctx context.Context, order models.Order) (models.Order, error) {
	if order.LinkID == "" {
		return models.Order{}, fmt.Errorf("Пстой orderLinkId.")
	}

	if order.Type != models.OrderTypeMarket {
		if existing, err := e.findOpenOrderByLinkID(ctx, order.Symbol, order.LinkID); err == nil && existing.ID != "" {
			e.logEntry().WithField("link_id", order.LinkID).Info("Найден существующий ордер по link_id, повтор не нужен.")
			return existing, nil
		}
	}

	e.logOrderContext(ctx, order)
	placed, err := e.withRetry(ctx, func() (models.Order, error) {
		return e.client.PlaceOrder(ctx, order)
	})
	if err == nil {
		return placed, nil
	}
	if isDuplicateClientOrderID(err) {
		if existing, ok := e.findOrderAfterDuplicate(ctx, order.Symbol, order.LinkID); ok {
			return existing, nil
		}
	}

	if order.Type == models.OrderTypeMarket {
		fills, fErr := e.withRetryFills(ctx, order.Symbol)
		if fErr == nil {
			for _, fill := range fills {
				if fill.LinkID == order.LinkID {
					e.logEntry().WithField("link_id", order.LinkID).Info("Ордер уже исполнен, повтор не нужен.")
					return models.Order{ID: fill.OrderID, LinkID: order.LinkID}, nil
				}
			}
		}
	}

	return models.Order{}, err
}

func (e *Engine) findOpenOrderByLinkID(ctx context.Context, symbol, linkID string) (models.Order, error) {
	orders, err := e.withRetryOrders(ctx, symbol)
	if err != nil {
		return models.Order{}, err
	}
	for _, ord := range orders {
		if ord.LinkID == linkID {
			return ord, nil
		}
	}
	return models.Order{}, nil
}

func (e *Engine) waitForTickerPrice(ctx context.Context, timeout time.Duration) (float64, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		price := e.state.LastTicker.LastPrice
		e.mu.Unlock()
		if price > 0 {
			return price, nil
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return 0, fmt.Errorf("Не удалось получить цену тикера для проверки min notional.")
}

func (e *Engine) validateMinNotional(order models.Order, priceHint float64) error {
	if e.rules.MinNotional <= 0 {
		return nil
	}
	price := order.Price
	if order.Type == models.OrderTypeMarket {
		price = priceHint
	}
	if price <= 0 {
		return fmt.Errorf("Нет цены для проверки min notional.")
	}
	notional := price * order.Qty
	if order.Type == models.OrderTypeMarket && strings.EqualFold(order.MarketUnit, "quoteCoin") {
		notional = order.Qty
	}
	if notional < e.rules.MinNotional {
		return fmt.Errorf("Объём меньше min notional: %f < %f", notional, e.rules.MinNotional)
	}
	return nil
}

func (e *Engine) linkID(suffix string) string {
	return fmt.Sprintf("%s-%s", e.state.DealID, suffix)
}

func (e *Engine) nextTPSuffix() string {
	e.mu.Lock()
	e.tpSeq++
	seq := e.tpSeq
	e.mu.Unlock()
	return fmt.Sprintf("tp-%d-%d", time.Now().Unix(), seq%1000)
}

func (e *Engine) ensureDealID() {
	if e.state.DealID == "" {
		e.state.DealID = newDealID()
	}
}

func newDealID() string {
	raw := strings.ReplaceAll(uuid.New().String(), "-", "")
	if len(raw) > 12 {
		return raw[:12]
	}
	return raw
}

func (e *Engine) roundPrice(price float64) float64 {
	return RoundDown(price, e.rules.TickSize)
}

func (e *Engine) roundQty(qty float64) float64 {
	return RoundDown(qty, e.rules.LotSize)
}

func oppositeSide(side models.OrderSide) models.OrderSide {
	if side == models.OrderSideBuy {
		return models.OrderSideSell
	}
	return models.OrderSideBuy
}

func normalizeSide(side string) (models.OrderSide, error) {
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "BUY":
		return models.OrderSideBuy, nil
	case "SELL":
		return models.OrderSideSell, nil
	default:
		return "", fmt.Errorf("Некорректно направление: %s", side)
	}
}

func (e *Engine) qtyUnit() string {
	unit := strings.TrimSpace(e.cfg.Bot.QtyUnit)
	if strings.EqualFold(unit, "quoteCoin") {
		return "quoteCoin"
	}
	return "baseCoin"
}

func isOrderNotExistError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "170213") || strings.Contains(msg, "Order does not exist")
}

func isDuplicateClientOrderID(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "170141") || strings.Contains(msg, "Duplicate clientOrderId")
}

func (e *Engine) isQtyZero(qty float64) bool {
	threshold := e.rules.LotSize / 2
	if threshold <= 0 {
		threshold = 1e-9
	}
	return qty <= threshold
}

func (e *Engine) hasOpenBotOrders(ctx context.Context) (bool, error) {
	openOrders, err := e.withRetryOrders(ctx, e.cfg.Bot.Symbol)
	if err != nil {
		return false, err
	}
	for _, ord := range openOrders {
		if _, ok := dealIDFromLinkID(ord.LinkID); ok {
			return true, nil
		}
	}
	return false, nil
}

func (e *Engine) hasOpenTPOrders(ctx context.Context) (bool, error) {
	e.mu.Lock()
	dealID := e.state.DealID
	active := e.state.Active
	e.mu.Unlock()

	if !active || dealID == "" {
		return false, nil
	}
	openOrders, err := e.withRetryOrders(ctx, e.cfg.Bot.Symbol)
	if err != nil {
		return false, err
	}
	for _, ord := range openOrders {
		if !isTPLinkID(ord.LinkID) {
			continue
		}
		if ordDealID, ok := dealIDFromLinkID(ord.LinkID); ok && ordDealID == dealID {
			return true, nil
		}
	}
	return false, nil
}

func (e *Engine) ensureStateMaps() {
	if e.state.SafetyOrders == nil {
		e.state.SafetyOrders = map[string]string{}
	}
	if e.state.FilledByLink == nil {
		e.state.FilledByLink = map[string]float64{}
	}
	if e.state.ProcessedExecIDs == nil {
		e.state.ProcessedExecIDs = map[string]bool{}
	}
}

func (e *Engine) baseAvailable(ctx context.Context) (float64, error) {
	base := e.rules.BaseCoin
	if base == "" {
		return 0, nil
	}
	balances, err := e.client.GetBalances(ctx, []string{base})
	if err != nil {
		return 0, err
	}
	bal, ok := balances[base]
	if !ok {
		return 0, nil
	}
	if bal.Wallet > 0 {
		return bal.Wallet, nil
	}
	return bal.Available, nil
}

func (e *Engine) logOrderContext(ctx context.Context, order models.Order) {
	base := e.rules.BaseCoin
	quote := e.rules.QuoteCoin
	if base == "" || quote == "" {
		e.logEntry().WithFields(map[string]interface{}{
			"side":        order.Side,
			"type":        order.Type,
			"qty":         order.Qty,
			"price":       order.Price,
			"market_unit": order.MarketUnit,
		}).Info("Попытка ордера.")
		return
	}

	balances, err := e.client.GetBalances(ctx, []string{base, quote})
	if err != nil {
		e.logEntry().WithError(err).Warn("Не удалось получить баланс перед ордером.")
		e.logEntry().WithFields(map[string]interface{}{
			"side":        order.Side,
			"type":        order.Type,
			"qty":         order.Qty,
			"price":       order.Price,
			"market_unit": order.MarketUnit,
		}).Info("Попытка ордера.")
		return
	}

	baseBal := balances[base]
	quoteBal := balances[quote]
	needBase := 0.0
	needQuote := 0.0
	priceHint := order.Price
	if priceHint == 0 {
		priceHint = e.state.LastTicker.LastPrice
	}

	if order.Side == models.OrderSideBuy {
		if order.Type == models.OrderTypeMarket && strings.EqualFold(order.MarketUnit, "quoteCoin") {
			needQuote = order.Qty
		} else if priceHint > 0 {
			needQuote = order.Qty * priceHint
		}
	} else {
		needBase = order.Qty
	}

	e.logEntry().WithFields(map[string]interface{}{
		"side":        order.Side,
		"type":        order.Type,
		"qty":         order.Qty,
		"price":       order.Price,
		"market_unit": order.MarketUnit,
		"need_base":   needBase,
		"need_quote":  needQuote,
		"base_asset":  base,
		"quote_asset": quote,
		"bal_base":    baseBal.Available,
		"bal_quote":   quoteBal.Available,
	}).Info("Попытка ордера.")
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Too many visits!") || strings.Contains(msg, "429") || strings.Contains(msg, "10006")
}

func (e *Engine) findOrderAfterDuplicate(ctx context.Context, symbol, linkID string) (models.Order, bool) {
	const attempts = 3
	const delay = 300 * time.Millisecond
	for i := 0; i < attempts; i++ {
		existing, err := e.findOpenOrderByLinkID(ctx, symbol, linkID)
		if err == nil && existing.ID != "" {
			e.logEntry().WithField("link_id", linkID).Debug("Найден ордер после duplicate clientOrderId.")
			return existing, true
		}
		if i < attempts-1 {
			select {
			case <-ctx.Done():
				return models.Order{}, false
			case <-time.After(delay):
			}
		}
	}
	return models.Order{}, false
}
