package config

import (
	"os"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Exchange ExchangeConfig
	Bot      BotConfig
	Runtime  RuntimeConfig
}

type ExchangeConfig struct {
	BaseUrl string
	WSUrl   string
	ApiKey  string
	Secret  string
}

type BotConfig struct {
	Symbol           string
	Side             string
	BaseOrderQty     float64
	TPPercent        float64
	SOCount          int
	SOStepPercent    float64
	SOStepMultiplier float64
	SOBaseQty        float64
	SOQtyMultiplier  float64
}

type RuntimeConfig struct {
	DryRun              bool
	RestoreStateOnStart bool
	LogLevel            string
}

func Load() (*Config, error) {

	cfg := &Config{}
	viper.AddConfigPath("configs")
	viper.SetConfigName("config")
	viper.ReadInConfig()

	cfg.Exchange = ExchangeConfig{
		BaseUrl: viper.GetString("exchange.base_url"),
		WSUrl:   viper.GetString("exchange.ws_url"),
		ApiKey:  envSub("exchange.api_key"),
		Secret:  envSub("exchange.secret"),
	}

	cfg.Bot = BotConfig{
		Symbol:           viper.GetString("bot.symbol"),
		Side:             viper.GetString("bot.side"),
		BaseOrderQty:     viper.GetFloat64("bot.base_order_qty"),
		TPPercent:        viper.GetFloat64("bot.tp_percent"),
		SOCount:          viper.GetInt("bot.so_count"),
		SOStepPercent:    viper.GetFloat64("bot.so_step_percent"),
		SOStepMultiplier: viper.GetFloat64("bot.so_step_multiplier"),
		SOBaseQty:        viper.GetFloat64("bot.so_base_qty"),
		SOQtyMultiplier:  viper.GetFloat64("bot.so_qty_multiplier"),
	}

	cfg.Runtime = RuntimeConfig{
		DryRun:              viper.GetBool("runtime.dry_run"),
		RestoreStateOnStart: viper.GetBool("runtime.restore_state_on_start"),
		LogLevel:            viper.GetString("runtime.log_level"),
	}

	return cfg, nil
}

func envSub(key string) string {
	val := viper.GetString(key)
	if val == "" {
		return ""
	}

	re := regexp.MustCompile(`\$\{(\w+)\}`)
	return re.ReplaceAllStringFunc(val, func(match string) string {
		envKey := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		return os.Getenv(envKey)
	})
}
