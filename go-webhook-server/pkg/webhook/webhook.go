package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
	svix "github.com/svix/svix-webhooks/go"
)

// Sentinel errors for error inspection
var (
	ErrClientError = errors.New("webhook: client error")
	ErrServerError = errors.New("webhook: server error")
	ErrNetwork     = errors.New("webhook: network error")
)

// Config holds the webhook client configuration
type Config struct {
	TargetURL   string        // URL to send webhooks to
	Secret      string        // Svix signing secret (whsec_...)
	MaxRetries  uint64        // Max retry attempts (default: 3)
	Timeout     time.Duration // HTTP timeout (default: 10s)
	MaxInterval time.Duration // Max backoff interval (default: 30s)
	Logger      *slog.Logger  // Optional structured logger
	HTTPClient  *http.Client  // Optional custom HTTP client
}

// Client is a reusable webhook sender
type Client struct {
	config Config
	signer *svix.Webhook
	http   *http.Client
	logger *slog.Logger
}

// Payload represents a generic webhook payload
type Payload struct {
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// Response contains the result of a webhook send
type Response struct {
	Success    bool
	StatusCode int
	MessageID  string
	Error      error
}

// Option is a functional option for configuring the Client
type Option func(*Config)

// WithMaxRetries sets the maximum number of retry attempts
func WithMaxRetries(n uint64) Option {
	return func(c *Config) {
		c.MaxRetries = n
	}
}

// WithTimeout sets the HTTP request timeout
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// WithMaxInterval sets the maximum backoff interval
func WithMaxInterval(d time.Duration) Option {
	return func(c *Config) {
		c.MaxInterval = d
	}
}

// WithLogger sets a custom structured logger
func WithLogger(l *slog.Logger) Option {
	return func(c *Config) {
		c.Logger = l
	}
}

// WithHTTPClient sets a custom HTTP client for connection pooling
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) {
		c.HTTPClient = client
	}
}

// NewClient creates a new webhook client using functional options
func NewClient(targetURL, secret string, opts ...Option) (*Client, error) {
	if targetURL == "" {
		return nil, fmt.Errorf("webhook: targetURL is required")
	}
	if secret == "" {
		return nil, fmt.Errorf("webhook: secret is required")
	}

	cfg := Config{
		TargetURL:   targetURL,
		Secret:      secret,
		MaxRetries:  3,
		Timeout:     10 * time.Second,
		MaxInterval: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	signer, err := svix.NewWebhook(cfg.Secret)
	if err != nil {
		return nil, fmt.Errorf("webhook: failed to create signer: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.Timeout}
	}

	return &Client{
		config: cfg,
		signer: signer,
		http:   httpClient,
		logger: logger,
	}, nil
}

// Send dispatches a webhook with the given event and data
func (c *Client) Send(ctx context.Context, event string, data any) Response {
	payload := Payload{
		Event:     event,
		Timestamp: time.Now(),
		Data:      data,
	}
	return c.SendPayload(ctx, payload)
}

// SendPayload dispatches a custom payload.
// Note: The signing timestamp is generated at send time and may differ from payload.Timestamp.
func (c *Client) SendPayload(ctx context.Context, payload Payload) Response {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return Response{Error: fmt.Errorf("webhook: failed to marshal payload: %w", err)}
	}

	msgID := fmt.Sprintf("msg_%s", uuid.New().String())
	signingTimestamp := time.Now()

	signature, err := c.signer.Sign(msgID, signingTimestamp, jsonData)
	if err != nil {
		return Response{Error: fmt.Errorf("webhook: failed to sign: %w", err)}
	}

	return c.sendWithRetry(ctx, jsonData, msgID, signingTimestamp, signature)
}

func (c *Client) sendWithRetry(ctx context.Context, payload []byte, msgID string, timestamp time.Time, signature string) Response {
	var lastErr error
	var lastStatusCode int

	// Configure exponential backoff with jitter
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = 1 * time.Second
	expBackoff.MaxInterval = c.config.MaxInterval
	expBackoff.MaxElapsedTime = 0 // control via MaxRetries instead

	// Wrap with retry limit and context
	retries := c.config.MaxRetries
	if retries > 0 {
		retries--
	}
	b := backoff.WithMaxRetries(expBackoff, retries)
	b = backoff.WithContext(b, ctx)

	operation := func() error {
		req, err := http.NewRequestWithContext(ctx, "POST", c.config.TargetURL, bytes.NewReader(payload))
		if err != nil {
			lastErr = fmt.Errorf("%w: %v", ErrNetwork, err)
			return lastErr
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("svix-id", msgID)
		req.Header.Set("svix-timestamp", fmt.Sprintf("%d", timestamp.Unix()))
		req.Header.Set("svix-signature", signature)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%w: %v", ErrNetwork, err)
			c.logger.Warn("webhook: network error", "error", err)
			return lastErr
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		lastStatusCode = resp.StatusCode

		// 4xx - permanent failure, don't retry
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			lastErr = fmt.Errorf("%w: status %d: %s", ErrClientError, resp.StatusCode, string(body))
			return backoff.Permanent(lastErr)
		}

		// 5xx - retryable
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("%w: status %d: %s", ErrServerError, resp.StatusCode, string(body))
			c.logger.Warn("webhook: server error", "status", resp.StatusCode)
			return lastErr
		}

		return nil
	}

	if err := backoff.Retry(operation, b); err != nil {
		return Response{Error: lastErr, StatusCode: lastStatusCode}
	}

	return Response{
		Success:    true,
		StatusCode: lastStatusCode,
		MessageID:  msgID,
	}
}
