import { createHmac, timingSafeEqual } from "crypto";

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

// Event handlers
const eventHandlers: Record<string, (data: unknown) => void> = {
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

const server = Bun.serve({
  port: PORT,
  async fetch(req) {
    const url = new URL(req.url);

    // Health check endpoint
    if (url.pathname === "/health" && req.method === "GET") {
      return new Response("OK", { status: 200 });
    }

    // Webhook endpoint
    if (url.pathname === "/webhook" && req.method === "POST") {
      const signature = req.headers.get("X-Webhook-Signature");
      const timestamp = req.headers.get("X-Webhook-Timestamp");

      // Validate required headers
      if (!signature || !timestamp) {
        return new Response("Missing signature or timestamp", { status: 401 });
      }

      // Check timestamp freshness (prevent replay attacks)
      const webhookTime = new Date(timestamp).getTime();
      const now = Date.now();
      if (Math.abs(now - webhookTime) > 5 * 60 * 1000) {
        return new Response("Webhook timestamp too old", { status: 401 });
      }

      // Get raw body for signature verification
      const rawBody = await req.text();

      // Verify signature
      if (!verifySignature(rawBody, signature)) {
        return new Response("Invalid signature", { status: 401 });
      }

      // Parse and process payload
      try {
        const payload: WebhookPayload = JSON.parse(rawBody);
        console.log(`\n🔔 Received webhook: ${payload.event}`);

        // Route to appropriate handler
        const handler = eventHandlers[payload.event];
        if (handler) {
          handler(payload.data);
        } else {
          console.log(`⚠️ Unknown event type: ${payload.event}`);
        }

        return new Response("Webhook received", { status: 200 });
      } catch (error) {
        console.error("Failed to parse webhook:", error);
        return new Response("Invalid JSON payload", { status: 400 });
      }
    }

    return new Response("Not Found", { status: 404 });
  },
});

console.log(`🚀 Bun webhook server running on http://localhost:${PORT}`);
