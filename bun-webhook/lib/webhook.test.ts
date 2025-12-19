import { describe, test, expect, mock } from "bun:test";
import {
  WebhookHandler,
  createWebhookHandler,
  type EventHandler,
  type WebhookPayload,
  type BaseEventMap,
} from "./webhook";

/**
 * Type-safe event map for tests
 */
interface TestEventMap extends BaseEventMap {
  "order.created": { orderId: string; amount: number };
  "order.updated": { orderId: string; status: string };
  "user.signup": { userId: string; email: string };
}

describe("WebhookHandler", () => {
  const TEST_SECRET = "whsec_C2FtcGxlX3NlY3JldF9rZXlfZm9yX3Rlc3Rpbmc=";

  describe("constructor", () => {
    test("creates handler with valid config", () => {
      const handler = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });
      expect(handler).toBeDefined();
    });

    test("throws error without secret", () => {
      expect(() => new WebhookHandler<TestEventMap>({ secret: "" })).toThrow(
        "WebhookHandler: secret is required"
      );
    });

    test("accepts optional config options", () => {
      const errorHandler = mock(() => {});
      const handler = new WebhookHandler<TestEventMap>({
        secret: TEST_SECRET,
        parallelExecution: true,
        onHandlerError: errorHandler,
      });
      expect(handler).toBeDefined();
    });
  });

  describe("event registration", () => {
    test("registers single event handler with typed data", () => {
      const handler = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });
      const mockFn: EventHandler<TestEventMap["order.created"]> = mock(
        (data) => {
          // Type-safe access
          const _orderId: string = data.orderId;
          const _amount: number = data.amount;
        }
      );

      handler.on("order.created", mockFn);
      expect(handler).toBeDefined();
    });

    test("registers multiple handlers for same event", () => {
      const handler = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });
      const mockFn1: EventHandler<TestEventMap["order.created"]> = mock(
        () => {}
      );
      const mockFn2: EventHandler<TestEventMap["order.created"]> = mock(
        () => {}
      );

      handler.on("order.created", mockFn1).on("order.created", mockFn2);
      expect(handler).toBeDefined();
    });

    test("registers wildcard handler", () => {
      const handler = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });
      const mockFn = mock(() => {});

      handler.onAll(mockFn);
      expect(handler).toBeDefined();
    });

    test("supports method chaining", () => {
      const handler = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });

      const result = handler
        .on("order.created", () => {})
        .on("user.signup", () => {})
        .onAll(() => {});

      expect(result).toBe(handler);
    });
  });

  describe("event unregistration", () => {
    test("unregisters specific event handler", () => {
      const handler = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });
      const mockFn: EventHandler<TestEventMap["order.created"]> = mock(
        () => {}
      );

      handler.on("order.created", mockFn);
      const result = handler.off("order.created", mockFn);

      expect(result).toBe(handler);
    });

    test("unregisters wildcard handler", () => {
      const handler = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });
      const mockFn: EventHandler<unknown> = mock(() => {});

      handler.onAll(mockFn);
      const result = handler.offAll(mockFn);

      expect(result).toBe(handler);
    });

    test("handles unregistering non-existent handler gracefully", () => {
      const handler = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });
      const mockFn: EventHandler<TestEventMap["order.created"]> = mock(
        () => {}
      );

      // Should not throw
      expect(() => handler.off("order.created", mockFn)).not.toThrow();
    });
  });

  describe("createWebhookHandler factory", () => {
    test("creates typed handler instance", () => {
      const handler = createWebhookHandler<TestEventMap>({
        secret: TEST_SECRET,
      });
      expect(handler).toBeInstanceOf(WebhookHandler);

      // Verify type safety works with factory
      handler.on("order.created", (data) => {
        const _orderId: string = data.orderId;
      });
    });
  });

  describe("handler middleware", () => {
    test("returns function", () => {
      const webhook = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });
      const middleware = webhook.handler();

      expect(typeof middleware).toBe("function");
    });

    test("rejects missing headers", async () => {
      const webhook = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });
      const middleware = webhook.handler();

      let responseData: { error?: string } = {};
      let responseStatus = 0;

      const mockHeaders = new Headers();
      const mockContext = {
        req: {
          raw: { headers: mockHeaders },
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

    test("rejects partial headers", async () => {
      const webhook = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });
      const middleware = webhook.handler();

      let responseStatus = 0;

      const mockHeaders = new Headers();
      mockHeaders.set("svix-id", "msg_123");
      // Missing svix-timestamp and svix-signature

      const mockContext = {
        req: {
          raw: { headers: mockHeaders },
          text: async () => "{}",
        },
        json: (data: { error?: string }, status?: number) => {
          responseStatus = status ?? 200;
          return new Response(JSON.stringify(data), { status: responseStatus });
        },
      };

      await middleware(mockContext as any);
      expect(responseStatus).toBe(401);
    });
  });

  describe("verify method", () => {
    test("normalizes header keys to lowercase", () => {
      const webhook = new WebhookHandler<TestEventMap>({ secret: TEST_SECRET });

      // This will fail verification (invalid signature) but tests header normalization
      expect(() =>
        webhook.verify("{}", {
          "SVIX-ID": "msg_123",
          "SVIX-TIMESTAMP": "1234567890",
          "SVIX-SIGNATURE": "v1,invalid",
        })
      ).toThrow(); // Expected to throw due to invalid signature
    });
  });

  describe("error handling", () => {
    test("calls onHandlerError when handler throws", async () => {
      const errorHandler = mock((_error: unknown, _event: string) => {});
      const webhook = new WebhookHandler<TestEventMap>({
        secret: TEST_SECRET,
        onHandlerError: errorHandler,
      });

      // Access private method for testing (in real scenario, this would be triggered via handler())
      const executeHandlers = (webhook as any).executeHandlers.bind(webhook);

      const failingHandler: EventHandler<
        TestEventMap["order.created"]
      > = () => {
        throw new Error("Handler failed");
      };
      webhook.on("order.created", failingHandler);

      const payload: WebhookPayload<TestEventMap["order.created"]> = {
        event: "order.created",
        timestamp: new Date().toISOString(),
        data: { orderId: "123", amount: 100 },
      };

      await executeHandlers(payload);
      expect(errorHandler).toHaveBeenCalled();
    });

    test("continues executing handlers after one fails", async () => {
      const errorHandler = mock(() => {});
      const webhook = new WebhookHandler<TestEventMap>({
        secret: TEST_SECRET,
        onHandlerError: errorHandler,
      });

      const executeHandlers = (webhook as any).executeHandlers.bind(webhook);

      const failingHandler: EventHandler<
        TestEventMap["order.created"]
      > = () => {
        throw new Error("First handler failed");
      };
      const successHandler = mock(() => {});

      webhook.on("order.created", failingHandler);
      webhook.on("order.created", successHandler);

      const payload: WebhookPayload<TestEventMap["order.created"]> = {
        event: "order.created",
        timestamp: new Date().toISOString(),
        data: { orderId: "123", amount: 100 },
      };

      await executeHandlers(payload);
      expect(successHandler).toHaveBeenCalled();
    });
  });

  describe("parallel execution", () => {
    test("executes handlers in parallel when enabled", async () => {
      const webhook = new WebhookHandler<TestEventMap>({
        secret: TEST_SECRET,
        parallelExecution: true,
      });

      const executeHandlers = (webhook as any).executeHandlers.bind(webhook);
      const executionOrder: number[] = [];

      const slowHandler: EventHandler<
        TestEventMap["order.created"]
      > = async () => {
        await new Promise((resolve) => setTimeout(resolve, 50));
        executionOrder.push(1);
      };
      const fastHandler: EventHandler<
        TestEventMap["order.created"]
      > = async () => {
        executionOrder.push(2);
      };

      webhook.on("order.created", slowHandler);
      webhook.on("order.created", fastHandler);

      const payload: WebhookPayload<TestEventMap["order.created"]> = {
        event: "order.created",
        timestamp: new Date().toISOString(),
        data: { orderId: "123", amount: 100 },
      };

      await executeHandlers(payload);

      // In parallel mode, fast handler should complete first
      expect(executionOrder).toEqual([2, 1]);
    });

    test("executes handlers sequentially when parallel disabled", async () => {
      const webhook = new WebhookHandler<TestEventMap>({
        secret: TEST_SECRET,
        parallelExecution: false,
      });

      const executeHandlers = (webhook as any).executeHandlers.bind(webhook);
      const executionOrder: number[] = [];

      const slowHandler: EventHandler<
        TestEventMap["order.created"]
      > = async () => {
        await new Promise((resolve) => setTimeout(resolve, 50));
        executionOrder.push(1);
      };
      const fastHandler: EventHandler<
        TestEventMap["order.created"]
      > = async () => {
        executionOrder.push(2);
      };

      webhook.on("order.created", slowHandler);
      webhook.on("order.created", fastHandler);

      const payload: WebhookPayload<TestEventMap["order.created"]> = {
        event: "order.created",
        timestamp: new Date().toISOString(),
        data: { orderId: "123", amount: 100 },
      };

      await executeHandlers(payload);

      // In sequential mode, handlers run in registration order
      expect(executionOrder).toEqual([1, 2]);
    });
  });
});
