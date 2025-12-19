import { Hono } from "hono";
import { logger } from "hono/logger";
import { secureHeaders } from "hono/secure-headers";
import { WebhookHandler } from "./lib/webhook";

const app = new Hono();

// Configuration from environment
const PORT = process.env.PORT ? parseInt(process.env.PORT) : 4000;
const WEBHOOK_SECRET =
  process.env.WEBHOOK_SECRET ??
  "whsec_C2FtcGxlX3NlY3JldF9rZXlfZm9yX3Rlc3Rpbmc=";

// Create reusable webhook handler
const webhook = new WebhookHandler({ secret: WEBHOOK_SECRET });

// Register event handlers
webhook
  .on("order.created", (data) => {
    console.log("📦 New order created:", data);
  })
  .on("order.updated", (data) => {
    console.log("✏️ Order updated:", data);
  })
  .on("payment.received", (data) => {
    console.log("💰 Payment received:", data);
  })
  .onAll((data, payload) => {
    // Log all events (optional)
    console.log(`\n🔔 Received: ${payload.event}`);
  });

// Middleware
app.use("*", logger());
app.use("*", secureHeaders());

// Health check
app.get("/health", (c) => c.json({ status: "ok" }));

// Webhook endpoint using the handler
app.post("/webhook", webhook.handler());

// 404 handler
app.notFound((c) => c.json({ error: "Not Found" }, 404));

// Error handler
app.onError((err, c) => {
  console.error("Server error:", err);
  return c.json({ error: "Internal Server Error" }, 500);
});

export default {
  port: PORT,
  fetch: app.fetch,
};

console.log(`🚀 Hono + Webhook server running on http://localhost:${PORT}`);
