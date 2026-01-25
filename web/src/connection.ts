import { Transport, ConnectionState } from "./transport/transport";
import { TransportNegotiator, NegotiatorConfig } from "./transport/negotiator";
import { TransportMessage, IslandPatch, MessageType } from "./transport/message";

/**
 * IslandHandler is called when a patch is received for a specific island.
 */
export type IslandHandler = (patch: IslandPatch) => void;

/**
 * IslandRegistration tracks an island's subscription and handler.
 */
interface IslandRegistration {
    handler: IslandHandler;
}

/**
 * ConnectionManager manages a single shared transport connection
 * for all islands on a page. It routes incoming messages to the
 * correct island handlers and handles reconnection with re-subscription.
 *
 * This is a singleton - use ConnectionManager.getInstance() to access it.
 */
export class ConnectionManager {
    private static instance: ConnectionManager;

    private transport: Transport | null = null;
    private islands: Map<string, IslandRegistration> = new Map();
    private config: NegotiatorConfig | undefined;
    private negotiator: TransportNegotiator | null = null;

    /**
     * Private constructor enforces singleton pattern.
     */
    private constructor() {}

    /**
     * Get the singleton ConnectionManager instance.
     */
    static getInstance(): ConnectionManager {
        if (!ConnectionManager.instance) {
            ConnectionManager.instance = new ConnectionManager();
        }
        return ConnectionManager.instance;
    }

    /**
     * Register an island to receive messages.
     * If this is the first island, connection is established lazily.
     * @param islandId - Unique identifier for the island
     * @param handler - Function to call when patches arrive for this island
     */
    registerIsland(islandId: string, handler: IslandHandler): void {
        console.debug(`ConnectionManager: registering island ${islandId}`);

        // Store island registration
        this.islands.set(islandId, { handler });

        // Lazy connection: connect on first island registration
        if (this.islands.size === 1 && !this.transport) {
            this.connect(this.config).catch((err) => {
                console.error("ConnectionManager: failed to connect on island registration", err);
            });
        } else if (this.transport && this.transport.getState() === ConnectionState.Connected) {
            // If already connected, send subscription for this island
            this.subscribeIsland(islandId);
        }
    }

    /**
     * Unregister an island, stopping message delivery.
     * @param islandId - Island to unregister
     */
    unregisterIsland(islandId: string): void {
        console.debug(`ConnectionManager: unregistering island ${islandId}`);

        // Unsubscribe from transport if connected
        if (this.transport && this.transport.getState() === ConnectionState.Connected) {
            this.unsubscribeIsland(islandId);
        }

        // Remove from registrations
        this.islands.delete(islandId);

        // If no islands remain, we could optionally disconnect
        // For now, we keep the connection alive for potential future islands
    }

    /**
     * Establish the transport connection using the negotiator.
     * This is called automatically on first island registration.
     * @param config - Optional negotiator configuration
     */
    async connect(config?: NegotiatorConfig): Promise<void> {
        if (this.transport && this.transport.getState() !== ConnectionState.Closed) {
            console.debug("ConnectionManager: already connected or connecting");
            return;
        }

        this.config = config;
        this.negotiator = new TransportNegotiator(config);

        console.debug("ConnectionManager: negotiating transport");

        try {
            const result = await this.negotiator.negotiate();
            this.transport = result.transport;

            console.info(`ConnectionManager: connected using ${result.type} transport`);

            // Set up message routing
            this.transport.onMessage((message: TransportMessage) => {
                this.routeMessage(message);
            });

            // Handle reconnection
            this.transport.onStateChange((state: ConnectionState) => {
                console.debug(`ConnectionManager: state changed to ${state}`);

                if (state === ConnectionState.Connected) {
                    // Re-subscribe all islands after reconnection
                    this.resubscribeAllIslands();
                }
            });

            // Subscribe all registered islands
            this.resubscribeAllIslands();

        } catch (err) {
            console.error("ConnectionManager: transport negotiation failed", err);
            throw err;
        }
    }

    /**
     * Disconnect the transport connection.
     */
    disconnect(): void {
        console.debug("ConnectionManager: disconnecting");
        if (this.transport) {
            this.transport.close();
            this.transport = null;
        }
    }

    /**
     * Get the current connection state.
     */
    getState(): ConnectionState {
        if (!this.transport) {
            return ConnectionState.Closed;
        }
        return this.transport.getState();
    }

    /**
     * Route an incoming message to the appropriate island handler.
     * Messages with an 'island' field are routed to that island.
     * @param message - Incoming transport message
     */
    private routeMessage(message: TransportMessage): void {
        // Only route patch messages
        if (message.t !== MessageType.Patch) {
            console.debug("ConnectionManager: ignoring non-patch message", message.t);
            return;
        }

        // Extract island ID from message
        const islandId = message.island;
        if (!islandId) {
            console.warn("ConnectionManager: patch message missing island field", message);
            return;
        }

        // Find registered handler
        const registration = this.islands.get(islandId);
        if (!registration) {
            console.warn(`ConnectionManager: no handler registered for island ${islandId}`);
            return;
        }

        // Construct IslandPatch from message data
        const islandPatch: IslandPatch = {
            island_id: islandId,
            patches: message.d || [],
        };

        // Call island handler
        try {
            registration.handler(islandPatch);
        } catch (err) {
            console.error(`ConnectionManager: error in island handler for ${islandId}`, err);
        }
    }

    /**
     * Send subscription message for a specific island.
     * @param islandId - Island to subscribe
     */
    private subscribeIsland(islandId: string): void {
        if (!this.transport || this.transport.getState() !== ConnectionState.Connected) {
            console.debug(`ConnectionManager: cannot subscribe ${islandId}, not connected`);
            return;
        }

        // Send subscription message to server
        const subscribeMessage: TransportMessage = {
            t: "subscribe",
            island: islandId,
        };

        this.transport.send(subscribeMessage);
        console.debug(`ConnectionManager: subscribed island ${islandId}`);
    }

    /**
     * Send unsubscription message for a specific island.
     * @param islandId - Island to unsubscribe
     */
    private unsubscribeIsland(islandId: string): void {
        if (!this.transport || this.transport.getState() !== ConnectionState.Connected) {
            return;
        }

        // Send unsubscription message to server
        const unsubscribeMessage: TransportMessage = {
            t: "unsubscribe",
            island: islandId,
        };

        this.transport.send(unsubscribeMessage);
        console.debug(`ConnectionManager: unsubscribed island ${islandId}`);
    }

    /**
     * Re-subscribe all registered islands.
     * Called after initial connection and after reconnection.
     */
    private resubscribeAllIslands(): void {
        if (this.islands.size === 0) {
            return;
        }

        console.debug(`ConnectionManager: re-subscribing ${this.islands.size} islands`);

        for (const islandId of this.islands.keys()) {
            this.subscribeIsland(islandId);
        }
    }

    /**
     * Send an event message for a specific island.
     * This is a convenience method for islands to send events.
     * @param islandId - Source island ID
     * @param eventType - Event type
     * @param data - Event data payload
     */
    sendEvent(islandId: string, eventType: string, data?: any): void {
        if (!this.transport || this.transport.getState() !== ConnectionState.Connected) {
            console.warn("ConnectionManager: cannot send event, not connected");
            return;
        }

        const message: TransportMessage = {
            t: eventType,
            island: islandId,
            d: data,
        };

        this.transport.send(message);
    }

    /**
     * Get all registered island IDs.
     * Useful for testing and debugging.
     */
    getRegisteredIslands(): string[] {
        return Array.from(this.islands.keys());
    }
}
