package main

import (
	"context"
	"dcabot/internal/config"
	"dcabot/internal/engine"
	"dcabot/internal/exchange/bybit"
	"dcabot/internal/logger"
	"os"
	"os/signal"
	"syscall"
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

	client := bybit.New(cfg.Exchange.BaseUrl, cfg.Exchange.WSPublicURL, cfg.Exchange.WSPrivateURL, cfg.Exchange.AccountType, cfg.Exchange.ApiKey, cfg.Exchange.Secret, logger)
	eng := engine.New(cfg, client, logger)
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
