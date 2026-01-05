package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type config struct {
	port           int
	symbol         string
	baseCoin       string
	quoteCoin      string
	tickSize       string
	lotSize        string
	minQty         string
	minNotional    string
	entryPrice     float64
	soFillDelay    time.Duration
	tpFillDelay    time.Duration
	balanceBase    float64
	balanceQuote   float64
	tickerPrice    float64
	tickerInterval time.Duration
	accountType    string
}

type order struct {
	id        string
	linkID    string
	symbol    string
	side      string
	orderType string
	price     float64
	qty       float64
	reduce    bool
}

type fill struct {
	orderID string
	linkID  string
	execID  string
	symbol  string
	side    string
	price   float64
	qty     float64
	ts      int64
	seq     int64
}

type wsClient struct {
	conn   *websocket.Conn
	topics map[string]bool
	mu     sync.Mutex
}

type wsHub struct {
	mu      sync.Mutex
	clients map[*wsClient]struct{}
}

type server struct {
	cfg          config
	mu           sync.Mutex
	orders       map[string]order
	fills        []fill
	nextOrderID  int
	nextExecID   int
	nextSeq      int64
	stage        int
	soScheduled  bool
	tpScheduled  bool
	publicHub    *wsHub
	privateHub   *wsHub
	baseBalance  float64
	quoteBalance float64
}

func main() {
	cfg := loadConfig()
	srv := &server{
		cfg:          cfg,
		orders:       map[string]order{},
		publicHub:    &wsHub{clients: map[*wsClient]struct{}{}},
		privateHub:   &wsHub{clients: map[*wsClient]struct{}{}},
		baseBalance:  cfg.balanceBase,
		quoteBalance: cfg.balanceQuote,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/v5/market/instruments-info", srv.handleInstruments)
	mux.HandleFunc("/v5/order/create", srv.handleCreateOrder)
	mux.HandleFunc("/v5/order/cancel", srv.handleCancelOrder)
	mux.HandleFunc("/v5/order/realtime", srv.handleOpenOrders)
	mux.HandleFunc("/v5/execution/list", srv.handleExecutions)
	mux.HandleFunc("/v5/account/wallet-balance", srv.handleBalances)
	mux.HandleFunc("/ws/public", srv.handleWSPublic)
	mux.HandleFunc("/ws/private", srv.handleWSPrivate)

	go srv.tickerLoop()

	addr := fmt.Sprintf(":%d", cfg.port)
	log.Printf("Мок сервер запущен %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}

func loadConfig() config {
	cfg := config{
		port:           envInt("MOCK_PORT", 18080),
		symbol:         envString("MOCK_SYMBOL", "XRPUSDT"),
		baseCoin:       envString("MOCK_BASE_COIN", "XRP"),
		quoteCoin:      envString("MOCK_QUOTE_COIN", "USDT"),
		tickSize:       envString("MOCK_TICK_SIZE", "0.01"),
		lotSize:        envString("MOCK_LOT_SIZE", "0.01"),
		minQty:         envString("MOCK_MIN_QTY", "0.01"),
		minNotional:    envString("MOCK_MIN_NOTIONAL", "0"),
		entryPrice:     envFloat("MOCK_ENTRY_PRICE", 100),
		soFillDelay:    time.Duration(envInt("MOCK_SO_FILL_AFTER_MS", 300)) * time.Millisecond,
		tpFillDelay:    time.Duration(envInt("MOCK_TP_FILL_AFTER_MS", 300)) * time.Millisecond,
		balanceBase:    envFloat("MOCK_BALANCE_BASE", 0),
		balanceQuote:   envFloat("MOCK_BALANCE_QUOTE", 1000),
		tickerPrice:    envFloat("MOCK_TICKER_PRICE", 100),
		tickerInterval: time.Duration(envInt("MOCK_TICKER_INTERVAL_MS", 1000)) * time.Millisecond,
		accountType:    envString("MOCK_ACCOUNT_TYPE", "UNIFIED"),
	}
	return cfg
}

func envString(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func envInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return parsed
}

func envFloat(key string, fallback float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *server) handleInstruments(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"retCode": 0,
		"retMsg":  "OK",
		"result": map[string]any{
			"list": []any{
				map[string]any{
					"symbol":    s.cfg.symbol,
					"baseCoin":  s.cfg.baseCoin,
					"quoteCoin": s.cfg.quoteCoin,
					"priceFilter": map[string]any{
						"tickSize": s.cfg.tickSize,
					},
					"lotSizeFilter": map[string]any{
						"basePrecision":  s.cfg.lotSize,
						"quotePrecision": "0.00000001",
						"minOrderQty":    s.cfg.minQty,
						"minOrderAmt":    s.cfg.minNotional,
						"qtyStep":        s.cfg.lotSize,
					},
				},
			},
		},
		"time": time.Now().UnixMilli(),
	}
	writeJSON(w, resp)
}

type createOrderRequest struct {
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	OrderType   string `json:"orderType"`
	Qty         string `json:"qty"`
	Price       string `json:"price"`
	TimeInForce string `json:"timeInForce"`
	OrderLinkID string `json:"orderLinkId"`
	MarketUnit  string `json:"marketUnit"`
}

func (s *server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"retCode": 10001, "retMsg": "bad request"})
		return
	}
	s.mu.Lock()
	s.nextOrderID++
	orderID := fmt.Sprintf("order-%d", s.nextOrderID)
	qty := parseFloat(req.Qty)
	price := parseFloat(req.Price)
	if strings.EqualFold(req.OrderType, "Market") && price == 0 {
		price = s.cfg.entryPrice
	}
	order := order{
		id:        orderID,
		linkID:    req.OrderLinkID,
		symbol:    req.Symbol,
		side:      req.Side,
		orderType: req.OrderType,
		price:     price,
		qty:       qty,
	}

	if strings.EqualFold(req.OrderType, "Market") {
		s.nextExecID++
		execID := fmt.Sprintf("exec-%d", s.nextExecID)
		fillQty := qty
		if strings.EqualFold(req.MarketUnit, "quoteCoin") && price > 0 {
			fillQty = qty / price
		}
		s.updateBalances(req.Side, price, fillQty)
		s.nextSeq++
		s.fills = append(s.fills, fill{
			orderID: orderID,
			linkID:  req.OrderLinkID,
			execID:  execID,
			symbol:  req.Symbol,
			side:    req.Side,
			price:   price,
			qty:     fillQty,
			ts:      time.Now().UnixMilli(),
			seq:     s.nextSeq,
		})
		s.mu.Unlock()
		s.broadcastExecution(fill{
			orderID: orderID,
			linkID:  req.OrderLinkID,
			execID:  execID,
			symbol:  req.Symbol,
			side:    req.Side,
			price:   price,
			qty:     fillQty,
			ts:      time.Now().UnixMilli(),
			seq:     s.nextSeq,
		})
	} else {
		s.orders[orderID] = order
		s.mu.Unlock()
		s.scheduleFill(order)
	}

	resp := map[string]any{
		"retCode": 0,
		"retMsg":  "OK",
		"result": map[string]any{
			"orderId": orderID,
		},
		"time": time.Now().UnixMilli(),
	}
	writeJSON(w, resp)
}

