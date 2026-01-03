package rest

import (
	"dcabot/internal/logger"
	"net/http"
	"time"
)

func New(baseURL, apiKey, secret, accountType string, log *logger.Logger) *Client {
	return &Client{
		baseURL:     baseURL,
		accountType: accountType,
		apiKey:      apiKey,
		secret:      secret,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		log: log,
	}
}
