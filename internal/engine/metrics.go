package engine

import (
	"context"
	"dcabot/internal/metrics"
	"time"
)

func (e *Engine) startMetricsUpdater(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.updateAgeMetrics()
			}
		}
	}()
}

func (e *Engine) updateAgeMetrics() {
	e.mu.Lock()
	lastTickerAt := e.state.LastTicker.Timestamp
	lastFillAt := e.state.LastFillAt
	symbol := e.state.Symbol
	side := e.state.Side
	e.mu.Unlock()

	if symbol == "" {
		symbol = e.cfg.Bot.Symbol
	}
	sideLabel := string(side)

	if !lastTickerAt.IsZero() {
		metrics.M.LastTickerAge.WithLabelValues(symbol).Set(time.Since(lastTickerAt).Seconds())
	} else {
		metrics.M.LastTickerAge.WithLabelValues(symbol).Set(0)
	}

	if !lastFillAt.IsZero() {
		metrics.M.LastFillAge.WithLabelValues(symbol, sideLabel).Set(time.Since(lastFillAt).Seconds())
	} else {
		metrics.M.LastFillAge.WithLabelValues(symbol, sideLabel).Set(0)
	}
}
