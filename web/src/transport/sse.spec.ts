import { SSETransport } from "./sse";
import { ConnectionState } from "./transport";
import { TransportMessage } from "./message";

// Mock EventSource
class MockEventSource {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSED = 2;

    readyState = MockEventSource.CONNECTING;
    url: string;
    private listeners: { [event: string]: Function[] } = {};

    constructor(url: string) {
        this.url = url;
        // Simulate async connection
        setTimeout(() => this.simulateOpen(), 0);
    }

    addEventListener(event: string, handler: Function) {
        if (!this.listeners[event]) {
            this.listeners[event] = [];
        }
        this.listeners[event].push(handler);
    }

    removeEventListener(event: string, handler: Function) {
        if (this.listeners[event]) {
            this.listeners[event] = this.listeners[event].filter(h => h !== handler);
        }
    }

    close() {
        this.readyState = MockEventSource.CLOSED;
    }

    simulateOpen() {
        this.readyState = MockEventSource.OPEN;
        this.emit("open", {});
    }

    simulateError(error?: any) {
        this.emit("error", error || {});
    }

    simulateMessage(data: string) {
        this.emit("message", { data });
    }

    private emit(event: string, data: any) {
        if (this.listeners[event]) {
            this.listeners[event].forEach(handler => handler(data));
        }
    }
}

// Mock fetch
const mockFetch = jest.fn();

// Replace globals with mocks
(global as any).EventSource = MockEventSource;
(global as any).fetch = mockFetch;
(global as any).location = {
    protocol: "http:",
    host: "localhost:8080",
    pathname: "/",
    search: "",
    hash: "",
};

