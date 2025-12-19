package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	svix "github.com/svix/svix-webhooks/go"
)

// Config holds the webhook client configuration
type Config struct {
	TargetURL  string        // URL to send webhooks to
	Secret     string        // Svix signing secret (whsec_...)
	MaxRetries int           // Max retry attempts (default: 3)
	Timeout    time.Duration // HTTP timeout (default: 10s)
}

// Client is a reusable webhook sender
type Client struct {
	config Config
	signer *svix.Webhook
	http   *http.Client
}

// Payload represents a generic webhook payload
type Payload struct {
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// Response contains the result of a webhook send
type Response struct {
	Success   bool
	MessageID string
	Signature string
	Error     error
}

// NewClient creates a new webhook client
func NewClient(cfg Config) (*Client, error) {
	if cfg.TargetURL == "" {
		return nil, fmt.Errorf("webhook: TargetURL is required")
	}
	if cfg.Secret == "" {
		return nil, fmt.Errorf("webhook: Secret is required")
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	signer, err := svix.NewWebhook(cfg.Secret)
	if err != nil {
		return nil, fmt.Errorf("webhook: failed to create signer: %w", err)
	}

	return &Client{
		config: cfg,
		signer: signer,
		http:   &http.Client{Timeout: cfg.Timeout},
	}, nil
}

// Send dispatches a webhook with the given event and data
func (c *Client) Send(event string, data any) Response {
	payload := Payload{
		Event:     event,
		Timestamp: time.Now(),
		Data:      data,
	}
	return c.SendPayload(payload)
}

// SendPayload dispatches a custom payload
func (c *Client) SendPayload(payload Payload) Response {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return Response{Error: fmt.Errorf("webhook: failed to marshal payload: %w", err)}
	}

	msgID := fmt.Sprintf("msg_%s", time.Now().Format("20060102150405.000"))
	timestamp := time.Now()

	signature, err := c.signer.Sign(msgID, timestamp, jsonData)
	if err != nil {
		return Response{Error: fmt.Errorf("webhook: failed to sign: %w", err)}
	}

	return c.sendWithRetry(jsonData, msgID, timestamp, signature)
}

func (c *Client) sendWithRetry(payload []byte, msgID string, timestamp time.Time, signature string) Response {
	var lastErr error

	for attempt := 0; attempt < c.config.MaxRetries; attempt++ {
		req, err := http.NewRequest("POST", c.config.TargetURL, bytes.NewReader(payload))
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("svix-id", msgID)
		req.Header.Set("svix-timestamp", timestamp.Format(time.RFC3339))
		req.Header.Set("svix-signature", signature)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			backoff := time.Duration(1<<attempt) * time.Second
			log.Printf("webhook: retry %d/%d in %v: %v", attempt+1, c.config.MaxRetries, backoff, err)
			time.Sleep(backoff)
			continue
		}
		defer resp.Body.Close()

		// Read response body
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("webhook: server returned %d: %s", resp.StatusCode, string(body))
			backoff := time.Duration(1<<attempt) * time.Second
			log.Printf("webhook: retry %d/%d in %v: %v", attempt+1, c.config.MaxRetries, backoff, lastErr)
			time.Sleep(backoff)
			continue
		}

		return Response{
			Success:   true,
			MessageID: msgID,
			Signature: signature,
		}
	}

	return Response{Error: lastErr}
}
