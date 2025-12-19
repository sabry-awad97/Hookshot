import { Hono } from "hono";
import { logger } from "hono/logger";
import { secureHeaders } from "hono/secure-headers";
import { Webhook } from "svix";

const app = new Hono();
const PORT = 4000;

// Must match the secret used in Go server
const WEBHOOK_SECRET = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw";

interface WebhookPayload {
  event: string;
  timestamp: string;
  data: Record<string, unknown>;
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

// Webhook endpoint with Svix verification
app.post("/webhook", async (c) => {
  const svixId = c.req.header("svix-id");
  const svixTimestamp = c.req.header("svix-timestamp");
  const svixSignature = c.req.header("svix-signature");

  // Validate required Svix headers
  if (!svixId || !svixTimestamp || !svixSignature) {
    console.log("❌ Missing Svix headers");
    return c.json({ error: "Missing Svix headers" }, 401);
  }

  const rawBody = await c.req.text();

  // Verify webhook using Svix SDK
  const wh = new Webhook(WEBHOOK_SECRET);

  try {
    // Svix verify handles signature validation and timestamp checking
    const payload = wh.verify(rawBody, {
      "svix-id": svixId,
      "svix-timestamp": svixTimestamp,
      "svix-signature": svixSignature,
    }) as WebhookPayload;

    console.log(`\n🔔 Verified webhook: ${payload.event} (ID: ${svixId})`);

    // Route to appropriate handler
    const handler = eventHandlers[payload.event];
    if (handler) {
      await handler(payload.data);
    } else {
      console.log(`⚠️ Unknown event type: ${payload.event}`);
    }

    return c.json({
      message: "Webhook verified and processed",
      event: payload.event,
      msgId: svixId,
    });
  } catch (error) {
    console.error("❌ Webhook verification failed:", error);
    return c.json({ error: "Invalid webhook signature" }, 401);
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

console.log(
  `🚀 Hono + Svix webhook server running on http://localhost:${PORT}`
);
