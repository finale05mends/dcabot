FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/bot ./cmd/bot

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /out/bot /app/bot
COPY configs /app/configs

RUN mkdir -p /data
VOLUME ["/data"]

ENTRYPOINT ["/app/bot"]