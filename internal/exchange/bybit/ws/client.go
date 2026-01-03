package ws

import (
	"context"
	"dcabot/internal/exchange"
	"dcabot/internal/logger"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func New(url, apiKey, secret string, log *logger.Logger) (*Client, error) {
	return &Client{
		url:          url,
		apiKey:       apiKey,
		secret:       secret,
		log:          log,
		events:       make(chan exchange.Event, 100),
		stopCh:       make(chan struct{}),
		reconnectMin: 1 * time.Second,
		reconnectMax: 30 * time.Second,
	}, nil
}

func (w *Client) Connect(ctx context.Context) error {
	w.logEntry().WithField("url", w.url).Info("Подключение к WS.")

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, w.url, nil)
	if err != nil {
		return fmt.Errorf("Не удалось подключиться к WS: %w", err)
	}

	w.conn = conn
	w.conn.SetReadLimit(2 << 20)

	if w.apiKey != "" && w.secret != "" {
		if err := w.authenticate(); err != nil {
			return err
		}
	}

	w.logEntry().Info("WS соединение установлено.")

	go w.readLoop()

	return nil
}

func (w *Client) logEntry() *logrus.Entry {
	entry := w.log.WithComponent("bybit_ws")
	if w.symbol != "" {
		entry = entry.WithField("symbol", w.symbol)
	}
	return entry
}

func (w *Client) Events() <-chan exchange.Event {
	return w.events
}
