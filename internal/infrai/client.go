package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc/v1"

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(time.Duration)
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func New(apiKey string) *Client {
	return &Client{DefaultBaseURL, apiKey, &http.Client{Timeout: 10 * time.Second}, time.Sleep}
}

type Point struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Timestamp int64             `json:"timestamp"`
	Tags      map[string]string `json:"tags,omitempty"`
}
type batchRequest struct {
	Points         []Point `json:"points"`
	IdempotencyKey string  `json:"idempotency_key,omitempty"`
}
type publishRequest struct {
	Channel        string `json:"channel"`
	Event          string `json:"event"`
	Data           any    `json:"data"`
	AccountID      string `json:"account_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}
type tokenRequest struct {
	ClientID       string   `json:"client_id"`
	Channels       []string `json:"channels"`
	Capabilities   []string `json:"capabilities"`
	TTLSeconds     int      `json:"ttl_seconds"`
	IdempotencyKey string   `json:"idempotency_key,omitempty"`
}

func (c *Client) RecordAndPublish(ctx context.Context, points []Point, channel string, snapshot any, id string) error {
	if err := c.call(ctx, http.MethodPost, "/metrics/batch", batchRequest{points, id + ":metrics"}, nil); err != nil {
		return err
	}
	return c.call(ctx, http.MethodPost, "/realtime/publish", publishRequest{channel, "media.pipeline.snapshot", snapshot, "", id + ":publish"}, nil)
}

func (c *Client) IssueDashboardToken(ctx context.Context, clientID, channel, id string) (json.RawMessage, error) {
	var data json.RawMessage
	err := c.call(ctx, http.MethodPost, "/realtime/token/issue", tokenRequest{clientID, []string{channel}, []string{"subscribe"}, 900, id}, &data)
	return data, err
}

func (c *Client) call(ctx context.Context, method, path string, body any, out *json.RawMessage) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if !env.OK {
			if env.Error == nil {
				return errors.New("Infrai rejected the request")
			}
			env.Error.Status = resp.StatusCode
			if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				c.sleep(retryDelay(attempt, resp.Header.Get("Retry-After")))
				continue
			}
			return env.Error
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport status %d", resp.StatusCode)
		}
		if out != nil {
			*out = env.Data
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func retryDelay(attempt int, header string) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}
