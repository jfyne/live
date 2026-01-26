import { ConnectionManager } from "./connection";
import { Transport, ConnectionState } from "./transport/transport";
import { TransportMessage, IslandPatch, MessageType } from "./transport/message";
import { TransportNegotiator, NegotiationResult, TransportType } from "./transport/negotiator";

// Mock Transport implementation
class MockTransport implements Transport {
    private state: ConnectionState = ConnectionState.Closed;
    private messageHandler: ((message: any) => void) | null = null;
    private stateChangeHandler: ((state: ConnectionState) => void) | null = null;
    public sentMessages: any[] = [];

    async connect(): Promise<void> {
        this.setState(ConnectionState.Connecting);
        // Immediately connect for testing (synchronous)
        this.setState(ConnectionState.Connected);
        return Promise.resolve();
    }

    send(message: any): void {
        this.sentMessages.push(message);
    }

    onMessage(handler: (message: any) => void): void {
        this.messageHandler = handler;
    }

    onStateChange(handler: (state: ConnectionState) => void): void {
        this.stateChangeHandler = handler;
    }

    close(): void {
        this.setState(ConnectionState.Closed);
    }

    getState(): ConnectionState {
        return this.state;
    }

    // Test helper methods
    simulateMessage(message: any): void {
        if (this.messageHandler) {
            this.messageHandler(message);
        }
    }

    simulateStateChange(state: ConnectionState): void {
        this.setState(state);
    }

    private setState(state: ConnectionState): void {
        this.state = state;
        if (this.stateChangeHandler) {
            this.stateChangeHandler(state);
        }
    }
}

// Mock TransportNegotiator
jest.mock("./transport/negotiator", () => {
    return {
        TransportNegotiator: jest.fn(),
        TransportType: {
            WebSocket: "websocket",
            SSE: "sse",
            Polling: "polling",
        },
    };
});

