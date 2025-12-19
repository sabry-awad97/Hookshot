# 🪝 Hookshot

A cross-language webhook system using **Svix** for secure, battle-tested webhook communication between Go and Bun/TypeScript.

## Architecture

```
┌─────────────────────┐         ┌─────────────────────┐
│   Go (Gin + Svix)   │ ──────► │  Bun (Hono + Svix)  │
│   Webhook Sender    │  HTTPS  │  Webhook Listener   │
│   Port: 8080        │         │   Port: 4000        │
└─────────────────────┘         └─────────────────────┘
```

## Features

- ✅ **Svix SDK** - Industry-standard webhook signatures
- ✅ **Gin** - High-performance Go web framework
- ✅ **Hono** - Lightweight TypeScript web framework
- ✅ **HMAC-SHA256** - Cryptographic signature verification
- ✅ **Replay Protection** - Timestamp validation
- ✅ **Event Routing** - Extensible event handler registry

## Quick Start

### 1. Install Dependencies

```bash
# Go server
cd go-webhook-server
go mod tidy

# Bun listener
cd ../bun-webhook
bun install
```

### 2. Start Services

```bash
# Terminal 1 - Start Bun listener
cd bun-webhook
bun run index.ts

# Terminal 2 - Start Go sender
cd go-webhook-server
go run main.go
```

### 3. Trigger Webhooks

```bash
# Simple trigger
curl -X POST http://localhost:8080/trigger

# Custom event with payload
curl -X POST http://localhost:8080/trigger/payment.received \
  -H "Content-Type: application/json" \
  -d '{"transaction_id": "TXN-001", "amount": 150.00}'
```

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

## Configuration

Both services use a shared Svix secret key for signing/verification:

```
whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw
```

> ⚠️ **Production**: Replace with a secure secret and use environment variables.

## Adding Event Handlers

### Bun (`index.ts`)

```typescript
const eventHandlers: Record<string, (data: unknown) => void> = {
  "order.created": (data) => console.log("Order:", data),
  "your.custom.event": (data) => {
    // Handle your event
  },
};
```

## Tech Stack

| Component    | Technology | Version                    |
| ------------ | ---------- | -------------------------- |
| **Sender**   | Go + Gin   | 1.11.0                     |
| **Listener** | Bun + Hono | 4.11.1                     |
| **Signing**  | Svix       | 1.83.0 (Go) / 1.82.0 (Bun) |

## License

MIT
