package engine

import (
	"context"
	"dcabot/internal/config"
	"dcabot/internal/exchange"
	"dcabot/internal/logger"
	"dcabot/internal/store"
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
	store              store.Store
}

func New(cfg *config.Config, client exchange.Client, log *logger.Logger, store store.Store) *Engine {
	return &Engine{
		cfg:    cfg,
		client: client,
		log:    log,
		state:  DealState{},
		store:  store,
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

	if e.cfg.Runtime.DryRun {
		return e.runDry(ctx)
	}

	restoredFromStore := false
	if e.cfg.Runtime.RestoreStateOnStart {
		loaded, err := e.loadState(ctx)
		if err != nil {
			return err
		}
		restoredFromStore = loaded
		if restoredFromStore {
			e.logEntry().Info("Состояние сделки восстановлено из БД.")
		}
	}

	events, err := e.client.Subscribe(ctx, e.cfg.Bot.Symbol)
	if err != nil {
		return err
	}

	go e.handleEvents(ctx, events)
	e.startMetricsUpdater(ctx)

	restored := false
	if restoredFromStore {
		if err := e.syncOpenOrders(ctx); err != nil {
			e.logEntry().WithError(err).Warn("Не удалось сверить ордера после восстановления из БД.")
		}
		if err := e.ensureTPMatchesAverage(ctx); err != nil {
			return err
		}
		restored = true
	} else {
		var err error
		restored, err = e.restoreActiveOrders(ctx)
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
	}

	return nil
}