describe("SSETransport", () => {
    let transport: SSETransport;
    let mockCookie: string;

    beforeEach(() => {
        mockCookie = "";
        Object.defineProperty(document, "cookie", {
            get: () => mockCookie,
            set: (value: string) => {
                mockCookie = value;
            },
            configurable: true,
        });

        mockFetch.mockClear();
        mockFetch.mockResolvedValue({
            ok: true,
            status: 200,
            statusText: "OK",
        });
    });

    afterEach(() => {
        if (transport) {
            transport.close();
        }
    });

    describe("Connection Establishment", () => {
        test("should connect successfully", async () => {
            transport = new SSETransport();
            await expect(transport.connect()).resolves.toBeUndefined();
            expect(transport.getState()).toBe(ConnectionState.Connected);
        });

        test("should start in Connecting state", () => {
            transport = new SSETransport();
            const promise = transport.connect();
            expect(transport.getState()).toBe(ConnectionState.Connecting);
            return promise;
        });

        test("should emit state change events", async () => {
            transport = new SSETransport();
            const states: ConnectionState[] = [];
            transport.onStateChange((state) => states.push(state));

            await transport.connect();

            expect(states).toContain(ConnectionState.Connecting);
            expect(states).toContain(ConnectionState.Connected);
        });

        test("should use custom endpoints", async () => {
            transport = new SSETransport({
                sseEndpoint: "/custom/sse",
                postEndpoint: "/custom/post",
            });

            await transport.connect();

            expect((transport as any).eventSource.url).toContain("/custom/sse");
        });
    });

    describe("Session ID Management", () => {
        test("should persist session ID to cookie", () => {
            transport = new SSETransport();
            expect(mockCookie).toContain("_psid=");
        });

        test("should read existing session ID from cookie", () => {
            mockCookie = "_psid=test-session-123; path=/";
            transport = new SSETransport();
            // Session ID is read from cookie on construction
            expect(mockCookie).toContain("test-session-123");
        });

        test("should set cookie with expiration", () => {
            transport = new SSETransport();
            expect(mockCookie).toContain("expires=");
            expect(mockCookie).toContain("path=/");
        });
    });

    describe("Message Handling", () => {
        test("should receive and parse SSE messages", async () => {
            transport = new SSETransport();
            const messages: TransportMessage[] = [];
            transport.onMessage((msg) => messages.push(msg));

            await transport.connect();

            const testMessage: TransportMessage = {
                t: "patch",
                i: 1,
                island: "counter-1",
                d: { patches: [] },
            };

            (transport as any).eventSource.simulateMessage(JSON.stringify(testMessage));

            expect(messages).toHaveLength(1);
            expect(messages[0]).toEqual(testMessage);
        });

        test("should handle message with island field", async () => {
            transport = new SSETransport();
            const messages: TransportMessage[] = [];
            transport.onMessage((msg) => messages.push(msg));

            await transport.connect();

            const testMessage: TransportMessage = {
                t: "patch",
                island: "my-island",
                d: [{ Anchor: "_i_my-island_0", Action: 1, HTML: "<div>test</div>" }],
            };

            (transport as any).eventSource.simulateMessage(JSON.stringify(testMessage));

            expect(messages[0].island).toBe("my-island");
        });

        test("should handle malformed JSON gracefully", async () => {
            transport = new SSETransport();
            const messages: TransportMessage[] = [];
            transport.onMessage((msg) => messages.push(msg));

            await transport.connect();

            const consoleSpy = jest.spyOn(console, "error").mockImplementation();

            // Send invalid JSON
            (transport as any).eventSource.simulateMessage("{invalid json}");

            expect(messages).toHaveLength(0);
            expect(consoleSpy).toHaveBeenCalled();
            consoleSpy.mockRestore();
        });
    });

    describe("Sending Messages via POST", () => {
        test("should POST events to server", async () => {
            transport = new SSETransport();
            await transport.connect();

            const message: TransportMessage = {
                t: "click",
                i: 1,
                island: "counter-1",
                d: { button: "increment" },
            };

            transport.send(message);

            // Wait for async fetch to complete
            await new Promise(resolve => setTimeout(resolve, 0));

            expect(mockFetch).toHaveBeenCalledWith(
                expect.stringContaining("/live/post"),
                expect.objectContaining({
                    method: "POST",
                    headers: expect.objectContaining({
                        "Content-Type": "application/json",
                    }),
                    body: JSON.stringify(message),
                })
            );
        });

        test("should include session ID in POST headers", async () => {
            mockCookie = "_psid=session-123; path=/";
            transport = new SSETransport();
            await transport.connect();

            transport.send({ t: "test" });

            // Wait for async fetch to complete
            await new Promise(resolve => setTimeout(resolve, 0));

            expect(mockFetch).toHaveBeenCalledWith(
                expect.anything(),
                expect.objectContaining({
                    headers: expect.objectContaining({
                        "X-Live-Session": "session-123",
                    }),
                })
            );
        });

        test("should not send when not connected", () => {
            transport = new SSETransport();
            // Don't connect

            const consoleSpy = jest.spyOn(console, "warn").mockImplementation();
            transport.send({ t: "test" });

            expect(consoleSpy).toHaveBeenCalledWith(
                "Cannot send: connection not ready",
                ConnectionState.Closed
            );
            expect(mockFetch).not.toHaveBeenCalled();
            consoleSpy.mockRestore();
        });

        test("should handle POST errors", async () => {
            mockFetch.mockRejectedValueOnce(new Error("Network error"));

            transport = new SSETransport();
            await transport.connect();

            const consoleSpy = jest.spyOn(console, "error").mockImplementation();

            transport.send({ t: "test" });

            // Wait for async fetch to complete
            await new Promise(resolve => setTimeout(resolve, 10));

            expect(consoleSpy).toHaveBeenCalledWith(
                "POST error",
                expect.any(Error)
            );
            consoleSpy.mockRestore();
        });

        test("should use custom POST endpoint", async () => {
            transport = new SSETransport({
                postEndpoint: "/custom/post",
            });
            await transport.connect();

            transport.send({ t: "test" });

            // Wait for async fetch to complete
            await new Promise(resolve => setTimeout(resolve, 0));

            expect(mockFetch).toHaveBeenCalledWith(
                expect.stringContaining("/custom/post"),
                expect.anything()
            );
        });
    });

    describe("Island Subscription", () => {
        test("should track subscribed islands", () => {
            transport = new SSETransport();
            transport.subscribeIsland("island-1");
            transport.subscribeIsland("island-2");

            const subscribed = transport.getSubscribedIslands();
            expect(subscribed).toContain("island-1");
            expect(subscribed).toContain("island-2");
        });

        test("should unsubscribe from islands", () => {
            transport = new SSETransport();
            transport.subscribeIsland("island-1");
            transport.subscribeIsland("island-2");
            transport.unsubscribeIsland("island-1");

            const subscribed = transport.getSubscribedIslands();
            expect(subscribed).not.toContain("island-1");
            expect(subscribed).toContain("island-2");
        });

        test("should re-subscribe islands after reconnection", async () => {
            transport = new SSETransport();
            transport.subscribeIsland("island-1");
            transport.subscribeIsland("island-2");

            await transport.connect();

            // Simulate error and reconnect
            (transport as any).eventSource.simulateError();

            // Wait for reconnection
            await new Promise(resolve => setTimeout(resolve, 150));

            // Islands should still be subscribed
            const subscribed = transport.getSubscribedIslands();
            expect(subscribed).toContain("island-1");
            expect(subscribed).toContain("island-2");
        });
    });

    describe("Reconnection with Backoff", () => {
        test("should reconnect after error", async () => {
            transport = new SSETransport();
            await transport.connect();

            const states: ConnectionState[] = [];
            transport.onStateChange((state) => states.push(state));

            // Simulate error
            (transport as any).eventSource.simulateError();

            expect(transport.getState()).toBe(ConnectionState.Reconnecting);

            // Wait for reconnection with backoff (100ms + jitter + connection time)
            await new Promise(resolve => setTimeout(resolve, 300));
            expect(transport.getState()).toBe(ConnectionState.Connected);
        });

        test("should use exponential backoff delays", () => {
            transport = new SSETransport();

            // Test backoff calculation directly
            expect((transport as any).getReconnectDelay()).toBeGreaterThanOrEqual(100);
            expect((transport as any).getReconnectDelay()).toBeLessThanOrEqual(120);

            (transport as any).reconnectAttempts = 1;
            expect((transport as any).getReconnectDelay()).toBeGreaterThanOrEqual(200);
            expect((transport as any).getReconnectDelay()).toBeLessThanOrEqual(240);

            (transport as any).reconnectAttempts = 2;
            expect((transport as any).getReconnectDelay()).toBeGreaterThanOrEqual(400);
            expect((transport as any).getReconnectDelay()).toBeLessThanOrEqual(480);
        });

        test("should cap backoff at max delay", () => {
            transport = new SSETransport();

            // Set many attempts to exceed max
            (transport as any).reconnectAttempts = 20;
            const delay = (transport as any).getReconnectDelay();

            // Should be capped at 5000ms + 10% jitter = 5500ms max
            expect(delay).toBeLessThanOrEqual(5500);
        });

        test("should reset backoff counter on successful connection", async () => {
            transport = new SSETransport();

            // Set some failed attempts
            (transport as any).reconnectAttempts = 5;

            await transport.connect();

            // Reconnect counter should be reset on successful connection
            expect((transport as any).reconnectAttempts).toBe(0);
        });

        test("should not reconnect after explicit close", async () => {
            transport = new SSETransport();
            await transport.connect();

            transport.close();

            // Simulate error (should not trigger reconnection)
            if ((transport as any).eventSource) {
                (transport as any).eventSource.simulateError();
            }

            // Wait to ensure no reconnection happens
            await new Promise(resolve => setTimeout(resolve, 200));

            // Should still be closed
            expect(transport.getState()).toBe(ConnectionState.Closed);
        });
    });

    describe("Connection Lifecycle", () => {
        test("should transition through states correctly", async () => {
            transport = new SSETransport();
            const states: ConnectionState[] = [];
            transport.onStateChange((state) => states.push(state));

            await transport.connect();
            transport.close();

            // State changes should include Connecting, Connected, and Closed
            expect(states).toContain(ConnectionState.Connecting);
            expect(states).toContain(ConnectionState.Connected);
            expect(states).toContain(ConnectionState.Closed);
        });

        test("should close cleanly", async () => {
            transport = new SSETransport();
            await transport.connect();

            const mockClose = jest.fn();
            (transport as any).eventSource.close = mockClose;

            transport.close();

            expect(mockClose).toHaveBeenCalled();
            expect(transport.getState()).toBe(ConnectionState.Closed);
        });

        test("should handle multiple close calls", async () => {
            transport = new SSETransport();
            await transport.connect();

            transport.close();
            transport.close(); // Should not throw

            expect(transport.getState()).toBe(ConnectionState.Closed);
        });

        test("should handle close when not connected", () => {
            transport = new SSETransport();
            // Don't connect

            expect(() => transport.close()).not.toThrow();
            expect(transport.getState()).toBe(ConnectionState.Closed);
        });
    });

    describe("Last-Event-ID Reconnection", () => {
        test("should automatically handle Last-Event-ID via EventSource", async () => {
            // EventSource automatically sends Last-Event-ID header on reconnect
            // We just need to verify the transport creates new EventSource on reconnect
            transport = new SSETransport();
            await transport.connect();

            const firstEventSource = (transport as any).eventSource;

            // Simulate error to trigger reconnect
            firstEventSource.simulateError();

            // Wait for reconnection to complete (backoff + connection time)
            await new Promise(resolve => setTimeout(resolve, 250));

            const secondEventSource = (transport as any).eventSource;

            // Should have created a new EventSource
            expect(secondEventSource).not.toBe(firstEventSource);
            expect(transport.getState()).toBe(ConnectionState.Connected);
        });
    });
});
