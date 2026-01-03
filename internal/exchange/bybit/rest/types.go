package rest

import (
	"dcabot/internal/exchange/bybit/ws"
	"dcabot/internal/logger"
	"net/http"
)

type Client struct {
	baseURL      string
	wsPublicURL  string
	wsPrivateURL string
	accountType  string
	apiKey       string
	secret       string
	httpClient   *http.Client
	log          *logger.Logger
	wsPublic     *ws.Client
	wsPrivate    *ws.Client
}

type bybitResponse[T any] struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  T      `json:"result"`
	Time    int64  `json:"time"`
}

type instrumentInfo struct {
	List []struct {
		Symbol      string `json:"symbol"`
		BaseCoin    string `json:"baseCoin"`
		QuoteCoin   string `json:"quoteCoin"`
		PriceFilter struct {
			TickSize string `json:"tickSize"`
		} `json:"priceFilter"`
		LotSizeFilter struct {
			BasePrecision  string `json:"basePrecision"`
			QuotePrecision string `json:"quotePrecision"`
			MinOrderQty    string `json:"minOrderQty"`
			MinOrderAmt    string `json:"minOrderAmt"`
			QtyStep        string `json:"qtyStep"`
		} `json:"lotSizeFilter"`
	} `json:"list"`
}
