package bybit

import (
	"dcabot/internal/logger"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	wsUrl   string
	apiKey  string
	secret  string

	httpClient *http.Client
	log        *logger.Logger
}

func New(baseURL, wsUrl, apiKey, secret string, log *logger.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		wsUrl:   wsUrl,
		apiKey:  apiKey,
		secret:  secret,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		log: log,
	}
}
