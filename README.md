# dcabot

MVP DCA-бот

## Быстрый старт

1.) Подготовить конфиг и указать актуальные данные.

2.) Задать переменные окружения:

```sh
export BYBIT_API_KEY=""
export BYBIT_API_SECRET=""
```

3.) Запустить бота:

```sh
go run ./cmd/bot/main.go
```

Логи пишутся либо в stdout, либо в файл, если он указан в runtime.log.file

##Docker TODO

## Кофигурация

Пример находится в configs/config.example.yaml
exchange:
```sh
exchange.base_url #bybit api url.
exchange.ws_public_url #Публичный ws (Тикеры).
exchange.ws_private_url #Приватный ws (Ордера и исполнения).
exchange.account_type #Тип аккаунта UNIFIED/CLASSIC. На Classic аккаунте не тестировалось.
exchange.api_key, excahnge.secret #Ключи. Указывается переменная окружения, откуда подтягивать данные.
```

bot:
```sh
bot.symbol #Указание торговой пары.
bot.side #Направление торгов. Buy/sell.
bot.base_order_qty #Объём входного маркет ордера.
bot.qty_unit #baseCoin/quoteCoint единица измерения ордеров. (И маркет и страховочных).
bot.tp_percent #Процент тейк-профита.
bot.so_count #Количество страховочных ордеров.
bot.so_step_percent #Первый шаг в сетке страховочных ордеров, от цены входного ордера.
bot.so_step_multiplier #Множитель шага для постановки последующих страховочных ордеров. >=1, <=2
bot.so_base_qty #Объём первого страховочного ордера.
bot.so_qty_multiplier #Множитель объйма страховочных ордеров. >=1, <=2.
```

runtime
```sh
runtime.dry_run #Режим без постановки реальных заявок. true/false //В процессе.
runtime.restore_state_on_start #Восстанавливать состояние после рестарта. true/false.
runtime.log.level #Уровень логирования. debug/info/warn/error/fatal/panic. По умолчанию "info".
runtime.log.format #Формат вывода логов. text/json.
runtime.log.file #Путь к файлу логов. Без указания выводи в stdout.
runtime.log.max_size #Максимальный размер файла логов в МБ.
runtime.log.max_backups #Максимальное количество архивных файлов логов.
runtime.log.max_age #Максимальный возраст логов в днях.
runtime.log.compress #Требуется ли сжимать старые логи. true/false.
```

## Примеры логов

### runtime.log.format="text"

