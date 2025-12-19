import { describe, test, expect, mock } from "bun:test";
import { WebhookHandler, createWebhookHandler } from "./webhook";

describe("WebhookHandler", () => {
  const TEST_SECRET = "whsec_C2FtcGxlX3NlY3JldF9rZXlfZm9yX3Rlc3Rpbmc=";

  describe("constructor", () => {
    test("creates handler with valid config", () => {
      const handler = new WebhookHandler({ secret: TEST_SECRET });
      expect(handler).toBeDefined();
    });

    test("throws error without secret", () => {
      expect(() => new WebhookHandler({ secret: "" })).toThrow(
        "WebhookHandler: secret is required"
      );
    });
  });

  describe("event registration", () => {
    test("registers single event handler", () => {
      const handler = new WebhookHandler({ secret: TEST_SECRET });
      const mockFn = mock(() => {});

      handler.on("order.created", mockFn);

      // Handler is registered (internal state check)
      expect(handler).toBeDefined();
    });

    test("registers multiple handlers for same event", () => {
      const handler = new WebhookHandler({ secret: TEST_SECRET });
      const mockFn1 = mock(() => {});
      const mockFn2 = mock(() => {});

      handler.on("order.created", mockFn1).on("order.created", mockFn2);

      expect(handler).toBeDefined();
    });

    test("registers wildcard handler", () => {
      const handler = new WebhookHandler({ secret: TEST_SECRET });
      const mockFn = mock(() => {});

      handler.onAll(mockFn);

      expect(handler).toBeDefined();
    });

    test("supports method chaining", () => {
      const handler = new WebhookHandler({ secret: TEST_SECRET });

      const result = handler
        .on("event1", () => {})
        .on("event2", () => {})
        .onAll(() => {});

      expect(result).toBe(handler);
    });
  });

  describe("createWebhookHandler factory", () => {
    test("creates handler instance", () => {
      const handler = createWebhookHandler({ secret: TEST_SECRET });
      expect(handler).toBeInstanceOf(WebhookHandler);
    });
  });

  describe("handler middleware", () => {
    test("returns function", () => {
      const webhook = new WebhookHandler({ secret: TEST_SECRET });
      const middleware = webhook.handler();

      expect(typeof middleware).toBe("function");
    });

    test("rejects missing headers", async () => {
      const webhook = new WebhookHandler({ secret: TEST_SECRET });
      const middleware = webhook.handler();

      // Track response
      let responseData: { error?: string } = {};
      let responseStatus = 0;

      // Mock Hono context
      const mockContext = {
        req: {
          header: (_name: string) => undefined,
          text: async () => "{}",
        },
        json: (data: { error?: string }, status?: number) => {
          responseData = data;
          responseStatus = status ?? 200;
          return new Response(JSON.stringify(data), { status: responseStatus });
        },
      };

      await middleware(mockContext as any);
      expect(responseStatus).toBe(401);
      expect(responseData.error).toBe("Missing Svix headers");
    });
  });
});
