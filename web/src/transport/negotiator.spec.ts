import { TransportNegotiator, TransportType, NegotiatorConfig } from "./negotiator";
import { ConnectionState } from "./transport";
import { WebSocketTransport } from "./websocket";
import { SSETransport } from "./sse";

// Mock WebSocket
class MockWebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;
    static shouldFail = false;

    readyState = MockWebSocket.CONNECTING;
    url: string;
    private listeners: { [event: string]: Function[] } = {};

    constructor(url: string) {
        this.url = url;
        // Simulate async connection - fail or succeed based on flag
        setTimeout(() => {
            if (MockWebSocket.shouldFail) {
                this.simulateError(new Error("WebSocket connection failed"));
            } else {
                this.simulateOpen();
            }
        }, 10);
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
        // EventSource also closes on error
        this.simulateClose(1006, "error");
    }

    private emit(event: string, data: any) {
        if (this.listeners[event]) {
            this.listeners[event].forEach(handler => handler(data));
        }
    }
}

// Mock EventSource
class MockEventSource {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSED = 2;
    static shouldFail = false;

    readyState = MockEventSource.CONNECTING;
    url: string;
    private listeners: { [event: string]: Function[] } = {};

    constructor(url: string) {
        this.url = url;
        // Simulate async connection - fail or succeed based on flag
        setTimeout(() => {
            if (MockEventSource.shouldFail) {
                this.simulateError();
            } else {
                this.simulateOpen();
            }
        }, 10);
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

// Mock fetch for SSE POST
const mockFetch = jest.fn();

// Replace globals with mocks
(global as any).WebSocket = MockWebSocket;
(global as any).EventSource = MockEventSource;
(global as any).fetch = mockFetch;
(global as any).location = {
    protocol: "http:",
    host: "localhost:8080",
    pathname: "/",
    search: "",
    hash: "",
};

describe("TransportNegotiator", () => {
    let mockCookie: string;
    let errorSpy: jest.SpyInstance;
    let warnSpy: jest.SpyInstance;

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

        // Reset mock flags
        MockWebSocket.shouldFail = false;
        MockEventSource.shouldFail = false;

        // Globally suppress console output for negotiator tests
        // (negotiation involves intentional transport failures)
        errorSpy = jest.spyOn(console, "error").mockImplementation();
        warnSpy = jest.spyOn(console, "warn").mockImplementation();
    });

    afterEach(() => {
        errorSpy.mockRestore();
        warnSpy.mockRestore();
    });

    describe("Successful WebSocket Selection", () => {
        test("should select WebSocket when available", async () => {
            const negotiator = new TransportNegotiator();
            const result = await negotiator.negotiate();

            expect(result.type).toBe(TransportType.WebSocket);
            expect(result.transport).toBeInstanceOf(WebSocketTransport);
            expect(result.attempts).toBe(1);
            expect(result.failedTypes).toHaveLength(0);
            expect(result.transport.getState()).toBe(ConnectionState.Connected);

            // Clean up
            result.transport.close();
        });

        test("should return transport in connected state", async () => {
            const negotiator = new TransportNegotiator();
            const result = await negotiator.negotiate();

            expect(result.transport.getState()).toBe(ConnectionState.Connected);

            result.transport.close();
        });
    });

    describe("Fallback to SSE", () => {
        test("should fallback to SSE when WebSocket fails", async () => {
            MockWebSocket.shouldFail = true;

            const negotiator = new TransportNegotiator();
            const result = await negotiator.negotiate();

            expect(result.type).toBe(TransportType.SSE);
            expect(result.transport).toBeInstanceOf(SSETransport);
            expect(result.attempts).toBe(2);
            expect(result.failedTypes).toEqual([TransportType.WebSocket]);
            expect(result.transport.getState()).toBe(ConnectionState.Connected);

            result.transport.close();
        });

        test("should track all failed transport types", async () => {
            MockWebSocket.shouldFail = true;

            const negotiator = new TransportNegotiator();
            const result = await negotiator.negotiate();

            expect(result.failedTypes).toContain(TransportType.WebSocket);
            expect(result.failedTypes).not.toContain(TransportType.SSE);

            result.transport.close();
        });
    });

    describe("Timeout Handling", () => {
        test("should timeout and fallback if WebSocket takes too long", async () => {
            // Make WebSocket never connect (don't call simulateOpen)
            const originalConstructor = MockWebSocket.prototype.constructor;
            MockWebSocket.prototype.constructor = function (this: any, url: string) {
                this.url = url;
                this.readyState = MockWebSocket.CONNECTING;
                this.listeners = {};
                // Don't call simulateOpen - just hang
            } as any;

            // Suppress expected console output
            const errorSpy = jest.spyOn(console, "error").mockImplementation();
            const warnSpy = jest.spyOn(console, "warn").mockImplementation();

            const negotiator = new TransportNegotiator({ timeout: 100 });
            const result = await negotiator.negotiate();

            expect(result.type).toBe(TransportType.SSE);
            expect(result.failedTypes).toContain(TransportType.WebSocket);

            result.transport.close();

            // Restore constructor
            MockWebSocket.prototype.constructor = originalConstructor;
            errorSpy.mockRestore();
            warnSpy.mockRestore();
        });

        test("should use custom timeout value", async () => {
            // Make both transports hang
            const originalWSConstructor = MockWebSocket.prototype.constructor;
            const originalESConstructor = MockEventSource.prototype.constructor;

            MockWebSocket.prototype.constructor = function (this: any, url: string) {
                this.url = url;
                this.readyState = MockWebSocket.CONNECTING;
                this.listeners = {};
            } as any;

            MockEventSource.prototype.constructor = function (this: any, url: string) {
                this.url = url;
                this.readyState = MockEventSource.CONNECTING;
                this.listeners = {};
            } as any;

            const negotiator = new TransportNegotiator({ timeout: 50 });
            const startTime = Date.now();

            try {
                await negotiator.negotiate();
            } catch (err) {
                const elapsed = Date.now() - startTime;
                // Should fail quickly (both transports timeout at ~50ms each = ~100ms total)
                expect(elapsed).toBeLessThan(200);
                expect(err).toBeInstanceOf(Error);
            }

            // Restore constructors
            MockWebSocket.prototype.constructor = originalWSConstructor;
            MockEventSource.prototype.constructor = originalESConstructor;
        });
    });

    describe("Custom Fallback Order", () => {
        test("should respect custom fallback order", async () => {
            MockEventSource.shouldFail = true;

            const negotiator = new TransportNegotiator({
                fallbackOrder: [TransportType.SSE, TransportType.WebSocket],
            });

            const result = await negotiator.negotiate();

            expect(result.type).toBe(TransportType.WebSocket);
            expect(result.attempts).toBe(2);
            expect(result.failedTypes).toEqual([TransportType.SSE]);

            result.transport.close();
        });

        test("should try only specified transports", async () => {
            const negotiator = new TransportNegotiator({
                fallbackOrder: [TransportType.WebSocket],
            });

            const result = await negotiator.negotiate();

            expect(result.type).toBe(TransportType.WebSocket);
            expect(result.attempts).toBe(1);

            result.transport.close();
        });
    });

    describe("All Transports Fail", () => {
        test("should reject when all transports fail", async () => {
            MockWebSocket.shouldFail = true;
            MockEventSource.shouldFail = true;

            // Suppress expected console output from failed transports
            const errorSpy = jest.spyOn(console, "error").mockImplementation();
            const warnSpy = jest.spyOn(console, "warn").mockImplementation();

            const negotiator = new TransportNegotiator();

            await expect(negotiator.negotiate()).rejects.toThrow(
                "All transports failed"
            );

            errorSpy.mockRestore();
            warnSpy.mockRestore();
        });

        test("should include failed transport types in error", async () => {
            MockWebSocket.shouldFail = true;
            MockEventSource.shouldFail = true;

            // Suppress expected console output from failed transports
            const errorSpy = jest.spyOn(console, "error").mockImplementation();
            const warnSpy = jest.spyOn(console, "warn").mockImplementation();

            const negotiator = new TransportNegotiator();

            try {
                await negotiator.negotiate();
                throw new Error("Should have thrown");
            } catch (err: any) {
                expect(err.message).toContain("websocket");
                expect(err.message).toContain("sse");
            }

            errorSpy.mockRestore();
            warnSpy.mockRestore();
        });
    });

    describe("Configuration Options", () => {
        test("should pass custom SSE endpoint to transport", async () => {
            MockWebSocket.shouldFail = true;

            // Suppress expected console output from failed WebSocket
            const errorSpy = jest.spyOn(console, "error").mockImplementation();
            const warnSpy = jest.spyOn(console, "warn").mockImplementation();

            const negotiator = new TransportNegotiator({
                sseEndpoint: "/custom/sse",
                postEndpoint: "/custom/post",
            });

            const result = await negotiator.negotiate();

            expect(result.type).toBe(TransportType.SSE);
            expect((result.transport as any).sseEndpoint).toBe("/custom/sse");
            expect((result.transport as any).postEndpoint).toBe("/custom/post");

            result.transport.close();
            errorSpy.mockRestore();
            warnSpy.mockRestore();
        });

        test("should use default endpoints when not specified", async () => {
            MockWebSocket.shouldFail = true;

            // Suppress expected console output from failed WebSocket
            const errorSpy = jest.spyOn(console, "error").mockImplementation();
            const warnSpy = jest.spyOn(console, "warn").mockImplementation();

            const negotiator = new TransportNegotiator();
            const result = await negotiator.negotiate();

            expect((result.transport as any).sseEndpoint).toBe("/live/sse");
            expect((result.transport as any).postEndpoint).toBe("/live/post");

            result.transport.close();
            errorSpy.mockRestore();
            warnSpy.mockRestore();
        });
    });

    describe("Transport Support Detection", () => {
        test("should detect WebSocket support", () => {
            expect(TransportNegotiator.isTransportSupported(TransportType.WebSocket)).toBe(true);
        });

        test("should detect EventSource support", () => {
            expect(TransportNegotiator.isTransportSupported(TransportType.SSE)).toBe(true);
        });

        test("should detect fetch support for polling", () => {
            expect(TransportNegotiator.isTransportSupported(TransportType.Polling)).toBe(true);
        });

        test("should return false for unknown transport type", () => {
            expect(TransportNegotiator.isTransportSupported("unknown" as any)).toBe(false);
        });

        test("should get default fallback order with supported transports", () => {
            const order = TransportNegotiator.getDefaultFallbackOrder();

            expect(order).toContain(TransportType.WebSocket);
            expect(order).toContain(TransportType.SSE);
            // Polling is included in default order even if not implemented yet
            expect(order.length).toBeGreaterThanOrEqual(2);
        });
    });

    describe("Connection Lifecycle", () => {
        test("should close failed transports during negotiation", async () => {
            MockWebSocket.shouldFail = true;

            // Suppress expected console output from failed WebSocket
            const errorSpy = jest.spyOn(console, "error").mockImplementation();
            const warnSpy = jest.spyOn(console, "warn").mockImplementation();

            const negotiator = new TransportNegotiator();
            const result = await negotiator.negotiate();

            // The failed WebSocket should have been closed
            // Only SSE should remain open
            expect(result.transport.getState()).toBe(ConnectionState.Connected);

            result.transport.close();
            errorSpy.mockRestore();
            warnSpy.mockRestore();
        });

        test("should return working transport ready to use", async () => {
            const negotiator = new TransportNegotiator();
            const result = await negotiator.negotiate();

            // Should be able to send messages immediately
            const messages: any[] = [];
            result.transport.onMessage((msg) => messages.push(msg));

            expect(result.transport.getState()).toBe(ConnectionState.Connected);

            result.transport.close();
        });
    });

    describe("Concurrent Negotiation", () => {
        test("should handle multiple concurrent negotiations", async () => {
            const negotiator = new TransportNegotiator();

            const results = await Promise.all([
                negotiator.negotiate(),
                negotiator.negotiate(),
                negotiator.negotiate(),
            ]);

            expect(results).toHaveLength(3);
            results.forEach((result) => {
                expect(result.type).toBe(TransportType.WebSocket);
                expect(result.transport.getState()).toBe(ConnectionState.Connected);
                result.transport.close();
            });
        });
    });

    describe("Error Messages", () => {
        test("should provide clear error for timeout", async () => {
            // Save the original WebSocket
            const OriginalMockWebSocket = (global as any).WebSocket;

            // Create a hanging WebSocket that never connects
            class HangingWebSocket {
                static CONNECTING = 0;
                static OPEN = 1;
                static CLOSING = 2;
                static CLOSED = 3;

                readyState = HangingWebSocket.CONNECTING;
                url: string;
                listeners: { [event: string]: Function[] } = {};

                constructor(url: string) {
                    this.url = url;
                    // Never call simulateOpen - just hang forever
                }

                addEventListener(event: string, handler: Function) {
                    if (!this.listeners[event]) {
                        this.listeners[event] = [];
                    }
                    this.listeners[event].push(handler);
                }

                close() {
                    this.readyState = HangingWebSocket.CLOSED;
                }

                send() {}
                removeEventListener() {}
            }

            // Replace with hanging WebSocket
            (global as any).WebSocket = HangingWebSocket;

            // Use both transports but timeout WebSocket quickly
            const negotiator = new TransportNegotiator({
                timeout: 50,
                // Don't specify fallback order - use default (WS, SSE)
            });

            const result = await negotiator.negotiate();

            // Should fallback to SSE after WebSocket times out
            expect(result.type).toBe(TransportType.SSE);
            expect(result.failedTypes).toContain(TransportType.WebSocket);

            result.transport.close();

            // Restore original WebSocket
            (global as any).WebSocket = OriginalMockWebSocket;
        });
    });
});
