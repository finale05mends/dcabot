package ws

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func (w *Client) authenticate() error {
	expires := time.Now().UnixMilli() + 5_000
	payload := fmt.Sprintf("GET/realtime%d", expires)

	sign := sign(w.secret, payload)

	msg := AuthMessage{
		Op:   "auth",
		Args: []string{w.apiKey, fmt.Sprintf("%d", expires), sign},
	}

	if err := w.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("Не удалось авторизоваться: %w", err)
	}

	return nil
}

func sign(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
