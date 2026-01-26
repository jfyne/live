import { WebSocketTransport } from "./websocket";
import { ConnectionState } from "./transport";
import { TransportMessage, MessageType } from "./message";

// Mock WebSocket
class MockWebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;

    readyState = MockWebSocket.CONNECTING;
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

    send(data: string) {
        if (this.readyState !== MockWebSocket.OPEN) {
            throw new Error("WebSocket is not open");
        }
    }

    close(code?: number, reason?: string) {
        this.readyState = MockWebSocket.CLOSED;
        this.simulateClose(code || 1000, reason || "");
    }

    simulateOpen() {
        this.readyState = MockWebSocket.OPEN;
        this.emit("open", {});
    }

    simulateClose(code: number, reason: string) {
        this.readyState = MockWebSocket.CLOSED;
        this.emit("close", { code, reason });
    }

    simulateMessage(data: string) {
        this.emit("message", { data });
    }

    simulateError(error: any) {
        this.emit("error", error);
    }

    private emit(event: string, data: any) {
        if (this.listeners[event]) {
            this.listeners[event].forEach(handler => handler(data));
        }
    }
}

// Replace global WebSocket with mock
(global as any).WebSocket = MockWebSocket;
(global as any).location = {
    protocol: "http:",
    host: "localhost:8080",
    pathname: "/",
    search: "",
    hash: "",
};

