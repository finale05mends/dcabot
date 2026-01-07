APP_NAME := dcabot
DATA_DIR := $(CURDIR)/data

build:
	go build -o bin/bot ./cmd/bot

run:
	go run ./cmd/bot/main.go

test:
	go test ./... -race -count=1

test-integration:
	go test -v -tags=integration ./internal/engine

test-all: test test-integration

lint:
	gofmt -w .
	go vet ./...

docker-build:
	docker build -t $(APP_NAME):latest .

docker-run:
	mkdir -p $(DATA_DIR)
	docker run -d \
		-p 2112:2112 \
		-v $(DATA_DIR):/app/data \
		-v $(CURDIR)/configs:/app/configs \
		-e BYBIT_API_KEY \
		-e BYBIT_API_SECRET \
		--name $(APP_NAME) \
		$(APP_NAME):latest 

docker-run-once:
	mkdir -p $(DATA_DIR)
	docker run --rm \
		-p 2112:2112 \
		-v $(DATA_DIR):/app/data \
		-v $(CURDIR)/configs:/app/configs \
		-e BYBIT_API_KEY \
		-e BYBIT_API_SECRET \
		$(APP_NAME):latest 

.PHONY: build run test test-integration test-all lint docker-build docker-run docker-run-once
