import { Hono } from "hono";
import { logger } from "hono/logger";
import { secureHeaders } from "hono/secure-headers";
import { createHmac, timingSafeEqual } from "crypto";

const app = new Hono();
const SECRET = "your-shared-secret-key";
const PORT = 4000;

interface WebhookPayload {
  event: string;
  timestamp: string;
  data: Record<string, unknown>;
}

// Verify HMAC signature
function verifySignature(payload: string, signature: string): boolean {
  const expectedSignature = createHmac("sha256", SECRET)
    .update(payload)
    .digest("hex");

  try {
    return timingSafeEqual(
      Buffer.from(signature),
      Buffer.from(expectedSignature)
    );
  } catch {
    return false;
  }
}

// Event handlers registry
const eventHandlers: Record<string, (data: unknown) => void | Promise<void>> = {
  "order.created": (data) => {
    console.log("📦 New order created:", data);
  },
  "order.updated": (data) => {
    console.log("✏️ Order updated:", data);
  },
  "payment.received": (data) => {
    console.log("💰 Payment received:", data);
  },
};

// Middleware
app.use("*", logger());
app.use("*", secureHeaders());

// Health check endpoint
app.get("/health", (c) => {
  return c.json({ status: "ok" });
});

// Webhook endpoint
app.post("/webhook", async (c) => {
  const signature = c.req.header("X-Webhook-Signature");
  const timestamp = c.req.header("X-Webhook-Timestamp");

  // Validate required headers
  if (!signature || !timestamp) {
    return c.json({ error: "Missing signature or timestamp" }, 401);
  }

  // Check timestamp freshness (prevent replay attacks - 5 min window)
  const webhookTime = new Date(timestamp).getTime();
  const now = Date.now();
  if (Math.abs(now - webhookTime) > 5 * 60 * 1000) {
    return c.json({ error: "Webhook timestamp too old" }, 401);
  }

  // Get raw body for signature verification
  const rawBody = await c.req.text();

  // Verify HMAC signature
  if (!verifySignature(rawBody, signature)) {
    return c.json({ error: "Invalid signature" }, 401);
  }

  // Parse and process payload
  try {
    const payload: WebhookPayload = JSON.parse(rawBody);
    console.log(`\n🔔 Received webhook: ${payload.event}`);

    // Route to appropriate handler
    const handler = eventHandlers[payload.event];
    if (handler) {
      await handler(payload.data);
    } else {
      console.log(`⚠️ Unknown event type: ${payload.event}`);
    }

    return c.json({ message: "Webhook received", event: payload.event });
  } catch (error) {
    console.error("Failed to parse webhook:", error);
    return c.json({ error: "Invalid JSON payload" }, 400);
  }
});

// Catch-all 404
app.notFound((c) => {
  return c.json({ error: "Not Found" }, 404);
});

// Error handler
app.onError((err, c) => {
  console.error("Server error:", err);
  return c.json({ error: "Internal Server Error" }, 500);
});

export default {
  port: PORT,
  fetch: app.fetch,
};

console.log(`🚀 Hono webhook server running on http://localhost:${PORT}`);