describe("ConnectionManager", () => {
    let manager: ConnectionManager;
    let mockTransport: MockTransport;
    let mockNegotiate: jest.Mock;

    beforeEach(() => {
        // Reset singleton instance before each test
        (ConnectionManager as any).instance = undefined;
        manager = ConnectionManager.getInstance();

        // Create mock transport
        mockTransport = new MockTransport();

        // Create mock negotiate function that connects the transport
        mockNegotiate = jest.fn().mockImplementation(async () => {
            await mockTransport.connect();
            return {
                transport: mockTransport,
                type: TransportType.WebSocket,
                attempts: 1,
                failedTypes: [],
            } as NegotiationResult;
        });

        // Mock the negotiator constructor to return our mock
        (TransportNegotiator as jest.MockedClass<typeof TransportNegotiator>).mockImplementation(() => {
            return {
                negotiate: mockNegotiate,
            } as any;
        });
    });

    afterEach(() => {
        if (manager) {
            manager.disconnect();
        }
    });

    describe("Singleton Pattern", () => {
        test("should return the same instance on multiple calls", () => {
            const manager1 = ConnectionManager.getInstance();
            const manager2 = ConnectionManager.getInstance();

            expect(manager1).toBe(manager2);
        });

        test("should only create one instance", () => {
            const instances = [];
            for (let i = 0; i < 10; i++) {
                instances.push(ConnectionManager.getInstance());
            }

            // All should be the same instance
            const first = instances[0];
            expect(instances.every(inst => inst === first)).toBe(true);
        });
    });

    describe("Island Registration", () => {
        test("should register an island with handler", async () => {
            const handler = jest.fn();
            manager.registerIsland("island-1", "test", handler);

            // Wait for lazy connection
            await new Promise(resolve => setTimeout(resolve, 50));

            const registered = manager.getRegisteredIslands();
            expect(registered).toContain("island-1");
        });

        test("should register multiple islands", async () => {
            const handler1 = jest.fn();
            const handler2 = jest.fn();

            manager.registerIsland("island-1", "test", handler1);
            manager.registerIsland("island-2", "test", handler2);

            await new Promise(resolve => setTimeout(resolve, 50));

            const registered = manager.getRegisteredIslands();
            expect(registered).toContain("island-1");
            expect(registered).toContain("island-2");
        });

        test("should unregister an island", async () => {
            const handler = jest.fn();
            manager.registerIsland("island-1", "test", handler);

            await new Promise(resolve => setTimeout(resolve, 50));

            manager.unregisterIsland("island-1");

            const registered = manager.getRegisteredIslands();
            expect(registered).not.toContain("island-1");
        });

        test("should send subscription message on registration when connected", async () => {
            // First connect manually
            await manager.connect();

            const handler = jest.fn();
            manager.registerIsland("island-1", "test", handler);

            // Check that subscription message was sent
            const subscribeMsg = mockTransport.sentMessages.find(
                msg => msg.t === "subscribe" && msg.island === "island-1"
            );
            expect(subscribeMsg).toBeDefined();
            expect(subscribeMsg.d).toEqual({ type: "test" });
        });

        test("should send unsubscription message on unregister when connected", async () => {
            await manager.connect();

            const handler = jest.fn();
            manager.registerIsland("island-1", "test", handler);

            mockTransport.sentMessages = []; // Clear previous messages

            manager.unregisterIsland("island-1");

            const unsubscribeMsg = mockTransport.sentMessages.find(
                msg => msg.t === "unsubscribe" && msg.island === "island-1"
            );
            expect(unsubscribeMsg).toBeDefined();
        });
    });

    describe("Lazy Connection", () => {
        test("should not connect until first island is registered", () => {
            expect(manager.getState()).toBe(ConnectionState.Closed);
        });

        test("should connect automatically on first island registration", async () => {
            const handler = jest.fn();
            manager.registerIsland("island-1", "test", handler);

            // Wait for async connection
            await new Promise(resolve => setTimeout(resolve, 50));

            expect(manager.getState()).toBe(ConnectionState.Connected);
        });

        test("should not reconnect on subsequent island registrations", async () => {
            const handler1 = jest.fn();
            const handler2 = jest.fn();

            manager.registerIsland("island-1", "test", handler1);
            await new Promise(resolve => setTimeout(resolve, 50));

            mockNegotiate.mockClear();

            manager.registerIsland("island-2", "test", handler2);
            await new Promise(resolve => setTimeout(resolve, 50));

            // negotiate should not be called again
            expect(mockNegotiate).not.toHaveBeenCalled();
        });
    });

    describe("Message Routing", () => {
        test("should route patch messages to correct island handler", async () => {
            const handler1 = jest.fn();
            const handler2 = jest.fn();

            manager.registerIsland("island-1", "test", handler1);
            manager.registerIsland("island-2", "test", handler2);

            await new Promise(resolve => setTimeout(resolve, 50));

            // Simulate patch message for island-1
            const patchMessage: TransportMessage = {
                t: MessageType.Patch,
                island: "island-1",
                d: [
                    { Anchor: "_i_island-1_0", Action: 1, HTML: "<div>test</div>" }
                ],
            };

            mockTransport.simulateMessage(patchMessage);

            // Only handler1 should be called
            expect(handler1).toHaveBeenCalledTimes(1);
            expect(handler2).not.toHaveBeenCalled();

            // Check the patch structure
            const receivedPatch: IslandPatch = handler1.mock.calls[0][0];
            expect(receivedPatch.island_id).toBe("island-1");
            expect(receivedPatch.patches).toEqual(patchMessage.d);
        });

        test("should route messages to multiple islands correctly", async () => {
            const handler1 = jest.fn();
            const handler2 = jest.fn();

            manager.registerIsland("island-1", "test", handler1);
            manager.registerIsland("island-2", "test", handler2);

            await new Promise(resolve => setTimeout(resolve, 50));

            // Send message to island-1
            mockTransport.simulateMessage({
                t: MessageType.Patch,
                island: "island-1",
                d: [{ Anchor: "_i_island-1_0", Action: 1, HTML: "<div>1</div>" }],
            });

            // Send message to island-2
            mockTransport.simulateMessage({
                t: MessageType.Patch,
                island: "island-2",
                d: [{ Anchor: "_i_island-2_0", Action: 1, HTML: "<div>2</div>" }],
            });

            // Both handlers should be called once
            expect(handler1).toHaveBeenCalledTimes(1);
            expect(handler2).toHaveBeenCalledTimes(1);

            // Check correct patches were delivered
            expect(handler1.mock.calls[0][0].island_id).toBe("island-1");
            expect(handler2.mock.calls[0][0].island_id).toBe("island-2");
        });

        test("should ignore messages for unregistered islands", async () => {
            const handler = jest.fn();
            manager.registerIsland("island-1", "test", handler);

            await new Promise(resolve => setTimeout(resolve, 50));

            // Simulate message for unregistered island
            const consoleSpy = jest.spyOn(console, "warn").mockImplementation();

            mockTransport.simulateMessage({
                t: MessageType.Patch,
                island: "island-999",
                d: [],
            });

            expect(handler).not.toHaveBeenCalled();
            expect(consoleSpy).toHaveBeenCalledWith(
                expect.stringContaining("no handler registered for island island-999")
            );

            consoleSpy.mockRestore();
        });

        test("should ignore non-patch messages", async () => {
            const handler = jest.fn();
            manager.registerIsland("island-1", "test", handler);

            await new Promise(resolve => setTimeout(resolve, 50));

            // Simulate non-patch message
            mockTransport.simulateMessage({
                t: "ack",
                island: "island-1",
            });

            expect(handler).not.toHaveBeenCalled();
        });

        test("should warn on patch messages without island field", async () => {
            const handler = jest.fn();
            manager.registerIsland("island-1", "test", handler);

            await new Promise(resolve => setTimeout(resolve, 50));

            const consoleSpy = jest.spyOn(console, "warn").mockImplementation();

            mockTransport.simulateMessage({
                t: MessageType.Patch,
                d: [],
            });

            expect(handler).not.toHaveBeenCalled();
            expect(consoleSpy).toHaveBeenCalledWith(
                expect.stringContaining("patch message missing island field"),
                expect.anything()
            );

            consoleSpy.mockRestore();
        });

        test("should handle errors in island handlers gracefully", async () => {
            const errorHandler = jest.fn(() => {
                throw new Error("Handler error");
            });
            const okHandler = jest.fn();

            manager.registerIsland("island-error", "test", errorHandler);
            manager.registerIsland("island-ok", "test", okHandler);

            await new Promise(resolve => setTimeout(resolve, 50));

            const consoleErrorSpy = jest.spyOn(console, "error").mockImplementation();

            // Send message to error handler
            mockTransport.simulateMessage({
                t: MessageType.Patch,
                island: "island-error",
                d: [],
            });

            // Should log error but not throw
            expect(consoleErrorSpy).toHaveBeenCalled();
            expect(errorHandler).toHaveBeenCalled();

            // Other handlers should still work
            mockTransport.simulateMessage({
                t: MessageType.Patch,
                island: "island-ok",
                d: [],
            });

            expect(okHandler).toHaveBeenCalled();

            consoleErrorSpy.mockRestore();
        });
    });

    describe("Reconnection and Re-subscription", () => {
        test("should re-subscribe all islands after reconnection", async () => {
            const handler1 = jest.fn();
            const handler2 = jest.fn();

            manager.registerIsland("island-1", "test", handler1);
            manager.registerIsland("island-2", "test", handler2);

            await new Promise(resolve => setTimeout(resolve, 50));

            // Clear sent messages
            mockTransport.sentMessages = [];

            // Simulate reconnection
            mockTransport.simulateStateChange(ConnectionState.Connected);

            // Wait a tick for re-subscription
            await new Promise(resolve => setTimeout(resolve, 0));

            // Check that subscription messages were sent for both islands
            const subscribeMessages = mockTransport.sentMessages.filter(
                msg => msg.t === "subscribe"
            );

            expect(subscribeMessages.length).toBe(2);
            expect(subscribeMessages.some(msg => msg.island === "island-1")).toBe(true);
            expect(subscribeMessages.some(msg => msg.island === "island-2")).toBe(true);
        });

        test("should handle reconnection with no registered islands", async () => {
            await manager.connect();

            mockTransport.sentMessages = [];

            // Simulate reconnection with no islands
            mockTransport.simulateStateChange(ConnectionState.Connected);

            await new Promise(resolve => setTimeout(resolve, 0));

            // No subscription messages should be sent
            const subscribeMessages = mockTransport.sentMessages.filter(
                msg => msg.t === "subscribe"
            );
            expect(subscribeMessages.length).toBe(0);
        });
    });

    describe("Connection State Management", () => {
        test("should start in Closed state", () => {
            expect(manager.getState()).toBe(ConnectionState.Closed);
        });

        test("should transition to Connected after connect", async () => {
            await manager.connect();
            expect(manager.getState()).toBe(ConnectionState.Connected);
        });

        test("should return to Closed after disconnect", async () => {
            await manager.connect();
            expect(manager.getState()).toBe(ConnectionState.Connected);

            manager.disconnect();
            expect(manager.getState()).toBe(ConnectionState.Closed);
        });

        test("should handle disconnect when not connected", () => {
            expect(manager.getState()).toBe(ConnectionState.Closed);
            manager.disconnect(); // Should not throw
            expect(manager.getState()).toBe(ConnectionState.Closed);
        });

        test("should handle connect when already connected", async () => {
            await manager.connect();
            expect(manager.getState()).toBe(ConnectionState.Connected);

            // Try to connect again
            await manager.connect();
            expect(manager.getState()).toBe(ConnectionState.Connected);
        });
    });

    describe("Unregister During Disconnected State", () => {
        test("should allow unregister when disconnected", () => {
            const handler = jest.fn();
            manager.registerIsland("island-1", "test", handler);

            // Don't connect, just unregister immediately
            manager.unregisterIsland("island-1");

            const registered = manager.getRegisteredIslands();
            expect(registered).not.toContain("island-1");
        });

        test("should not send unsubscribe message when disconnected", () => {
            const handler = jest.fn();
            manager.registerIsland("island-1", "test", handler);

            // Unregister before connection is established
            manager.unregisterIsland("island-1");

            // No messages should be sent
            expect(mockTransport.sentMessages.length).toBe(0);
        });
    });

    describe("Event Sending", () => {
        test("should send events for islands", async () => {
            await manager.connect();

            manager.sendEvent("island-1", "click", { button: "increment" });

            const eventMsg = mockTransport.sentMessages.find(
                msg => msg.t === "click" && msg.island === "island-1"
            );

            expect(eventMsg).toBeDefined();
            expect(eventMsg.d).toEqual({ button: "increment" });
        });

        test("should not send events when disconnected", () => {
            const consoleSpy = jest.spyOn(console, "warn").mockImplementation();

            manager.sendEvent("island-1", "click", {});

            expect(mockTransport.sentMessages.length).toBe(0);
            expect(consoleSpy).toHaveBeenCalledWith(
                expect.stringContaining("cannot send event, not connected")
            );

            consoleSpy.mockRestore();
        });
    });
});
