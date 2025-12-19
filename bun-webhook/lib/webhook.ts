import { Webhook } from "svix";
import type { Context } from "hono";

/**
 * WebhookHandler configuration
 */
export interface WebhookConfig {
  /** Svix signing secret (whsec_...) */
  secret: string;
  /** Timestamp tolerance in ms (default: 5 minutes) */
  timestampTolerance?: number;
}

/**
 * Generic webhook payload structure
 */
export interface WebhookPayload<T = unknown> {
  event: string;
  timestamp: string;
  data: T;
}

/**
 * Event handler function type
 */
export type EventHandler<T = unknown> = (
  data: T,
  payload: WebhookPayload<T>
) => void | Promise<void>;

/**
 * Reusable webhook handler with Svix verification
 */
export class WebhookHandler {
  private svix: Webhook;
  private handlers: Map<string, EventHandler[]> = new Map();
  private wildcardHandlers: EventHandler[] = [];
  private config: Required<WebhookConfig>;

  constructor(config: WebhookConfig) {
    if (!config.secret) {
      throw new Error("WebhookHandler: secret is required");
    }

    this.config = {
      secret: config.secret,
      timestampTolerance: config.timestampTolerance ?? 5 * 60 * 1000,
    };

    this.svix = new Webhook(this.config.secret);
  }

  /**
   * Register an event handler
   */
  on<T = unknown>(event: string, handler: EventHandler<T>): this {
    if (event === "*") {
      this.wildcardHandlers.push(handler as EventHandler);
    } else {
      const handlers = this.handlers.get(event) ?? [];
      handlers.push(handler as EventHandler);
      this.handlers.set(event, handlers);
    }
    return this;
  }

  /**
   * Register handler for all events
   */
  onAll<T = unknown>(handler: EventHandler<T>): this {
    return this.on("*", handler);
  }

  /**
   * Create Hono-compatible handler middleware
   */
  handler() {
    return async (c: Context) => {
      const svixId = c.req.header("svix-id");
      const svixTimestamp = c.req.header("svix-timestamp");
      const svixSignature = c.req.header("svix-signature");

      // Validate headers
      if (!svixId || !svixTimestamp || !svixSignature) {
        return c.json({ error: "Missing Svix headers" }, 401);
      }

      const rawBody = await c.req.text();

      try {
        // Verify signature
        const payload = this.svix.verify(rawBody, {
          "svix-id": svixId,
          "svix-timestamp": svixTimestamp,
          "svix-signature": svixSignature,
        }) as WebhookPayload;

        // Execute handlers
        await this.executeHandlers(payload);

        return c.json({
          success: true,
          event: payload.event,
          msgId: svixId,
        });
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "Unknown error";
        return c.json({ error: "Verification failed", details: message }, 401);
      }
    };
  }

  /**
   * Standalone verify method for custom usage
   */
  verify(body: string, headers: Record<string, string>): WebhookPayload {
    return this.svix.verify(body, headers) as WebhookPayload;
  }

  private async executeHandlers(payload: WebhookPayload): Promise<void> {
    const { event, data } = payload;

    // Execute specific handlers
    const handlers = this.handlers.get(event) ?? [];
    for (const handler of handlers) {
      await handler(data, payload);
    }

    // Execute wildcard handlers
    for (const handler of this.wildcardHandlers) {
      await handler(data, payload);
    }
  }
}

/**
 * Create a webhook handler (factory function)
 */
export function createWebhookHandler(config: WebhookConfig): WebhookHandler {
  return new WebhookHandler(config);
}
