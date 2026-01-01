package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Exchange ExchangeConfig `mapstructure:"exchange"`
	Bot      BotConfig      `mapstructure:"bot"`
	Runtime  RuntimeConfig  `mapstructure:"runtime"`
}

type ExchangeConfig struct {
	BaseUrl string `mapstructure:"base_url"`
	WSUrl   string `mapstructure:"ws_url"`
	ApiKey  string `mapstructure:"api_key"`
	Secret  string `mapstructure:"secret"`
}

type BotConfig struct {
	Symbol           string  `mapstructure:"symbol"`
	Side             string  `mapstructure:"side"`
	BaseOrderQty     float64 `mapstructure:"base_order_qty"`
	TPPercent        float64 `mapstructure:"tp_percent"`
	SOCount          int     `mapstructure:"so_count"`
	SOStepPercent    float64 `mapstructure:"so_step_percent"`
	SOStepMultiplier float64 `mapstructure:"so_step_multiplier"`
	SOBaseQty        float64 `mapstructure:"so_base_qty"`
	SOQtyMultiplier  float64 `mapstructure:"so_qty_multiplier"`
}

type RuntimeConfig struct {
	DryRun              bool   `mapstructure:"dry_run"`
	RestoreStateOnStart bool   `mapstructure:"restore_state_on_start"`
	LogLevel            string `mapstructure:"log_level"`
}

func Load() (*Config, error) {

	cfg := &Config{}
	viper.AddConfigPath("configs")
	viper.SetConfigName("config")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("Не удалось прочитать конфиг: %w", err)
	}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("Не удалось разобрать конфиг: %w", err)
	}

	cfg.Exchange.ApiKey = os.ExpandEnv(cfg.Exchange.ApiKey)
	cfg.Exchange.Secret = os.ExpandEnv(cfg.Exchange.Secret)

	return cfg, nil
}
