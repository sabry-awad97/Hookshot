package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	svix "github.com/svix/svix-webhooks/go"
)

// WebhookPayload represents the data sent to subscribers
type WebhookPayload struct {
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

const webhookSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"

func main() {
	r := gin.Default()

	// Initialize Svix webhook sender
	wh, err := svix.NewWebhook(webhookSecret)
	if err != nil {
		log.Fatalf("Failed to initialize Svix webhook: %v", err)
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Trigger webhook endpoint
	r.POST("/trigger", func(c *gin.Context) {
		payload := []byte(`{"event":"order.created","timestamp":"` + time.Now().Format(time.RFC3339) + `","data":{"order_id":"12345","amount":99.99}}`)

		// Generate Svix signature headers
		msgID := "msg_" + time.Now().Format("20060102150405")
		timestamp := time.Now()
		signature, err := wh.Sign(msgID, timestamp, payload)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Send webhook to listener
		client := &http.Client{Timeout: 10 * time.Second}
		req, _ := http.NewRequest("POST", "http://localhost:4000/webhook", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("svix-id", msgID)
		req.Header.Set("svix-timestamp", timestamp.Format(time.RFC3339))
		req.Header.Set("svix-signature", signature)

		// Use bytes.NewBuffer for body
		req, _ = http.NewRequest("POST", "http://localhost:4000/webhook",
			&payloadReader{payload: payload})
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("svix-id", msgID)
		req.Header.Set("svix-timestamp", timestamp.Format(time.RFC3339))
		req.Header.Set("svix-signature", signature)

		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer resp.Body.Close()

		c.JSON(http.StatusOK, gin.H{
			"message":   "Webhook sent!",
			"msgId":     msgID,
			"signature": signature,
		})
	})

	// Custom trigger with event type
	r.POST("/trigger/:event", func(c *gin.Context) {
		event := c.Param("event")

		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
			return
		}

		payload := WebhookPayload{
			Event:     event,
			Timestamp: time.Now(),
			Data:      data,
		}

		payloadBytes, _ := json.Marshal(payload)
		msgID := "msg_" + time.Now().Format("20060102150405")
		timestamp := time.Now()
		signature, err := wh.Sign(msgID, timestamp, payloadBytes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		client := &http.Client{Timeout: 10 * time.Second}
		req, _ := http.NewRequest("POST", "http://localhost:4000/webhook",
			&payloadReader{payload: payloadBytes})
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("svix-id", msgID)
		req.Header.Set("svix-timestamp", timestamp.Format(time.RFC3339))
		req.Header.Set("svix-signature", signature)

		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer resp.Body.Close()

		c.JSON(http.StatusOK, gin.H{
			"message": "Webhook sent!",
			"event":   event,
			"msgId":   msgID,
		})
	})

	log.Println("🚀 Gin + Svix webhook server running on :8080")
	r.Run(":8080")
}

// payloadReader implements io.Reader for sending payload
type payloadReader struct {
	payload []byte
	offset  int
}

func (r *payloadReader) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.payload) {
		return 0, io.EOF
	}
	n = copy(p, r.payload[r.offset:])
	r.offset += n
	return n, nil
}
