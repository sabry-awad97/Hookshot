# bun-webhook

Webhook listener library for Bun/TypeScript with [Svix](https://svix.com) signature verification.

## Installation

```bash
bun install
```

## Usage

```bash
bun run index.ts
```

## Library: `lib/webhook`

A reusable webhook handler with type-safe event definitions.

### Basic Usage

```typescript
import { WebhookHandler } from "./lib/webhook";

const webhook = new WebhookHandler({ secret: process.env.WEBHOOK_SECRET! });

webhook
  .on("order.created", (data) => console.log("Order:", data))
  .onAll((data, payload) => console.log("Event:", payload.event));

app.post("/webhook", webhook.handler());
```

### Type-Safe Events

```typescript
import { createWebhookHandler, type BaseEventMap } from "./lib/webhook";

interface MyEvents extends BaseEventMap {
  "order.created": { orderId: string; amount: number };
  "user.signup": { userId: string; email: string };
}

const webhook = createWebhookHandler<MyEvents>({
  secret: process.env.WEBHOOK_SECRET!,
  parallelExecution: true,
  onHandlerError: (err, event) => console.error(`${event} failed:`, err),
});

webhook.on("order.created", (data) => {
  console.log(data.orderId); // typed as string
});
```

### Configuration Options

| Option              | Type       | Default    | Description                        |
| ------------------- | ---------- | ---------- | ---------------------------------- |
| `secret`            | `string`   | (required) | Svix signing secret (`whsec_...`)  |
| `parallelExecution` | `boolean`  | `false`    | Run handlers concurrently          |
| `onHandlerError`    | `function` | `() => {}` | Error callback for failed handlers |

### API

- `on(event, handler)` - Register handler for specific event
- `off(event, handler)` - Unregister event handler
- `onAll(handler)` - Register handler for all events
- `offAll(handler)` - Unregister wildcard handler
- `handler()` - Returns Hono-compatible middleware
- `verify(body, headers)` - Standalone signature verification

## Testing

```bash
bun test
```
