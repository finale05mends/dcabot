package ws

import (
	"dcabot/internal/exchange"
	"dcabot/internal/logger"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	url          string
	apiKey       string
	secret       string
	log          *logger.Logger
	conn         *websocket.Conn
	events       chan exchange.Event
	stopCh       chan struct{}
	stopOnce     sync.Once
	symbol       string
	topics       []string
	reconnectMin time.Duration
	reconnectMax time.Duration
}

type Message struct {
	Topic string          `json:"topic"`
	Type  string          `json:"type"`
	TS    int64           `json:"ts"`
	Data  json.RawMessage `json:"data"`
}

type AuthMessage struct {
	Op   string   `json:"op"`
	Args []string `json:"args"`
}

type SubscribeMessage struct {
	Op   string   `json:"op"`
	Args []string `json:"args"`
}