```log
INFO[2026-01-03T20:44:05+07:00] Бот запущен.                                 
INFO[2026-01-03T20:44:06+07:00] Получены ограничения торговой пары.           component=engine rules_base=XRP rules_lot_size=0.0001 rules_min_notional=0.0000000001 rules_min_qty=0.0001 rules_quote=USDT rules_tick_size=0.000001 symbol=XRPUSDT
INFO[2026-01-03T20:44:06+07:00] Подписываемся на торговую парую               component=bybit symbol=XRPUSDT
INFO[2026-01-03T20:44:06+07:00] Подключение к WS.                             component=bybit_ws url="wss://stream-testnet.bybit.com/v5/public/spot"
INFO[2026-01-03T20:44:07+07:00] WS соединение установлено.                    component=bybit_ws
INFO[2026-01-03T20:44:07+07:00] Подключение к WS.                             component=bybit_ws url="wss://stream-testnet.bybit.com/v5/private"
INFO[2026-01-03T20:44:08+07:00] WS соединение установлено.                    component=bybit_ws
INFO[2026-01-03T20:44:08+07:00] Подписки активированы.                        component=bybit symbol=XRPUSDT
INFO[2026-01-03T20:44:08+07:00] Входной ордер.                                component=engine qty=100 side=Buy symbol=XRPUSDT type=Market
INFO[2026-01-03T20:44:08+07:00] Попытка ордера.                               bal_base=95.02175123 bal_quote=89496.06390374 base_asset=XRP component=engine market_unit=quoteCoin need_base=0 need_quote=100 price=0 qty=100 quote_asset=USDT side=Buy symbol=XRPUSDT type=Market
INFO[2026-01-03T20:44:09+07:00] Отправка market ордер на вход.                component=engine symbol=XRPUSDT
INFO[2026-01-03T20:44:11+07:00] Постановка TP.                                component=engine link_id=48c168d04f89-tp-1767462250-1 price=2.180406 qty=46.0923 symbol=XRPUSDT
INFO[2026-01-03T20:44:12+07:00] Попытка ордера.                               bal_base=141.07832969 bal_quote=89396.06393944 base_asset=XRP component=engine market_unit= need_base=46.0923 need_quote=0 price=2.180406 qty=46.0923 quote_asset=USDT side=Sell symbol=XRPUSDT type=Limit
INFO[2026-01-03T20:44:12+07:00] TP поставлен.                                 component=engine order_id=2120021468447512064 symbol=XRPUSDT
INFO[2026-01-03T20:44:13+07:00] TP подтверждён в open orders.                 component=engine leaves_qty=46.0923 order_id=2120021468447512064 price=2.180406 qty=46.0923 status=New symbol=XRPUSDT
INFO[2026-01-03T20:44:13+07:00] План сетки страховочных ордеров.              component=engine count=10 symbol=XRPUSDT
INFO[2026-01-03T20:44:13+07:00] Постановка страховочного ордера.              component=engine link_id=48c168d04f89-so-1 price=2.147863 qty=46.557900000000004 symbol=XRPUSDT
INFO[2026-01-03T20:44:14+07:00] Попытка ордера.                               bal_base=141.07832969 bal_quote=89396.06393944 base_asset=XRP component=engine market_unit= need_base=0 need_quote=99.99999076770001 price=2.147863 qty=46.557900000000004 quote_asset=USDT side=Buy symbol=XRPUSDT type=Limit
INFO[2026-01-03T20:44:14+07:00] Страховочный ордер поставлен.                 component=engine order_id=2120021483018524160 symbol=XRPUSDT
INFO[2026-01-03T20:44:15+07:00] Постановка страховочного ордера.              component=engine link_id=48c168d04f89-so-2 price=2.104472 qty=52.269600000000004 symbol=XRPUSDT
INFO[2026-01-03T20:44:16+07:00] Попытка ордера.                               bal_base=141.07832969 bal_quote=89396.06393944 base_asset=XRP component=engine market_unit= need_base=0 need_quote=109.9999096512 price=2.104472 qty=52.269600000000004 quote_asset=USDT side=Buy symbol=XRPUSDT type=Limit
INFO[2026-01-03T20:44:16+07:00] Страховочный ордер поставлен.                 component=engine order_id=2120021496633234944 symbol=XRPUSDT
INFO[2026-01-03T20:44:16+07:00] Постановка страховочного ордера.              component=engine link_id=48c168d04f89-so-3 price=2.017689 qty=59.969500000000004 symbol=XRPUSDT
INFO[2026-01-03T20:44:17+07:00] Попытка ордера.                               bal_base=141.07832969 bal_quote=89396.06393944 base_asset=XRP component=engine market_unit= need_base=0 need_quote=120.9998004855 price=2.017689 qty=59.969500000000004 quote_asset=USDT side=Buy symbol=XRPUSDT type=Limit
INFO[2026-01-03T20:44:17+07:00] Страховочный ордер поставлен.                 component=engine order_id=2120021510306665984 symbol=XRPUSDT
INFO[2026-01-03T20:44:18+07:00] Постановка страховочного ордера.              component=engine link_id=48c168d04f89-so-4 price=1.844125 qty=72.1751 symbol=XRPUSDT
INFO[2026-01-03T20:44:19+07:00] Попытка ордера.                               bal_base=141.07832969 bal_quote=89396.06393944 base_asset=XRP component=engine market_unit= need_base=0 need_quote=133.0999062875 price=1.844125 qty=72.1751 quote_asset=USDT side=Buy symbol=XRPUSDT type=Limit
INFO[2026-01-03T20:44:19+07:00] Страховочный ордер поставлен.                 component=engine order_id=2120021526815446528 symbol=XRPUSDT
INFO[2026-01-03T20:44:20+07:00] Постановка страховочного ордера.              component=engine link_id=48c168d04f89-so-5 price=1.4969949999999999 qty=97.80250000000001 symbol=XRPUSDT
INFO[2026-01-03T20:44:21+07:00] Попытка ордера.                               bal_base=141.07832969 bal_quote=89396.06393944 base_asset=XRP component=engine market_unit= need_base=0 need_quote=146.4098534875 price=1.4969949999999999 qty=97.80250000000001 quote_asset=USDT side=Buy symbol=XRPUSDT type=Limit
INFO[2026-01-03T20:44:21+07:00] Страховочный ордер поставлен.                 component=engine order_id=2120021541126411776 symbol=XRPUSDT
INFO[2026-01-03T20:44:22+07:00] Постановка страховочного ордера.              component=engine link_id=48c168d04f89-so-6 price=0.802736 qty=200.6276 symbol=XRPUSDT
INFO[2026-01-03T20:44:22+07:00] Попытка ордера.                               bal_base=141.07832969 bal_quote=89396.06393944 base_asset=XRP component=engine market_unit= need_base=0 need_quote=161.0509971136 price=0.802736 qty=200.6276 quote_asset=USDT side=Buy symbol=XRPUSDT type=Limit
INFO[2026-01-03T20:44:22+07:00] Страховочный ордер поставлен.                 component=engine order_id=2120021551721223680 symbol=XRPUSDT
WARN[2026-01-03T20:44:23+07:00] Страховочный ордер пропущен, нет цены для пересчёта объёма.  component=engine entry_price=2.169559 index=7 price=-0.585781 so_count=10 symbol=XRPUSDT total_percent=127
WARN[2026-01-03T20:44:23+07:00] Страховочный ордер пропущен, нет цены для пересчёта объёма.  component=engine entry_price=2.169559 index=8 price=-3.3628169999999997 so_count=10 symbol=XRPUSDT total_percent=255
WARN[2026-01-03T20:44:23+07:00] Страховочный ордер пропущен, нет цены для пересчёта объёма.  component=engine entry_price=2.169559 index=9 price=-8.916888 so_count=10 symbol=XRPUSDT total_percent=511
WARN[2026-01-03T20:44:23+07:00] Страховочный ордер пропущен, нет цены для пересчёта объёма.  component=engine entry_price=2.169559 index=10 price=-20.025029999999997 so_count=10 symbol=XRPUSDT total_percent=1023
```

