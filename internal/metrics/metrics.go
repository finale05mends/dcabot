package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	LabelSymbol      = "symbol"
	LabelSide        = "side"
	LabelOrderKind   = "kind"
	LabelOrderType   = "type"
	LabelCoin        = "coin"
	LabelReason      = "reason"
	LabelEndpoint    = "endpoint"
	LabelMethod      = "method"
	LabelMessageType = "type"
)

type Metrics struct {
	DealActive         *prometheus.GaugeVec
	EntryPrice         *prometheus.GaugeVec
	AvgPrice           *prometheus.GaugeVec
	TotalQty           *prometheus.GaugeVec
	TPPrice            *prometheus.GaugeVec
	TPQty              *prometheus.GaugeVec
	CurrentPrice       *prometheus.GaugeVec
	UnrealizedPnL      *prometheus.GaugeVec
	LastTickerAge      *prometheus.GaugeVec
	LastFillAge        *prometheus.GaugeVec
	OrdersPlaced       *prometheus.CounterVec
	OrdersFilled       *prometheus.CounterVec
	OrdersCancelled    *prometheus.CounterVec
	OrdersFailed       *prometheus.CounterVec
	APIRequests        *prometheus.CounterVec
	APILatency         *prometheus.HistogramVec
	APIErrors          *prometheus.CounterVec
	APIRateLimitHits   *prometheus.CounterVec
	WSReconnects       prometheus.Counter
	WSMessagesReceived *prometheus.CounterVec
	WSParseErrors      prometheus.Counter
	WSConnectionAge    prometheus.Gauge
	BalanceAvailable   *prometheus.GaugeVec
	BalanceWallet      *prometheus.GaugeVec
	EngineCloseRequest *prometheus.CounterVec
}

var (
	M    Metrics
	once sync.Once
)

func Init() {
	once.Do(func() {
		M.DealActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "deal_active",
			Help:      "Флаг активности сделки (1 — активна, 0 — неактивна).",
		}, []string{LabelSymbol, LabelSide})
		M.EntryPrice = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "entry_price",
			Help:      "Цена входа в сделку.",
		}, []string{LabelSymbol, LabelSide})
		M.AvgPrice = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "position_avg_price",
			Help:      "Средняя цена позиции.",
		}, []string{LabelSymbol, LabelSide})
		M.TotalQty = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "position_total_qty",
			Help:      "Общий объём позиции.",
		}, []string{LabelSymbol, LabelSide})
		M.TPPrice = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "tp_price",
			Help:      "Текущая цена TP.",
		}, []string{LabelSymbol, LabelSide})
		M.TPQty = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "tp_qty",
			Help:      "Текущий объём TP.",
		}, []string{LabelSymbol, LabelSide})
		M.CurrentPrice = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "current_price",
			Help:      "Последняя цена из тикера.",
		}, []string{LabelSymbol})
		M.UnrealizedPnL = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "unrealized_pnl_usd",
			Help:      "Нереализованная прибыль/убыток.",
		}, []string{LabelSymbol, LabelSide})
		M.LastTickerAge = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "last_ticker_age_seconds",
			Help:      "Количество секунд с момента последнего события тикера.",
		}, []string{LabelSymbol})
		M.LastFillAge = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "last_fill_age_seconds",
			Help:      "Количество секунд с последнего исполнения ордера.",
		}, []string{LabelSymbol, LabelSide})
		M.OrdersPlaced = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "dcabot",
			Name:      "orders_placed_total",
			Help:      "Общее количество размещённых ордеров.",
		}, []string{LabelSymbol, LabelSide, LabelOrderKind, LabelOrderType})
		M.OrdersFilled = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "dcabot",
			Name:      "orders_filled_total",
			Help:      "Общее количество полностью исполненных ордеров.",
		}, []string{LabelSymbol, LabelSide, LabelOrderKind, LabelOrderType})
		M.OrdersCancelled = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "dcabot",
			Name:      "orders_cancelled_total",
			Help:      "Общее количество отменённых ордеров.",
		}, []string{LabelSymbol, LabelSide, LabelOrderKind, LabelOrderType})
		M.OrdersFailed = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "dcabot",
			Name:      "orders_failed_total",
			Help:      "Общее количество неудачных операций.",
		}, []string{LabelSymbol, LabelSide, LabelOrderKind, LabelOrderType})
		M.APIRequests = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "dcabot",
			Name:      "api_requests_total",
			Help:      "Общее количество API-запросов к бирже.",
		}, []string{LabelEndpoint, LabelMethod})
		M.APILatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "dcabot",
			Name:      "api_latency_seconds",
			Help:      "Задержка API-запросов в секундах.",
			Buckets:   prometheus.DefBuckets,
		}, []string{LabelEndpoint, LabelMethod})
		M.APIErrors = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "dcabot",
			Name:      "api_errors_total",
			Help:      "Общее количество ошибок API.",
		}, []string{LabelEndpoint, LabelMethod})
		M.APIRateLimitHits = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "dcabot",
			Name:      "api_rate_limit_hits_total",
			Help:      "Общее количество срабатываний лимитов API.",
		}, []string{LabelEndpoint, LabelMethod})
		M.WSReconnects = promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "dcabot",
			Name:      "ws_reconnects_total",
			Help:      "Общее количество переподключений WS.",
		})
		M.WSMessagesReceived = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "dcabot",
			Name:      "ws_messages_received_total",
			Help:      "Общее количество полученных сообщений WS.",
		}, []string{LabelMessageType})
		M.WSParseErrors = promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "dcabot",
			Name:      "ws_parse_errors_total",
			Help:      "Общее количество ошибок парсинга WebSocket.",
		})
		M.WSConnectionAge = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "ws_connection_age_seconds",
			Help:      "Время жизни текущего WS-соединения в секундах.",
		})
		M.BalanceAvailable = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "balance_available",
			Help:      "Доступный баланс.",
		}, []string{LabelCoin})
		M.BalanceWallet = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "dcabot",
			Name:      "balance_wallet",
			Help:      "Общий баланс.",
		}, []string{LabelCoin})
		M.EngineCloseRequest = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "dcabot",
			Name:      "engine_close_requested_total",
			Help:      "Количество запросов на закрытие цикла сделки с \"причиной\".",
		}, []string{LabelSymbol, LabelSide, LabelReason})
	})
}

func init() {
	Init()
}
