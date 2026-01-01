package main

import (
	"dcabot/internal/config"
	"dcabot/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := logger.New(cfg.Runtime.LogLevel)

	logger.Info("Бот запущен.")
}