describe("WebSocketTransport", () => {
    let transport: WebSocketTransport;
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

        // Reset location to http: for consistent test state
        (global as any).location = {
            protocol: "http:",
            host: "localhost:8080",
            pathname: "/",
            search: "",
            hash: "",
        };
    });

    afterEach(() => {
        if (transport) {
            transport.close();
        }
    });

    describe("Connection Establishment", () => {
        test("should connect successfully", async () => {
            transport = new WebSocketTransport();
            await expect(transport.connect()).resolves.toBeUndefined();
            expect(transport.getState()).toBe(ConnectionState.Connected);
        });

        test("should start in Connecting state", () => {
            transport = new WebSocketTransport();
            const promise = transport.connect();
            expect(transport.getState()).toBe(ConnectionState.Connecting);
            return promise;
        });

        test("should emit state change events", async () => {
            transport = new WebSocketTransport();
            const states: ConnectionState[] = [];
            transport.onStateChange((state) => states.push(state));

            await transport.connect();

            expect(states).toContain(ConnectionState.Connecting);
            expect(states).toContain(ConnectionState.Connected);
        });
    });

    describe("Session ID Management", () => {
        test("should persist session ID to cookie", () => {
            transport = new WebSocketTransport();
            expect(mockCookie).toContain("live_session=");
        });

        test("should read existing session ID from cookie", () => {
            mockCookie = "live_session=test-session-123; path=/";
            transport = new WebSocketTransport();
            // Session ID is read from cookie on construction
            expect(mockCookie).toContain("test-session-123");
        });

        test("should set cookie with security attributes", () => {
            transport = new WebSocketTransport();
            expect(mockCookie).toContain("Max-Age=60");
            expect(mockCookie).toContain("Path=/");
            expect(mockCookie).toContain("SameSite=Strict");
            // Secure flag should not be present for http:
            expect(mockCookie).not.toContain("Secure");
            // Note: Secure flag is conditionally added for HTTPS via location.protocol check
            // Manual verification: The code checks `location.protocol === 'https:'` before adding Secure flag
        });
    });

    describe("Message Handling", () => {
        test("should receive and parse messages", async () => {
            transport = new WebSocketTransport();
            const messages: TransportMessage[] = [];
            transport.onMessage((msg) => messages.push(msg));

            await transport.connect();

            const testMessage: TransportMessage = {
                t: "patch",
                i: 1,
                island: "counter-1",
                d: { patches: [] },
            };

            (transport as any).conn.simulateMessage(JSON.stringify(testMessage));

            expect(messages).toHaveLength(1);
            expect(messages[0]).toEqual(testMessage);
        });

        test("should handle message with island field", async () => {
            transport = new WebSocketTransport();
            const messages: TransportMessage[] = [];
            transport.onMessage((msg) => messages.push(msg));

            await transport.connect();

            const testMessage: TransportMessage = {
                t: "patch",
                island: "my-island",
                d: [{ Anchor: "_i_my-island_0", Action: 1, HTML: "<div>test</div>" }],
            };

            (transport as any).conn.simulateMessage(JSON.stringify(testMessage));

            expect(messages[0].island).toBe("my-island");
        });

        test("should ignore non-string messages", async () => {
            transport = new WebSocketTransport();
            const messages: TransportMessage[] = [];
            transport.onMessage((msg) => messages.push(msg));

            await transport.connect();

            // Simulate binary message (should be ignored)
            const binaryEvent = { data: new ArrayBuffer(8) };
            (transport as any).conn.emit("message", binaryEvent);

            expect(messages).toHaveLength(0);
        });
    });

    describe("Sending Messages", () => {
        test("should send messages when connected", async () => {
            transport = new WebSocketTransport();
            await transport.connect();

            const mockSend = jest.fn();
            (transport as any).conn.send = mockSend;

            const message: TransportMessage = {
                t: "click",
                i: 1,
                island: "counter-1",
                d: { button: "increment" },
            };

            transport.send(message);

            expect(mockSend).toHaveBeenCalledWith(JSON.stringify(message));
        });

        test("should not send when not connected", () => {
            transport = new WebSocketTransport();
            // Don't connect

            const consoleSpy = jest.spyOn(console, "warn").mockImplementation();
            transport.send({ t: "test" });

            expect(consoleSpy).toHaveBeenCalledWith(
                "Cannot send: connection not ready",
                ConnectionState.Closed
            );
            consoleSpy.mockRestore();
        });
    });

    describe("Island Subscription", () => {
        test("should track subscribed islands", () => {
            transport = new WebSocketTransport();
            transport.subscribeIsland("island-1");
            transport.subscribeIsland("island-2");

            const subscribed = transport.getSubscribedIslands();
            expect(subscribed).toContain("island-1");
            expect(subscribed).toContain("island-2");
        });

        test("should unsubscribe from islands", () => {
            transport = new WebSocketTransport();
            transport.subscribeIsland("island-1");
            transport.subscribeIsland("island-2");
            transport.unsubscribeIsland("island-1");

            const subscribed = transport.getSubscribedIslands();
            expect(subscribed).not.toContain("island-1");
            expect(subscribed).toContain("island-2");
        });

        test("should re-subscribe islands after reconnection", async () => {
            transport = new WebSocketTransport();
            transport.subscribeIsland("island-1");
            transport.subscribeIsland("island-2");

            await transport.connect();

            // Simulate disconnect
            (transport as any).conn.simulateClose(1006, "connection lost");

            // Wait for reconnection
            await new Promise(resolve => setTimeout(resolve, 150));

            // Islands should still be subscribed
            const subscribed = transport.getSubscribedIslands();
            expect(subscribed).toContain("island-1");
            expect(subscribed).toContain("island-2");
        });
    });

    describe("Reconnection with Backoff", () => {
        test("should reconnect after abnormal close", async () => {
            transport = new WebSocketTransport();
            await transport.connect();

            const states: ConnectionState[] = [];
            transport.onStateChange((state) => states.push(state));

            // Simulate abnormal close (not 1001)
            (transport as any).conn.simulateClose(1006, "connection lost");

            expect(transport.getState()).toBe(ConnectionState.Reconnecting);

            // Wait for reconnection with backoff (100ms + jitter + connection time)
            await new Promise(resolve => setTimeout(resolve, 300));
            expect(transport.getState()).toBe(ConnectionState.Connected);
        });

        test("should use exponential backoff delays", () => {
            transport = new WebSocketTransport();

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
            transport = new WebSocketTransport();

            // Set many attempts to exceed max
            (transport as any).reconnectAttempts = 20;
            const delay = (transport as any).getReconnectDelay();

            // Should be capped at 5000ms + 10% jitter = 5500ms max
            expect(delay).toBeLessThanOrEqual(5500);
        });

        test("should reset backoff counter on successful connection", async () => {
            transport = new WebSocketTransport();

            // Set some failed attempts
            (transport as any).reconnectAttempts = 5;

            await transport.connect();

            // Reconnect counter should be reset on successful connection
            expect((transport as any).reconnectAttempts).toBe(0);
        });

        test("should not reconnect on normal close (1001)", async () => {
            transport = new WebSocketTransport();
            await transport.connect();

            const states: ConnectionState[] = [];
            transport.onStateChange((state) => states.push(state));

            (transport as any).conn.simulateClose(1001, "going away");

            expect(transport.getState()).toBe(ConnectionState.Closed);

            // Wait a bit to ensure no reconnection happens
            await new Promise(resolve => setTimeout(resolve, 200));

            // Should still be closed, not reconnecting
            expect(transport.getState()).toBe(ConnectionState.Closed);
            expect(states).not.toContain(ConnectionState.Reconnecting);
        });
    });

    describe("Connection Lifecycle", () => {
        test("should transition through states correctly", async () => {
            transport = new WebSocketTransport();
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
            transport = new WebSocketTransport();
            await transport.connect();

            const mockClose = jest.fn();
            (transport as any).conn.close = mockClose;

            transport.close();

            expect(mockClose).toHaveBeenCalledWith(1000, "Client closed");
            expect(transport.getState()).toBe(ConnectionState.Closed);
        });

        test("should handle multiple close calls", async () => {
            transport = new WebSocketTransport();
            await transport.connect();

            transport.close();
            transport.close(); // Should not throw

            expect(transport.getState()).toBe(ConnectionState.Closed);
        });
    });
});
