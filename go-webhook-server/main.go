package main

import (
	"log"
	"net/http"
	"os"

	"hookshot-server/pkg/webhook"

	"github.com/gin-gonic/gin"
)

func main() {
	// Get configuration from environment (with defaults)
	secret := getEnv("WEBHOOK_SECRET", "whsec_C2FtcGxlX3NlY3JldF9rZXlfZm9yX3Rlc3Rpbmc=")
	targetURL := getEnv("WEBHOOK_TARGET_URL", "http://localhost:4000/webhook")

	// Create reusable webhook client
	client, err := webhook.NewClient(webhook.Config{
		TargetURL:  targetURL,
		Secret:     secret,
		MaxRetries: 3,
	})
	if err != nil {
		log.Fatalf("Failed to create webhook client: %v", err)
	}

	r := gin.Default()

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Trigger default webhook
	r.POST("/trigger", func(c *gin.Context) {
		resp := client.Send("order.created", map[string]any{
			"order_id": "12345",
			"amount":   99.99,
		})

		if !resp.Success {
			c.JSON(http.StatusInternalServerError, gin.H{"error": resp.Error.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "Webhook sent!",
			"msgId":     resp.MessageID,
			"signature": resp.Signature,
		})
	})

	// Trigger custom event
	r.POST("/trigger/:event", func(c *gin.Context) {
		event := c.Param("event")

		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
			return
		}

		resp := client.Send(event, data)

		if !resp.Success {
			c.JSON(http.StatusInternalServerError, gin.H{"error": resp.Error.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Webhook sent!",
			"event":   event,
			"msgId":   resp.MessageID,
		})
	})

	port := getEnv("PORT", "8080")
	log.Printf("🚀 Gin + Webhook server running on :%s", port)
	r.Run(":" + port)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
