package engine

import (
	"context"
	"dcabot/internal/config"
	"dcabot/internal/exchange"
	"dcabot/internal/logger"
	"fmt"
	"math"
	"strings"
	"time"
)

type Engine struct {
	cfg    *config.Config
	client exchange.Client
	log    *logger.Logger

	rules exchange.InstrumentRules
}

func New(cfg *config.Config, client exchange.Client, log *logger.Logger) *Engine {
	return &Engine{
		cfg:    cfg,
		client: client,
		log:    log,
	}
}

func (e *Engine) Start(ctx context.Context) error {
	fmt.Println("Start запущен")
	rules, err := e.withRetryRules(ctx, e.cfg.Bot.Symbol)
	if err != nil {
		return err
	}
	e.rules = rules
	e.log.Info(fmt.Sprintf("Получены ограничения торговой пары: %+v", e.rules))

	//TODO: добавить логику бота

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
		e.log.Info("Ошибка. Повторяем запрос")
		select {
		case <-ctx.Done():
			return exchange.InstrumentRules{}, ctx.Err()
		case <-time.After(wait):
		}
		reconnect *= 2
	}
	return exchange.InstrumentRules{}, lastErr
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Превышен лимит запросов.") || strings.Contains(msg, "429") || strings.Contains(msg, "10006")
}
