package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// WebhookPayload represents the data sent to subscribers
type WebhookPayload struct {
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// WebhookConfig holds subscriber configuration
type WebhookConfig struct {
	URL    string
	Secret string
}

// SendWebhook sends a signed webhook to a subscriber
func SendWebhook(config WebhookConfig, payload WebhookPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create HMAC signature for security
	signature := generateSignature(jsonData, config.Secret)

	req, err := http.NewRequest("POST", config.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Timestamp", time.Now().UTC().Format(time.RFC3339))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("Webhook failed with status: %d", resp.StatusCode)
	}
	return nil
}

// generateSignature creates HMAC-SHA256 signature
func generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	// HTTP server that triggers webhooks
	http.HandleFunc("/trigger", func(w http.ResponseWriter, r *http.Request) {
		config := WebhookConfig{
			URL:    "http://localhost:4000/webhook",
			Secret: "your-shared-secret-key",
		}

		payload := WebhookPayload{
			Event:     "order.created",
			Timestamp: time.Now(),
			Data: map[string]any{
				"order_id": "12345",
				"amount":   99.99,
			},
		}

		if err := SendWebhook(config, payload); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Webhook sent!"))
	})

	log.Println("Go server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
