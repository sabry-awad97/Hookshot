import { Webhook } from "svix";
import type { Context } from "hono";

/**
 * WebhookHandler configuration
 */
export interface WebhookConfig {
  /** Svix signing secret (whsec_...) */
  secret: string;
  /** Whether to run handlers in parallel (default: false) */
  parallelExecution?: boolean;
  /** Error handler for failed event handlers */
  onHandlerError?: (error: unknown, event: string) => void;
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
 * Base event map constraint - use specific interfaces for type-safe events
 */
// eslint-disable-next-line @typescript-eslint/no-empty-object-type
export interface BaseEventMap {}

/**
 * Resolved config with defaults applied
 */
interface ResolvedConfig {
  secret: string;
  parallelExecution: boolean;
  onHandlerError: (error: unknown, event: string) => void;
}

/**
 * Reusable webhook handler with Svix verification
 */
export class WebhookHandler<TEventMap extends BaseEventMap = BaseEventMap> {
  private svix: Webhook;
  private handlers: Map<string, EventHandler[]> = new Map();
  private wildcardHandlers: EventHandler[] = [];
  private config: ResolvedConfig;

  constructor(config: WebhookConfig) {
    if (!config.secret) {
      throw new Error("WebhookHandler: secret is required");
    }

    this.config = {
      secret: config.secret,
      parallelExecution: config.parallelExecution ?? false,
      onHandlerError: config.onHandlerError ?? (() => {}),
    };

    this.svix = new Webhook(this.config.secret);
  }

  /**
   * Register an event handler
   */
  on<K extends keyof TEventMap & string>(
    event: K,
    handler: EventHandler<TEventMap[K]>
  ): this {
    const handlers = this.handlers.get(event) ?? [];
    handlers.push(handler as EventHandler);
    this.handlers.set(event, handlers);
    return this;
  }

  /**
   * Unregister an event handler
   */
  off<K extends keyof TEventMap & string>(
    event: K,
    handler: EventHandler<TEventMap[K]>
  ): this {
    const handlers = this.handlers.get(event);
    if (handlers) {
      const idx = handlers.indexOf(handler as EventHandler);
      if (idx > -1) handlers.splice(idx, 1);
    }
    return this;
  }

  /**
   * Register handler for all events
   */
  onAll<T = unknown>(handler: EventHandler<T>): this {
    this.wildcardHandlers.push(handler as EventHandler);
    return this;
  }

  /**
   * Unregister a wildcard handler
   */
  offAll<T = unknown>(handler: EventHandler<T>): this {
    const idx = this.wildcardHandlers.indexOf(handler as EventHandler);
    if (idx > -1) this.wildcardHandlers.splice(idx, 1);
    return this;
  }

  /**
   * Create Hono-compatible handler middleware
   */
  handler() {
    return async (c: Context) => {
      const headers = c.req.raw.headers;
      const svixId = this.getHeader(headers, "svix-id");
      const svixTimestamp = this.getHeader(headers, "svix-timestamp");
      const svixSignature = this.getHeader(headers, "svix-signature");

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
    const normalized = this.normalizeHeaders(headers);
    return this.svix.verify(body, normalized) as WebhookPayload;
  }

  /**
   * Get header value case-insensitively
   */
  private getHeader(headers: Headers, name: string): string | null {
    return headers.get(name);
  }

  /**
   * Normalize header keys to lowercase
   */
  private normalizeHeaders(
    headers: Record<string, string>
  ): Record<string, string> {
    const normalized: Record<string, string> = {};
    for (const [key, value] of Object.entries(headers)) {
      normalized[key.toLowerCase()] = value;
    }
    return normalized;
  }

  private async executeHandlers(payload: WebhookPayload): Promise<void> {
    const { event, data } = payload;
    const handlers = this.handlers.get(event) ?? [];
    const allHandlers = [...handlers, ...this.wildcardHandlers];

    if (this.config.parallelExecution) {
      await Promise.allSettled(
        allHandlers.map((handler) =>
          Promise.resolve(handler(data, payload)).catch((err) =>
            this.config.onHandlerError(err, event)
          )
        )
      );
    } else {
      for (const handler of allHandlers) {
        try {
          await handler(data, payload);
        } catch (err) {
          this.config.onHandlerError(err, event);
        }
      }
    }
  }
}

/**
 * Create a webhook handler (factory function)
 */
export function createWebhookHandler<
  TEventMap extends BaseEventMap = BaseEventMap
>(config: WebhookConfig): WebhookHandler<TEventMap> {
  return new WebhookHandler<TEventMap>(config);
}
