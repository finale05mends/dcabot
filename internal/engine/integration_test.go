//go:build integration

package engine

import (
	"context"
	"dcabot/internal/config"
	"dcabot/internal/exchange/bybit"
	"dcabot/internal/logger"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForHealth(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Мок сервер недопступен по адресу: %s", url)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Не удалось получить текущую рабочую директорию: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("файл go.mod не найден %s", dir)
		}
		dir = parent
	}
}

func waitForIntegration(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("Условие не было выполнено в течение %s", timeout)
}

func TestIntegrationHappyPathServer(t *testing.T) {
	port := freePort(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	root := repoRoot(t)
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/mock_bybit")
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("MOCK_PORT=%d", port),
		"MOCK_SYMBOL=XRPUSDT",
		"MOCK_ENTRY_PRICE=100",
		"MOCK_BASE_COIN=XRP",
		"MOCK_QUOTE_COIN=USDT",
		"MOCK_TICK_SIZE=0.01",
		"MOCK_LOT_SIZE=0.01",
		"MOCK_MIN_QTY=0.01",
		"MOCK_MIN_NOTIONAL=0",
		"MOCK_SO_FILL_AFTER_MS=300",
		"MOCK_TP_FILL_AFTER_MS=300",
		"MOCK_BALANCE_BASE=0",
		"MOCK_BALANCE_QUOTE=1000",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("Ошибка запуска мок сервера: %v", err)
	}
	t.Cleanup(func() {
		stopProcess(t, cmd, 2*time.Second)
	})

	waitForHealth(t, fmt.Sprintf("http://127.0.0.1:%d/health", port), 3*time.Second)

	cfg := &config.Config{
		Exchange: config.ExchangeConfig{
			BaseUrl:      fmt.Sprintf("http://127.0.0.1:%d", port),
			WSPublicURL:  fmt.Sprintf("ws://127.0.0.1:%d/ws/public", port),
			WSPrivateURL: fmt.Sprintf("ws://127.0.0.1:%d/ws/private", port),
			AccountType:  "UNIFIED",
			ApiKey:       "test-key",
			Secret:       "test-secret",
		},
		Bot: config.BotConfig{
			Symbol:           "XRPUSDT",
			Side:             "BUY",
			BaseOrderQty:     10,
			QtyUnit:          "baseCoin",
			TPPercent:        1,
			SOCount:          2,
			SOStepPercent:    1,
			SOStepMultiplier: 1,
			SOBaseQty:        5,
			SOQtyMultiplier:  1,
		},
		Runtime: config.RuntimeConfig{
			DryRun: false,
			Log: config.LogCfg{
				Level:  "error",
				Format: "text",
			},
		},
	}

	log := logger.New(logger.Config{Level: "error", Format: "text", Output: "stdout"})
	client := bybit.New(cfg, log)
	eng := New(cfg, client, log, nil)

	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Не удалось запустить \"Двигатель\": %v", err)
	}

	waitForIntegration(t, 8*time.Second, func() bool {
		eng.mu.Lock()
		defer eng.mu.Unlock()
		return !eng.state.Active && !eng.state.Closing
	})

	openOrders, err := client.GetOpenOrders(context.Background(), cfg.Bot.Symbol)
	if err != nil {
		t.Fatalf("Ошибка получения ордеров: %v", err)
	}
	if len(openOrders) != 0 {
		t.Fatalf("Ожидалось отсутствие ордеров, но получено %d", len(openOrders))
	}
	cancel()
	stopProcess(t, cmd, 2*time.Second)
}

func stopProcess(t *testing.T, cmd *exec.Cmd, timeout time.Duration) {
	t.Helper()
	if cmd == nil || cmd.Process == nil {
		return
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return
	}
	pgid := cmd.Process.Pid
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(timeout):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		_ = cmd.Wait()
	case <-done:
	}
}