type cancelOrderRequest struct {
	OrderID string `json:"orderId"`
	Symbol  string `json:"symbol"`
}

func (s *server) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	var req cancelOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"retCode": 10001, "retMsg": "bad request"})
		return
	}
	s.mu.Lock()
	delete(s.orders, req.OrderID)
	s.mu.Unlock()
	writeJSON(w, map[string]any{
		"retCode": 0,
		"retMsg":  "OK",
		"result":  map[string]any{},
		"time":    time.Now().UnixMilli(),
	})
}

func (s *server) handleOpenOrders(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	list := make([]map[string]any, 0, len(s.orders))
	for _, ord := range s.orders {
		list = append(list, map[string]any{
			"orderId":     ord.id,
			"orderLinkId": ord.linkID,
			"side":        ord.side,
			"orderType":   ord.orderType,
			"price":       fmt.Sprintf("%.8f", ord.price),
			"qty":         fmt.Sprintf("%.8f", ord.qty),
			"leavesQty":   fmt.Sprintf("%.8f", ord.qty),
			"orderStatus": "New",
			"reduceOnly":  ord.reduce,
		})
	}
	s.mu.Unlock()
	writeJSON(w, map[string]any{
		"retCode": 0,
		"retMsg":  "OK",
		"result": map[string]any{
			"list": list,
		},
		"time": time.Now().UnixMilli(),
	})
}

func (s *server) handleExecutions(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	list := make([]map[string]any, 0, len(s.fills))
	for _, f := range s.fills {
		list = append(list, map[string]any{
			"orderId":     f.orderID,
			"orderLinkId": f.linkID,
			"execId":      f.execID,
			"side":        f.side,
			"execPrice":   fmt.Sprintf("%.8f", f.price),
			"execQty":     fmt.Sprintf("%.8f", f.qty),
			"execTime":    fmt.Sprintf("%d", f.ts),
		})
	}
	s.mu.Unlock()
	writeJSON(w, map[string]any{
		"retCode": 0,
		"retMsg":  "OK",
		"result": map[string]any{
			"list": list,
		},
		"time": time.Now().UnixMilli(),
	})
}

func (s *server) handleBalances(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	baseBal := s.baseBalance
	quoteBal := s.quoteBalance
	s.mu.Unlock()
	resp := map[string]any{
		"retCode": 0,
		"retMsg":  "OK",
		"result": map[string]any{
			"list": []any{
				map[string]any{
					"coin": []any{
						map[string]any{
							"coin":                s.cfg.baseCoin,
							"walletBalance":       fmt.Sprintf("%.8f", baseBal),
							"availableToWithdraw": fmt.Sprintf("%.8f", baseBal),
							"availableBalance":    fmt.Sprintf("%.8f", baseBal),
						},
						map[string]any{
							"coin":                s.cfg.quoteCoin,
							"walletBalance":       fmt.Sprintf("%.8f", quoteBal),
							"availableToWithdraw": fmt.Sprintf("%.8f", quoteBal),
							"availableBalance":    fmt.Sprintf("%.8f", quoteBal),
						},
					},
				},
			},
		},
		"time": time.Now().UnixMilli(),
	}
	writeJSON(w, resp)
}

