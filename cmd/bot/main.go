package main

import (
	"context"
	"dcabot/internal/config"
	"dcabot/internal/engine"
	"dcabot/internal/exchange/bybit"
	"dcabot/internal/logger"
	"dcabot/internal/store"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := logger.New(logger.Config{
		Level:      cfg.Runtime.Log.Level,
		Format:     cfg.Runtime.Log.Format,
		Output:     cfg.Runtime.Log.File,
		MaxSize:    cfg.Runtime.Log.MaxSize,
		MaxBackups: cfg.Runtime.Log.MaxBackups,
		MaxAge:     cfg.Runtime.Log.MaxAge,
		Compress:   cfg.Runtime.Log.Compress,
	})

	logger.Info("Бот запущен.")

	go func() {
		addr := ":2112"
		http.Handle("/metrics", promhttp.Handler())
		logger.Info("endpoint метрик запущен.")
		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.WithError(err).Error("Ошибка сервера метрик.")
		}
	}()

	var st store.Store
	if cfg.Runtime.Store.Path != "" {
		opened, err := store.NewBoltStore(cfg.Runtime.Store.Path, cfg.Runtime.Store.Bucket)
		if err != nil {
			logger.WithError(err).Fatal("Не удалось открыть БД.")
		}
		st = opened
		logger.WithFields(map[string]interface{}{
			"path":   cfg.Runtime.Store.Path,
			"bucket": cfg.Runtime.Store.Bucket,
		}).Info("БД включена.")
		defer func() {
			if err := st.Close(); err != nil {
				logger.WithError(err).Warn("Не удалось закрыть БД.")
			}
		}()
	}

	client := bybit.New(cfg, logger)
	eng := engine.New(cfg, client, logger, st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := eng.Start(ctx); err != nil {
			logger.WithError(err).Fatal("\"Двигатель\" завершился с ошибкой.")
		}
	}()
	<-sigCh

	cancel()

	logger.Info("Бот остановлен.")
}
