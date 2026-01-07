package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Exchange ExchangeConfig `mapstructure:"exchange"`
	Bot      BotConfig      `mapstructure:"bot"`
	Runtime  RuntimeConfig  `mapstructure:"runtime"`
}

type ExchangeConfig struct {
	BaseUrl      string `mapstructure:"base_url"`
	WSPublicURL  string `mapstructure:"ws_public_url"`
	WSPrivateURL string `mapstructure:"ws_private_url"`
	AccountType  string `mapstructure:"account_type"`
	ApiKey       string `mapstructure:"api_key"`
	Secret       string `mapstructure:"secret"`
}

type BotConfig struct {
	Symbol           string  `mapstructure:"symbol"`
	Side             string  `mapstructure:"side"`
	BaseOrderQty     float64 `mapstructure:"base_order_qty"`
	QtyUnit          string  `mapstructure:"qty_unit"`
	TPPercent        float64 `mapstructure:"tp_percent"`
	SOCount          int     `mapstructure:"so_count"`
	SOStepPercent    float64 `mapstructure:"so_step_percent"`
	SOStepMultiplier float64 `mapstructure:"so_step_multiplier"`
	SOBaseQty        float64 `mapstructure:"so_base_qty"`
	SOQtyMultiplier  float64 `mapstructure:"so_qty_multiplier"`
}

type RuntimeConfig struct {
	DryRun              bool     `mapstructure:"dry_run"`
	RestoreStateOnStart bool     `mapstructure:"restore_state_on_start"`
	Log                 LogCfg   `mapstructure:"log"`
	Store               StoreCfg `mapstructure:"store"`
}

type LogCfg struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

type StoreCfg struct {
	Path   string `mapstructure:"path"`
	Bucket string `mapstructure:"bucket"`
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

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Exchange.AccountType == "" {
		c.Exchange.AccountType = "UNIFIED"
	}

	if c.Bot.QtyUnit == "" {
		c.Bot.QtyUnit = "baseCoin"
	}

	if c.Runtime.Log.Level == "" {
		c.Runtime.Log.Level = "info"
	}
	if c.Runtime.Log.Format == "" {
		c.Runtime.Log.Format = "text"
	}
	if c.Runtime.Store.Path != "" {
		c.Runtime.Store.Bucket = strings.ToLower(c.Bot.Symbol) + "_" + strings.ToLower(c.Bot.Side)
	}

	if c.Exchange.BaseUrl == "" {
		return errors.New("Указание url API обязательно (exchange.base_url).")
	}
	if c.Exchange.WSPublicURL == "" {
		return errors.New("Указание url ws public API обязательно (exchange.ws_public_url).")
	}
	if c.Exchange.WSPublicURL == "" {
		return errors.New("Указание url ws private API обязательно (exchange.ws_private_url).")
	}
	if c.Exchange.ApiKey == "" {
		return errors.New("Переменная окружения не задана (exchange.api_key).")
	}
	if c.Exchange.Secret == "" {
		return errors.New("Переменная окружения не задана (exchange.secret).")
	}
	if c.Bot.Symbol == "" {
		return errors.New("Указание торговой пары обязательно (bot.symbol)")
	}

	if c.Bot.Side != "BUY" && c.Bot.Side != "SELL" {
		return errors.New("Возможные направления ордера только \"BUY\" или \"SELL\" (bot.side).")
	}
	if c.Bot.BaseOrderQty <= 0 {
		return errors.New("Объём входного маркет ордера должен быть > 0 (bot.base_order_qty).")
	}
	if c.Bot.TPPercent <= 0 {
		return errors.New("Процент тейк-профита должен быть > 0 (bot.tp_percent).")
	}
	if c.Bot.SOCount <= 0 {
		return errors.New("Количество страховочных ордеров должно быть > 0 (bot.so_count).")
	}
	if c.Bot.SOStepPercent <= 0 {
		return errors.New("Процентный шаг первого страховочного ордера должен быть > 0 (bot.so_step_percent).")
	}
	if c.Bot.SOStepMultiplier < 1 || c.Bot.SOStepMultiplier > 2 {
		return errors.New("Множитлеь шага должен быть в диапазоне 1.0-2.0 (bot.so_step_multiplier).")
	}
	if c.Bot.SOBaseQty <= 0 {
		return errors.New("Объём первого страховочного ордера должен быть > 0 (bot.so_base_qty).")
	}
	if c.Bot.SOQtyMultiplier < 1 || c.Bot.SOQtyMultiplier > 2 {
		return errors.New("Множитель объёма должен быть в диапазоне 1.0-2.0 (bot.so_qty_multiplier).")
	}

	return nil
}