func (s *server) handleWSPublic(w http.ResponseWriter, r *http.Request) {
	s.handleWS(w, r, s.publicHub)
}

func (s *server) handleWSPrivate(w http.ResponseWriter, r *http.Request) {
	s.handleWS(w, r, s.privateHub)
}

func (s *server) handleWS(w http.ResponseWriter, r *http.Request, hub *wsHub) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &wsClient{conn: conn, topics: map[string]bool{}}
	hub.add(client)

	go func() {
		defer hub.remove(client)
		for {
			var msg struct {
				Op   string   `json:"op"`
				Args []string `json:"args"`
			}
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			if msg.Op == "subscribe" {
				client.mu.Lock()
				for _, topic := range msg.Args {
					client.topics[topic] = true
				}
				client.mu.Unlock()
			}
		}
	}()
}

func (h *wsHub) add(client *wsClient) {
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
}

func (h *wsHub) remove(client *wsClient) {
	h.mu.Lock()
	delete(h.clients, client)
	h.mu.Unlock()
	_ = client.conn.Close()
}

func (h *wsHub) broadcast(topic string, payload any, ts int64) {
	h.mu.Lock()
	clients := make([]*wsClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.Unlock()

	msg := map[string]any{
		"topic": topic,
		"type":  "snapshot",
		"ts":    ts,
		"data":  payload,
	}

	for _, client := range clients {
		client.mu.Lock()
		subscribed := client.topics[topic]
		client.mu.Unlock()
		if !subscribed {
			continue
		}
		client.mu.Lock()
		_ = client.conn.WriteJSON(msg)
		client.mu.Unlock()
	}
}

func (s *server) scheduleFill(ord order) {
	if strings.Contains(ord.linkID, "-so-") && !s.soScheduled {
		s.soScheduled = true
		go func(orderID string) {
			time.Sleep(s.cfg.soFillDelay)
			s.fillOrder(orderID)
		}(ord.id)
		return
	}
	if strings.Contains(ord.linkID, "-tp") && s.stage >= 1 && !s.tpScheduled {
		s.tpScheduled = true
		go func(orderID string) {
			time.Sleep(s.cfg.tpFillDelay)
			s.fillOrder(orderID)
		}(ord.id)
	}
}

func (s *server) fillOrder(orderID string) {
	s.mu.Lock()
	ord, ok := s.orders[orderID]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.orders, orderID)
	s.nextExecID++
	s.nextSeq++
	f := fill{
		orderID: ord.id,
		linkID:  ord.linkID,
		execID:  fmt.Sprintf("exec-%d", s.nextExecID),
		symbol:  ord.symbol,
		side:    ord.side,
		price:   ord.price,
		qty:     ord.qty,
		ts:      time.Now().UnixMilli(),
		seq:     s.nextSeq,
	}
	s.fills = append(s.fills, f)
	if strings.Contains(ord.linkID, "-so-") {
		s.stage = 1
	}
	if strings.Contains(ord.linkID, "-tp") {
		s.stage = 2
	}
	s.updateBalances(ord.side, ord.price, ord.qty)
	s.mu.Unlock()

	s.broadcastExecution(f)
}

func (s *server) broadcastExecution(f fill) {
	payload := []map[string]any{
		{
			"orderId":     f.orderID,
			"orderLinkId": f.linkID,
			"execId":      f.execID,
			"symbol":      f.symbol,
			"side":        f.side,
			"execPrice":   fmt.Sprintf("%.8f", f.price),
			"execQty":     fmt.Sprintf("%.8f", f.qty),
			"execTime":    fmt.Sprintf("%d", f.ts),
			"seq":         f.seq,
		},
	}
	s.privateHub.broadcast("execution", payload, f.ts)
}

func (s *server) tickerLoop() {
	if s.cfg.tickerInterval <= 0 {
		return
	}
	ticker := time.NewTicker(s.cfg.tickerInterval)
	defer ticker.Stop()
	for range ticker.C {
		payload := []map[string]any{
			{
				"symbol":    s.cfg.symbol,
				"lastPrice": fmt.Sprintf("%.8f", s.cfg.tickerPrice),
				"seq":       time.Now().UnixMilli(),
				"ts":        time.Now().UnixMilli(),
			},
		}
		s.publicHub.broadcast("tickers."+s.cfg.symbol, payload, time.Now().UnixMilli())
	}
}

func (s *server) updateBalances(side string, price, qty float64) {
	if qty <= 0 || price <= 0 {
		return
	}
	switch strings.ToUpper(side) {
	case "BUY":
		s.baseBalance += qty
		s.quoteBalance -= qty * price
	case "SELL":
		s.baseBalance -= qty
		s.quoteBalance += qty * price
	}
	if s.baseBalance < 0 {
		s.baseBalance = 0
	}
	if s.quoteBalance < 0 {
		s.quoteBalance = 0
	}
}

func parseFloat(val string) float64 {
	if val == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
