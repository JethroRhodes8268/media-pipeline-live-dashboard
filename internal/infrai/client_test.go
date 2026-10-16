package infrai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEnvelopeBeforeHTTPStatusAndRetry(t *testing.T) {
	calls := 0
	c := New("test-key")
	c.sleep = func(time.Duration) {}
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		var request batchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.IdempotencyKey != "event-1" {
			t.Fatalf("idempotency_key = %q, want event-1", request.IdempotencyKey)
		}
		status, body := 429, `{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"slow down"},"metadata":{}}`
		if calls == 2 {
			status, body = 200, `{"ok":true,"data":{"accepted":1},"error":null,"metadata":{}}`
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": []string{"1"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	if err := c.call(context.Background(), http.MethodPost, "/metrics/batch", batchRequest{[]Point{{Name: "media.assets", Value: 1}}, "event-1"}, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}
