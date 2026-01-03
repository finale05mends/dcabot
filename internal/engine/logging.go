package engine

import (
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

func (e *Engine) logEntry() *logrus.Entry {
	entry := e.log.WithComponent("engine")
	if e.cfg != nil && e.cfg.Bot.Symbol != "" {
		entry = entry.WithField("symbol", e.cfg.Bot.Symbol)
	}
	return entry
}

func formatFloatPlain(val float64) string {
	formatted := strconv.FormatFloat(val, 'f', 12, 64)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	if formatted == "" || formatted == "-0" {
		return "0"
	}
	return formatted
}
