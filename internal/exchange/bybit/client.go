package bybit

import (
	"context"
	"dcabot/internal/config"
	"dcabot/internal/exchange"
	"dcabot/internal/exchange/bybit/rest"
	"dcabot/internal/exchange/bybit/ws"
	"dcabot/internal/logger"
	"dcabot/internal/models"
	"errors"
)

type Client struct {
	cfg         *config.Config
	rest        *rest.Client
	wsPublic    *ws.Client
	wsPrivate   *ws.Client
	wsConnected bool
	log         *logger.Logger
}

func New(cfg *config.Config, log *logger.Logger) *Client {
	return &Client{

		rest:      rest.New(cfg.Exchange.BaseUrl, cfg.Exchange.ApiKey, cfg.Exchange.Secret, cfg.Exchange.AccountType, log),
		wsPublic:  newWSClient(cfg.Exchange.WSPublicURL, "", "", log),
		wsPrivate: newWSClient(cfg.Exchange.WSPrivateURL, cfg.Exchange.ApiKey, cfg.Exchange.Secret, log),
		log:       log,
	}
}

func newWSClient(url, apiKey, secret string, log *logger.Logger) *ws.Client {
	client, _ := ws.New(url, apiKey, secret, log)
	return client
}

func (c *Client) GetInstrumentRules(ctx context.Context, symbol string) (exchange.InstrumentRules, error) {
	return c.rest.GetInstrumentRules(ctx, symbol)
}

func (c *Client) Subscribe(ctx context.Context, symbol string) (<-chan exchange.Event, error) {
	c.log.WithSymbol(symbol).WithField("component", "bybit").Info("Подписываемся на торговую парую")

	if c.wsConnected {
		return nil, errors.New("Подписка уже создана.")
	}

	if err := c.wsPublic.Connect(ctx); err != nil {
		return nil, err
	}

	if err := c.wsPrivate.Connect(ctx); err != nil {
		return nil, err
	}

	if err := c.wsPublic.SubscribeToTopics(ctx, symbol, []string{
		"tickers." + symbol,
	}); err != nil {
		return nil, err
	}

	if err := c.wsPrivate.SubscribeToTopics(ctx, symbol, []string{
		"order",
		"execution",
	}); err != nil {
		return nil, err
	}

	merged := make(chan exchange.Event, 200)

	c.log.WithSymbol(symbol).WithField("component", "bybit").Debug("Запуск forwardEvents.")
	go forwardEvents(c.wsPublic.Events(), merged)
	go forwardEvents(c.wsPrivate.Events(), merged)

	c.wsConnected = true

	c.log.WithSymbol(symbol).WithField("component", "bybit").Info("Подписки активированы.")

	return merged, nil
}

func (c *Client) CancelOrder(ctx context.Context, symbol, orderID string) error {
	return c.rest.CancelOrder(ctx, symbol, orderID)
}

func (c *Client) GetOpenOrders(ctx context.Context, symbol string) ([]models.Order, error) {
	return c.rest.GetOpenOrders(ctx, symbol)
}

func (c *Client) PlaceOrder(ctx context.Context, order models.Order) (models.Order, error) {
	return c.rest.PlaceOrder(ctx, order)
}

func (c *Client) GetFills(ctx context.Context, symbol string) ([]models.Fill, error) {
	return c.rest.GetFills(ctx, symbol)
}

func (c *Client) GetBalances(ctx context.Context, coins []string) (map[string]exchange.Balance, error) {
	return c.rest.GetBalances(ctx, coins)
}

func forwardEvents(src <-chan exchange.Event, dst chan<- exchange.Event) {
	for event := range src {
		dst <- event
	}
}
