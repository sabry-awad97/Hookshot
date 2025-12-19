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

	"github.com/gin-gonic/gin"
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

// generateSignature creates HMAC-SHA256 signature
func generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// SendWebhook sends a signed webhook to a subscriber
func SendWebhook(config WebhookConfig, payload WebhookPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

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

// SendWebhookWithRetry sends webhook with exponential backoff
func SendWebhookWithRetry(config WebhookConfig, payload WebhookPayload, maxRetries int) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := SendWebhook(config, payload); err == nil {
			return nil
		} else {
			lastErr = err
			backoff := time.Duration(1<<attempt) * time.Second
			log.Printf("Retry %d/%d in %v: %v", attempt+1, maxRetries, backoff, err)
			time.Sleep(backoff)
		}
	}
	return lastErr
}

func main() {
	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Trigger webhook endpoint
	r.POST("/trigger", func(c *gin.Context) {
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

		if err := SendWebhookWithRetry(config, payload, 3); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Webhook sent!"})
	})

	// Custom trigger with payload from request body
	r.POST("/trigger/:event", func(c *gin.Context) {
		event := c.Param("event")

		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
			return
		}

		config := WebhookConfig{
			URL:    "http://localhost:4000/webhook",
			Secret: "your-shared-secret-key",
		}

		payload := WebhookPayload{
			Event:     event,
			Timestamp: time.Now(),
			Data:      data,
		}

		if err := SendWebhookWithRetry(config, payload, 3); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Webhook sent!", "event": event})
	})

	log.Println("🚀 Gin webhook server running on :8080")
	r.Run(":8080")
}
