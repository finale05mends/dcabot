package engine

import (
	"context"
	"dcabot/internal/config"
	"dcabot/internal/exchange"
	"dcabot/internal/logger"
	"math"
	"sync"
	"time"
)

type Engine struct {
	cfg                *config.Config
	client             exchange.Client
	log                *logger.Logger
	rules              exchange.InstrumentRules
	tpSeq              int64
	mu                 sync.Mutex
	state              DealState
	lastTickerLog      time.Time
	tpRebuildScheduled bool
	tpRebuildAt        time.Time
}

func New(cfg *config.Config, client exchange.Client, log *logger.Logger) *Engine {
	return &Engine{
		cfg:    cfg,
		client: client,
		log:    log,
		state:  DealState{},
	}
}

func (e *Engine) Start(ctx context.Context) error {
	e.logEntry().Debug("Start запущен.")

	rules, err := e.withRetryRules(ctx, e.cfg.Bot.Symbol)
	if err != nil {
		return err
	}
	e.rules = rules
	e.logEntry().WithFields(map[string]interface{}{
		"rules_tick_size":    formatFloatPlain(e.rules.TickSize),
		"rules_lot_size":     formatFloatPlain(e.rules.LotSize),
		"rules_min_qty":      formatFloatPlain(e.rules.MinQty),
		"rules_min_notional": formatFloatPlain(e.rules.MinNotional),
		"rules_base":         e.rules.BaseCoin,
		"rules_quote":        e.rules.QuoteCoin,
	}).Info("Получены ограничения торговой пары.")

	events, err := e.client.Subscribe(ctx, e.cfg.Bot.Symbol)
	if err != nil {
		return err
	}

	go e.handleEvents(ctx, events)

	restored, err := e.restoreActiveOrders(ctx)
	if err != nil {
		return err
	}
	if restored {
		e.logEntry().Info("Восстановлены активные ордера после рестарта, новый вход не нужен.")
	}

	if !restored && !e.state.Active {
		if err := e.openDeal(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) withRetryRules(ctx context.Context, symbol string) (exchange.InstrumentRules, error) {
	var lastErr error
	var reconnect time.Duration = 1 * time.Second
	for i := 0; i < 5; i++ {
		rules, err := e.client.GetInstrumentRules(ctx, symbol)
		if err == nil {
			return rules, nil
		}
		lastErr = err
		wait := time.Duration(math.Min(float64(reconnect), float64(reconnect*30)))
		if isRateLimitError(err) {
			wait = time.Duration(math.Min(float64(reconnect*4), float64(reconnect*30)))
		}
		e.logEntry().WithError(lastErr).Warn("Ошибка. Повторяем запрос.")
		select {
		case <-ctx.Done():
			return exchange.InstrumentRules{}, ctx.Err()
		case <-time.After(wait):
		}
		reconnect *= 2
	}
	return exchange.InstrumentRules{}, lastErr
}
