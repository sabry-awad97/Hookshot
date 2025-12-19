# 🪝 Hookshot

A cross-language webhook system with **reusable libraries** for Go and Bun/TypeScript, powered by [Svix](https://svix.com).

## Architecture

```
┌─────────────────────────┐         ┌─────────────────────────┐
│  Go (Gin + pkg/webhook) │ ──────► │ Bun (Hono + lib/webhook)│
│   Webhook Sender        │  HTTPS  │   Webhook Listener      │
│   Port: 8080            │         │   Port: 4000            │
└─────────────────────────┘         └─────────────────────────┘
```

## Features

- ✅ **Reusable Libraries** - Drop-in packages for any project
- ✅ **Svix SDK** - Industry-standard webhook signatures
- ✅ **Environment Configuration** - No hardcoded secrets
- ✅ **Retry with Backoff** - Automatic exponential retry
- ✅ **Event Registration** - Fluent handler API
- ✅ **Comprehensive Tests** - Go and Bun test suites

## Quick Start

### Install Dependencies

```bash
task install
```

### Start Services

```bash
# Terminal 1 - Bun listener
task dev:bun

# Terminal 2 - Go sender
task dev:go
```

### Test Webhooks

```bash
task test
```

## Reusable Libraries

### Go: `pkg/webhook`

```go
import "hookshot-server/pkg/webhook"

client, _ := webhook.NewClient(webhook.Config{
    TargetURL:  "https://example.com/webhook",
    Secret:     os.Getenv("WEBHOOK_SECRET"),
    MaxRetries: 3,
})

resp := client.Send("order.created", map[string]any{
    "id": "123",
})
```

### Bun: `lib/webhook`

```typescript
import { WebhookHandler } from "./lib/webhook";

const webhook = new WebhookHandler({
  secret: process.env.WEBHOOK_SECRET!,
});

webhook
  .on("order.created", (data) => console.log(data))
  .onAll((data, payload) => console.log(payload.event));

app.post("/webhook", webhook.handler());
```

## Configuration

| Variable             | Default                         | Description         |
| -------------------- | ------------------------------- | ------------------- |
| `WEBHOOK_SECRET`     | (test secret)                   | Svix signing secret |
| `WEBHOOK_TARGET_URL` | `http://localhost:4000/webhook` | Webhook destination |
| `PORT`               | 8080 (Go) / 4000 (Bun)          | Server port         |

## API Endpoints

### Go Sender (`:8080`)

| Method | Endpoint          | Description       |
| ------ | ----------------- | ----------------- |
| `GET`  | `/health`         | Health check      |
| `POST` | `/trigger`        | Send test webhook |
| `POST` | `/trigger/:event` | Send custom event |

### Bun Listener (`:4000`)

| Method | Endpoint   | Description      |
| ------ | ---------- | ---------------- |
| `GET`  | `/health`  | Health check     |
| `POST` | `/webhook` | Receive webhooks |

## Testing

```bash
# Run all tests
task test:go    # Go tests
bun test        # Bun tests

# Integration test
task test       # Full webhook flow
```

## License

MIT
