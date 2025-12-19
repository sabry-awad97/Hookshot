package webhook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				TargetURL: "http://localhost:4000/webhook",
				Secret:    "whsec_C2FtcGxlX3NlY3JldF9rZXlfZm9yX3Rlc3Rpbmc=",
			},
			wantErr: false,
		},
		{
			name: "missing URL",
			config: Config{
				Secret: "whsec_C2FtcGxlX3NlY3JldF9rZXlfZm9yX3Rlc3Rpbmc=",
			},
			wantErr: true,
		},
		{
			name: "missing secret",
			config: Config{
				TargetURL: "http://localhost:4000/webhook",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_Send(t *testing.T) {
	// Create mock server
	var receivedPayload Payload
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		TargetURL: server.URL,
		Secret:    "whsec_C2FtcGxlX3NlY3JldF9rZXlfZm9yX3Rlc3Rpbmc=",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Send webhook
	resp := client.Send("order.created", map[string]any{
		"order_id": "12345",
		"amount":   99.99,
	})

	// Verify response
	if !resp.Success {
		t.Errorf("Expected success, got error: %v", resp.Error)
	}
	if resp.MessageID == "" {
		t.Error("Expected message ID")
	}
	if resp.Signature == "" {
		t.Error("Expected signature")
	}

	// Verify payload received
	if receivedPayload.Event != "order.created" {
		t.Errorf("Expected event 'order.created', got '%s'", receivedPayload.Event)
	}

	// Verify headers
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

func TestClient_Retry(t *testing.T) {
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

	client, _ := NewClient(Config{
		TargetURL:  server.URL,
		Secret:     "whsec_C2FtcGxlX3NlY3JldF9rZXlfZm9yX3Rlc3Rpbmc=",
		MaxRetries: 3,
		Timeout:    1 * time.Second,
	})

	resp := client.Send("test.retry", nil)

	if !resp.Success {
		t.Errorf("Expected success after retries, got error: %v", resp.Error)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestClient_MaxRetriesExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		TargetURL:  server.URL,
		Secret:     "whsec_C2FtcGxlX3NlY3JldF9rZXlfZm9yX3Rlc3Rpbmc=",
		MaxRetries: 2,
		Timeout:    1 * time.Second,
	})

	resp := client.Send("test.fail", nil)

	if resp.Success {
		t.Error("Expected failure after max retries")
	}
	if resp.Error == nil {
		t.Error("Expected error to be set")
	}
}
