package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testSecret = "whsec_C2FtcGxlX3NlY3JldF9rZXlfZm9yX3Rlc3Rpbmc="

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		targetURL string
		secret    string
		wantErr   bool
	}{
		{
			name:      "valid config",
			targetURL: "http://localhost:4000/webhook",
			secret:    testSecret,
			wantErr:   false,
		},
		{
			name:      "missing URL",
			targetURL: "",
			secret:    testSecret,
			wantErr:   true,
		},
		{
			name:      "missing secret",
			targetURL: "http://localhost:4000/webhook",
			secret:    "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.targetURL, tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_Send(t *testing.T) {
	var receivedPayload Payload
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, testSecret)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	resp := client.Send(ctx, "order.created", map[string]any{
		"order_id": "12345",
		"amount":   99.99,
	})

	if !resp.Success {
		t.Errorf("Expected success, got error: %v", resp.Error)
	}
	if resp.MessageID == "" {
		t.Error("Expected message ID")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	if receivedPayload.Event != "order.created" {
		t.Errorf("Expected event 'order.created', got '%s'", receivedPayload.Event)
	}

	if receivedHeaders.Get("svix-id") == "" {
		t.Error("Expected svix-id header")
	}
	if receivedHeaders.Get("svix-signature") == "" {
		t.Error("Expected svix-signature header")
	}
	if receivedHeaders.Get("svix-timestamp") == "" {
		t.Error("Expected svix-timestamp header")
	}
}

func TestClient_Send_MessageIDFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, testSecret)

	ctx := context.Background()
	resp := client.Send(ctx, "test.event", nil)

	if !strings.HasPrefix(resp.MessageID, "msg_") {
		t.Errorf("Expected message ID to start with 'msg_', got '%s'", resp.MessageID)
	}
	// UUID format: msg_xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if len(resp.MessageID) != 40 {
		t.Errorf("Expected message ID length 40 (msg_ + UUID), got %d", len(resp.MessageID))
	}
}

func TestClient_Send_TimestampFormat(t *testing.T) {
	var receivedTimestamp string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTimestamp = r.Header.Get("svix-timestamp")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, testSecret)

	before := time.Now().Unix()
	ctx := context.Background()
	client.Send(ctx, "test.event", nil)
	after := time.Now().Unix()

	// Timestamp should be Unix seconds (numeric string)
	var ts int64
	if err := json.Unmarshal([]byte(receivedTimestamp), &ts); err != nil {
		t.Errorf("Expected numeric timestamp, got '%s'", receivedTimestamp)
	}

	if ts < before || ts > after {
		t.Errorf("Timestamp %d not in expected range [%d, %d]", ts, before, after)
	}
}

func TestClient_Retry_ServerError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, testSecret,
		WithMaxRetries(3),
		WithTimeout(1*time.Second),
	)

	ctx := context.Background()
	resp := client.Send(ctx, "test.retry", nil)

	if !resp.Success {
		t.Errorf("Expected success after retries, got error: %v", resp.Error)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestClient_NoRetry_ClientError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, testSecret,
		WithMaxRetries(3),
		WithTimeout(1*time.Second),
	)

	ctx := context.Background()
	resp := client.Send(ctx, "test.client_error", nil)

	if resp.Success {
		t.Error("Expected failure on client error")
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
	// Should NOT retry on 4xx errors
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("Expected 1 attempt (no retry on 4xx), got %d", attempts)
	}
}

func TestClient_MaxRetriesExceeded(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, testSecret,
		WithMaxRetries(2),
		WithTimeout(1*time.Second),
	)

	ctx := context.Background()
	resp := client.Send(ctx, "test.fail", nil)

	if resp.Success {
		t.Error("Expected failure after max retries")
	}
	if resp.Error == nil {
		t.Error("Expected error to be set")
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Slow server
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, testSecret,
		WithMaxRetries(3),
		WithTimeout(10*time.Second),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	resp := client.Send(ctx, "test.cancel", nil)

	if resp.Success {
		t.Error("Expected failure due to context cancellation")
	}
	if resp.Error == nil {
		t.Error("Expected error to be set")
	}
}

func TestClient_SendPayload(t *testing.T) {
	var receivedPayload Payload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, testSecret)

	customTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	payload := Payload{
		Event:     "custom.event",
		Timestamp: customTime,
		Data:      map[string]string{"key": "value"},
	}

	ctx := context.Background()
	resp := client.SendPayload(ctx, payload)

	if !resp.Success {
		t.Errorf("Expected success, got error: %v", resp.Error)
	}
	if receivedPayload.Event != "custom.event" {
		t.Errorf("Expected event 'custom.event', got '%s'", receivedPayload.Event)
	}
	if !receivedPayload.Timestamp.Equal(customTime) {
		t.Errorf("Expected timestamp %v, got %v", customTime, receivedPayload.Timestamp)
	}
}

func TestClient_DefaultConfig(t *testing.T) {
	client, err := NewClient("http://localhost:4000/webhook", testSecret)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify defaults are applied
	if client.config.MaxRetries != 3 {
		t.Errorf("Expected default MaxRetries 3, got %d", client.config.MaxRetries)
	}
	if client.config.Timeout != 10*time.Second {
		t.Errorf("Expected default Timeout 10s, got %v", client.config.Timeout)
	}
	if client.config.MaxInterval != 30*time.Second {
		t.Errorf("Expected default MaxInterval 30s, got %v", client.config.MaxInterval)
	}
}

func TestClient_WithHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	customClient := &http.Client{Timeout: 5 * time.Second}
	client, err := NewClient(server.URL, testSecret, WithHTTPClient(customClient))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.http != customClient {
		t.Error("Expected custom HTTP client to be used")
	}

	// Verify it works
	resp := client.Send(context.Background(), "test.event", nil)
	if !resp.Success {
		t.Errorf("Expected success, got error: %v", resp.Error)
	}
}

func TestSentinelErrors(t *testing.T) {
	t.Run("client error wrapping", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		client, _ := NewClient(server.URL, testSecret)
		resp := client.Send(context.Background(), "test", nil)

		if !errors.Is(resp.Error, ErrClientError) {
			t.Errorf("Expected error to wrap ErrClientError, got: %v", resp.Error)
		}
	})

	t.Run("server error wrapping", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client, _ := NewClient(server.URL, testSecret, WithMaxRetries(1))
		resp := client.Send(context.Background(), "test", nil)

		if !errors.Is(resp.Error, ErrServerError) {
			t.Errorf("Expected error to wrap ErrServerError, got: %v", resp.Error)
		}
	})
}

func TestFunctionalOptions(t *testing.T) {
	client, err := NewClient(
		"http://localhost:4000/webhook",
		testSecret,
		WithMaxRetries(5),
		WithTimeout(30*time.Second),
		WithMaxInterval(60*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.config.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries 5, got %d", client.config.MaxRetries)
	}
	if client.config.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout 30s, got %v", client.config.Timeout)
	}
	if client.config.MaxInterval != 60*time.Second {
		t.Errorf("Expected MaxInterval 60s, got %v", client.config.MaxInterval)
	}
}
