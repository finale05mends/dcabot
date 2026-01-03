package rest

import (
	"context"
	"dcabot/internal/exchange"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func (c *Client) GetInstrumentRules(ctx context.Context, symbol string) (exchange.InstrumentRules, error) {
	params := url.Values{}
	params.Set("category", "spot")
	params.Set("symbol", symbol)

	var resp bybitResponse[instrumentInfo]

	if err := c.doRequest(ctx, http.MethodGet, "/v5/market/instruments-info", params, nil, false, &resp); err != nil {
		return exchange.InstrumentRules{}, err
	}

	if len(resp.Result.List) == 0 {
		return exchange.InstrumentRules{}, fmt.Errorf("Торговая пара не найдена: %s", symbol)
	}

	info := resp.Result.List[0]

	tick, err := strconv.ParseFloat(info.PriceFilter.TickSize, 64)
	if err != nil {
		return exchange.InstrumentRules{}, fmt.Errorf("Некорректное значение tickSize=%q: %w", info.PriceFilter.TickSize, err)
	}

	lot, err := parseFloatOrZero(info.LotSizeFilter.QtyStep)
	if err != nil {
		return exchange.InstrumentRules{}, fmt.Errorf("Некорректное значение qtyStep=%q: %w", info.LotSizeFilter.QtyStep, err)
	}

	if lot == 0 {
		lot, err = parseFloatOrZero(info.LotSizeFilter.BasePrecision)
		if err != nil {
			return exchange.InstrumentRules{}, fmt.Errorf("Некорректное значение basePrecision=%q: %w", info.LotSizeFilter.BasePrecision, err)
		}
	}

	if lot == 0 {
		return exchange.InstrumentRules{}, fmt.Errorf("Не удалось определить lot size для торговой пары: %s", symbol)
	}

	minQty, err := strconv.ParseFloat(info.LotSizeFilter.MinOrderQty, 64)
	if err != nil {
		return exchange.InstrumentRules{}, fmt.Errorf("Некорректное значение minOrderQty=%q: %w", info.LotSizeFilter.MinOrderQty, err)
	}

	minNotional, err := strconv.ParseFloat(info.LotSizeFilter.MinOrderAmt, 64)
	if err != nil {
		return exchange.InstrumentRules{}, fmt.Errorf("Некорректное значение minOrderAmt=%q: %w", info.LotSizeFilter.MinOrderAmt, err)
	}

	return exchange.InstrumentRules{
		TickSize:    tick,
		LotSize:     lot,
		MinQty:      minQty,
		MinNotional: minNotional,
		BaseCoin:    info.BaseCoin,
		QuoteCoin:   info.QuoteCoin,
	}, nil
}
