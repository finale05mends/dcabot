package ws

import (
	"context"
)

func (w *Client) SubscribeToTopics(ctx context.Context, symbol string, topics []string) error {
	w.symbol = symbol
	w.topics = topics

	msg := SubscribeMessage{
		Op:   "subscribe",
		Args: topics,
	}

	return w.conn.WriteJSON(msg)
}
