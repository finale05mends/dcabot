package ws

import (
	"context"
	"dcabot/internal/exchange"
	"encoding/json"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func (w *Client) readLoop() {
	w.logEntry().Debug("readLoop запущен.")

	for {
		select {
		case <-w.stopCh:
			return
		default:
		}
		_, data, err := w.conn.ReadMessage()
		if err != nil {
			w.logEntry().WithError(err).Warn("Ошибка чтения WS.")

			if !w.reconnect() {
				return
			}
			continue
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			w.logEntry().WithError(err).Warn("Не удалось разобрать WS сообщение.")
			continue
		}

		switch {
		case msg.Topic == "execution" || strings.HasPrefix(msg.Topic, "execution"):
			w.handleExecution(msg)
		case msg.Topic == "order" || strings.HasPrefix(msg.Topic, "order"):
			w.handleOrder(msg)
		case strings.HasPrefix(msg.Topic, "tickers"):
			w.handleTicker(msg)
		default:
			continue
		}
	}
}

func (w *Client) reconnect() bool {
	backoff := w.reconnectMin

	for {
		select {
		case <-w.stopCh:
			return false
		default:
		}

		w.logEntry().Info("Попытка переподключения к WS.")

		time.Sleep(backoff)

		conn, _, err := websocket.DefaultDialer.Dial(w.url, nil)
		if err != nil {
			w.logEntry().WithError(err).Warn("Не удалось переподключиться к WS.")
			backoff = w.nextBackoff(backoff)
			continue
		}

		if w.conn != nil {
			_ = w.conn.Close()
		}

		w.conn = conn
		w.conn.SetReadLimit(2 << 20)

		if w.apiKey != "" && w.secret != "" {
			if err := w.authenticate(); err != nil {
				w.logEntry().WithError(err).Warn("Не удалось повторно авторизоваться в WS.")
				backoff = w.nextBackoff(backoff)
				continue
			}
		}

		if w.symbol != "" {
			if err := w.SubscribeToTopics(context.Background(), w.symbol, w.topics); err != nil {
				w.logEntry().WithError(err).Warn("Не удалось повторно подписаться на WS.")
				backoff = w.nextBackoff(backoff)
				continue
			}
		}

		w.events <- exchange.Event{Type: exchange.EventTypeReconnect}
		w.logEntry().Info("WS переподключён и подписки восстановлены.")
		return true
	}
}

func (w *Client) nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > w.reconnectMax {
		return w.reconnectMax
	}
	return next
}