### runtime.log.format="json"

```json
{"level":"info","msg":"Бот запущен.","time":"2026-01-03T20:48:05+03:00"}
{"component":"engine","level":"info","msg":"Получены ограничения торговой пары.","rules_base":"XRP","rules_lot_size":"0.0001","rules_min_notional":"0.0000000001","rules_min_qty":"0.0001","rules_quote":"USDT","rules_tick_size":"0.000001","symbol":"XRPUSDT","time":"2026-01-03T20:48:05+03:00"}
{"component":"bybit","level":"info","msg":"Подписываемся на торговую парую","symbol":"XRPUSDT","time":"2026-01-03T20:48:05+03:00"}
{"component":"bybit_ws","level":"info","msg":"Подключение к WS.","time":"2026-01-03T20:48:05+03:00","url":"wss://stream-testnet.bybit.com/v5/public/spot"}
{"component":"bybit_ws","level":"info","msg":"WS соединение установлено.","time":"2026-01-03T20:48:06+03:00"}
{"component":"bybit_ws","level":"info","msg":"Подключение к WS.","time":"2026-01-03T20:48:06+03:00","url":"wss://stream-testnet.bybit.com/v5/private"}
{"component":"bybit_ws","level":"info","msg":"WS соединение установлено.","time":"2026-01-03T20:48:07+03:00"}
{"component":"bybit","level":"info","msg":"Подписки активированы.","symbol":"XRPUSDT","time":"2026-01-03T20:48:07+03:00"}
{"component":"engine","level":"info","msg":"Входной ордер.","qty":100,"side":"Buy","symbol":"XRPUSDT","time":"2026-01-03T20:48:07+03:00","type":"Market"}
{"bal_base":141.07832969,"bal_quote":89396.06393944,"base_asset":"XRP","component":"engine","level":"info","market_unit":"quoteCoin","msg":"Попытка ордера.","need_base":0,"need_quote":100,"price":0,"qty":100,"quote_asset":"USDT","side":"Buy","symbol":"XRPUSDT","time":"2026-01-03T20:48:08+03:00","type":"Market"}
{"component":"engine","level":"info","msg":"Отправка market ордер на вход.","symbol":"XRPUSDT","time":"2026-01-03T20:48:08+03:00"}
{"component":"engine","level":"info","link_id":"1f746787c83a-tp-1767462489-1","msg":"Постановка TP.","price":2.1803749999999997,"qty":46.0929,"symbol":"XRPUSDT","time":"2026-01-03T20:48:10+03:00"}
{"bal_base":187.1355077,"bal_quote":89296.06410229,"base_asset":"XRP","component":"engine","level":"info","market_unit":"","msg":"Попытка ордера.","need_base":46.0929,"need_quote":0,"price":2.1803749999999997,"qty":46.0929,"quote_asset":"USDT","side":"Sell","symbol":"XRPUSDT","time":"2026-01-03T20:48:10+03:00","type":"Limit"}
{"component":"engine","level":"info","msg":"TP поставлен.","order_id":"2120023464567117312","symbol":"XRPUSDT","time":"2026-01-03T20:48:11+03:00"}
{"component":"engine","leaves_qty":46.0929,"level":"info","msg":"TP подтверждён в open orders.","order_id":"2120023464567117312","price":2.180375,"qty":46.0929,"status":"New","symbol":"XRPUSDT","time":"2026-01-03T20:48:11+03:00"}
{"component":"engine","count":10,"level":"info","msg":"План сетки страховочных ордеров.","symbol":"XRPUSDT","time":"2026-01-03T20:48:11+03:00"}
{"component":"engine","level":"info","link_id":"1f746787c83a-so-1","msg":"Постановка страховочного ордера.","price":2.1478319999999997,"qty":46.5585,"symbol":"XRPUSDT","time":"2026-01-03T20:48:11+03:00"}
{"bal_base":187.1355077,"bal_quote":89296.06410229,"base_asset":"XRP","component":"engine","level":"info","market_unit":"","msg":"Попытка ордера.","need_base":0,"need_quote":99.99983617199999,"price":2.1478319999999997,"qty":46.5585,"quote_asset":"USDT","side":"Buy","symbol":"XRPUSDT","time":"2026-01-03T20:48:12+03:00","type":"Limit"}
{"component":"engine","level":"info","msg":"Страховочный ордер поставлен.","order_id":"2120023475069654528","symbol":"XRPUSDT","time":"2026-01-03T20:48:12+03:00"}
{"component":"engine","level":"info","link_id":"1f746787c83a-so-2","msg":"Постановка страховочного ордера.","price":2.1044419999999997,"qty":52.270300000000006,"symbol":"XRPUSDT","time":"2026-01-03T20:48:12+03:00"}
{"bal_base":187.1355077,"bal_quote":89296.06410229,"base_asset":"XRP","component":"engine","level":"info","market_unit":"","msg":"Попытка ордера.","need_base":0,"need_quote":109.9998146726,"price":2.1044419999999997,"qty":52.270300000000006,"quote_asset":"USDT","side":"Buy","symbol":"XRPUSDT","time":"2026-01-03T20:48:13+03:00","type":"Limit"}
{"component":"engine","level":"info","msg":"Страховочный ордер поставлен.","order_id":"2120023485169538560","symbol":"XRPUSDT","time":"2026-01-03T20:48:13+03:00"}
{"component":"engine","level":"info","link_id":"1f746787c83a-so-3","msg":"Постановка страховочного ордера.","price":2.017661,"qty":59.970400000000005,"symbol":"XRPUSDT","time":"2026-01-03T20:48:13+03:00"}
{"bal_base":187.1355077,"bal_quote":89296.06410229,"base_asset":"XRP","component":"engine","level":"info","market_unit":"","msg":"Попытка ордера.","need_base":0,"need_quote":120.99993723440001,"price":2.017661,"qty":59.970400000000005,"quote_asset":"USDT","side":"Buy","symbol":"XRPUSDT","time":"2026-01-03T20:48:14+03:00","type":"Limit"}
{"component":"engine","level":"info","msg":"Страховочный ордер поставлен.","order_id":"2120023495227479552","symbol":"XRPUSDT","time":"2026-01-03T20:48:14+03:00"}
{"component":"engine","level":"info","link_id":"1f746787c83a-so-4","msg":"Постановка страховочного ордера.","price":1.844098,"qty":72.17620000000001,"symbol":"XRPUSDT","time":"2026-01-03T20:48:15+03:00"}
{"bal_base":187.1355077,"bal_quote":89296.06410229,"base_asset":"XRP","component":"engine","level":"info","market_unit":"","msg":"Попытка ордера.","need_base":0,"need_quote":133.0999860676,"price":1.844098,"qty":72.17620000000001,"quote_asset":"USDT","side":"Buy","symbol":"XRPUSDT","time":"2026-01-03T20:48:15+03:00","type":"Limit"}
{"component":"engine","level":"info","msg":"Страховочный ордер поставлен.","order_id":"2120023505310586368","symbol":"XRPUSDT","time":"2026-01-03T20:48:15+03:00"}
{"component":"engine","level":"info","link_id":"1f746787c83a-so-5","msg":"Постановка страховочного ордера.","price":1.496974,"qty":97.8039,"symbol":"XRPUSDT","time":"2026-01-03T20:48:16+03:00"}
{"bal_base":187.1355077,"bal_quote":89296.06410229,"base_asset":"XRP","component":"engine","level":"info","market_unit":"","msg":"Попытка ордера.","need_base":0,"need_quote":146.4098953986,"price":1.496974,"qty":97.8039,"quote_asset":"USDT","side":"Buy","symbol":"XRPUSDT","time":"2026-01-03T20:48:16+03:00","type":"Limit"}
{"component":"engine","level":"info","msg":"Страховочный ордер поставлен.","order_id":"2120023515402081792","symbol":"XRPUSDT","time":"2026-01-03T20:48:17+03:00"}
{"component":"engine","level":"info","link_id":"1f746787c83a-so-6","msg":"Постановка страховочного ордера.","price":0.8027249999999999,"qty":200.6303,"symbol":"XRPUSDT","time":"2026-01-03T20:48:17+03:00"}
{"bal_base":187.1355077,"bal_quote":89296.06410229,"base_asset":"XRP","component":"engine","level":"info","market_unit":"","msg":"Попытка ордера.","need_base":0,"need_quote":161.05095756749998,"price":0.8027249999999999,"qty":200.6303,"quote_asset":"USDT","side":"Buy","symbol":"XRPUSDT","time":"2026-01-03T20:48:18+03:00","type":"Limit"}
{"component":"engine","level":"info","msg":"Страховочный ордер поставлен.","order_id":"2120023525476800000","symbol":"XRPUSDT","time":"2026-01-03T20:48:18+03:00"}
{"component":"engine","entry_price":2.169528,"index":7,"level":"warning","msg":"Страховочный ордер пропущен, нет цены для пересчёта объёма.","price":-0.585773,"so_count":10,"symbol":"XRPUSDT","time":"2026-01-03T20:48:18+03:00","total_percent":127}
{"component":"engine","entry_price":2.169528,"index":8,"level":"warning","msg":"Страховочный ордер пропущен, нет цены для пересчёта объёма.","price":-3.3627689999999997,"so_count":10,"symbol":"XRPUSDT","time":"2026-01-03T20:48:18+03:00","total_percent":255}
{"component":"engine","entry_price":2.169528,"index":9,"level":"warning","msg":"Страховочный ордер пропущен, нет цены для пересчёта объёма.","price":-8.916761,"so_count":10,"symbol":"XRPUSDT","time":"2026-01-03T20:48:18+03:00","total_percent":511}
{"component":"engine","entry_price":2.169528,"index":10,"level":"warning","msg":"Страховочный ордер пропущен, нет цены для пересчёта объёма.","price":-20.024744,"so_count":10,"symbol":"XRPUSDT","time":"2026-01-03T20:48:18+03:00","total_percent":1023}
```
