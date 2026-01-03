package rest

import (
	"context"
	"dcabot/internal/exchange"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (c *Client) GetBalances(ctx context.Context, coins []string) (map[string]exchange.Balance, error) {
	params := url.Values{}
	params.Set("accountType", c.accountType)

	if len(coins) > 0 {
		params.Set("coin", strings.Join(coins, ","))
	}

	var resp bybitResponse[struct {
		List []struct {
			Coin []struct {
				Coin                string `json:"coin"`
				WalletBalance       string `json:"walletBalance"`
				AvailableToWithdraw string `json:"availableToWithdraw"`
				AvailableBalance    string `json:"availableBalance"`
			} `json:"coin"`
		} `json:"list"`
	}]

	if err := c.doRequest(ctx, http.MethodGet, "/v5/account/wallet-balance", params, nil, true, &resp); err != nil {
		return nil, err
	}

	balances := map[string]exchange.Balance{}
	for _, account := range resp.Result.List {
		for _, item := range account.Coin {
			wallet, _ := parseFloatOrZero(item.WalletBalance)

			available, _ := parseFloatOrZero(item.AvailableToWithdraw)
			if available == 0 {
				available, _ = parseFloatOrZero(item.AvailableBalance)
			}
			if available == 0 {
				available = wallet
			}

			balances[item.Coin] = exchange.Balance{
				Coin:      item.Coin,
				Wallet:    wallet,
				Available: available,
			}
		}
	}
	return balances, nil
}

func parseFloatOrZero(value string) (float64, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}
