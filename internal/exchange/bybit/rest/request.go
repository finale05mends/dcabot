package rest

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"time"
)

func (c *Client) doRequest(ctx context.Context, method, path string, params url.Values, body any, auth bool, out any) error {
	var bodyReader io.Reader
	var bodyStr string
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("Не удалось подготовить тело запроса: %w", err)
		}
		bodyStr = string(payload)
		bodyReader = bytes.NewReader(payload)
	}

	urlStr := c.baseURL + path
	if len(params) > 0 {
		urlStr += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return fmt.Errorf("Не удалось создать запрос: %w", err)
	}

	if auth {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		recvWindow := "5000"
		query := ""

		if method == http.MethodGet && len(params) > 0 {
			query = params.Encode()
		}

		signBase := timestamp + c.apiKey + recvWindow + query + bodyStr
		signature := sign(c.secret, signBase)

		req.Header.Set("X-BAPI-API-KEY", c.apiKey)
		req.Header.Set("X-BAPI-SIGN", signature)
		req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
		req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Ошибка запроса: %w", err)
	}

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Не удалось прочитать ответ: %w", err)
	}

	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("Не удалось разобрать ответ: %w", err)
	}

	if retCode, retMsg, ok := extractRetCode(out); ok && retCode != 0 {
		return fmt.Errorf("Ошибка bybit: %s (code=%d)", retMsg, retCode)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Неуспешный статус: %s", resp.Status)
	}

	return nil
}

func sign(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// TODO
func extractRetCode(v any) (int, string, bool) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}

	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return 0, "", false
	}

	retCodeField := rv.FieldByName("RetCode")
	retMsgField := rv.FieldByName("RetMsg")

	if retCodeField.IsValid() && retMsgField.IsValid() {
		return int(retCodeField.Int()), retMsgField.String(), true
	}

	return 0, "", false
}
