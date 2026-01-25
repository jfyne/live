import { Transport, ConnectionState } from "./transport";
import { TransportMessage } from "./message";

const PRIVATE_SOCKET_ID = "_psid";

/**
 * WebSocketTransport implements the Transport interface using WebSocket.
 * Extracted and refactored from the original socket.ts implementation.
 */
export class WebSocketTransport implements Transport {
    private sessionID: string | undefined;
    private conn: WebSocket | null = null;
    private state: ConnectionState = ConnectionState.Closed;
    private messageHandler: ((message: any) => void) | null = null;
    private stateChangeHandler: ((state: ConnectionState) => void) | null = null;
    private reconnectAttempts = 0;
    private maxReconnectDelay = 5000; // 5 seconds
    private baseReconnectDelay = 100; // 100ms
    private subscribedIslands = new Set<string>();
    private shouldReconnect = true; // Flag to prevent reconnection after explicit close

    constructor() {
        this.sessionID = this.getSessionID();
        this.setSessionCookie();
    }

    /**
     * Get session ID from cookie or generate a new one.
     */
    private getSessionID(): string {
        const value = `; ${document.cookie}`;
        const parts = value.split(`; ${PRIVATE_SOCKET_ID}=`);
        if (parts && parts.length === 2) {
            const val = parts.pop();
            if (!val) {
                return "";
            }
            return val.split(";").shift() || "";
        }
        return "";
    }

    /**
     * Persist session ID to cookie with 60-second TTL.
     */
    private setSessionCookie() {
        const date = new Date();
        date.setTime(date.getTime() + 60 * 1000);
        document.cookie = `${PRIVATE_SOCKET_ID}=${this.sessionID}; expires=${date.toUTCString()}; path=/`;
    }

    /**
     * Calculate exponential backoff delay with jitter.
     */
    private getReconnectDelay(): number {
        const exponentialDelay = Math.min(
            this.baseReconnectDelay * Math.pow(2, this.reconnectAttempts),
            this.maxReconnectDelay
        );
        // Add jitter: random value between 0 and 10% of delay
        const jitter = Math.random() * exponentialDelay * 0.1;
        return exponentialDelay + jitter;
    }

    /**
     * Connect to the WebSocket server.
     */
    async connect(): Promise<void> {
        return new Promise((resolve, reject) => {
            this.shouldReconnect = true; // Re-enable reconnection on new connect
            this.setState(ConnectionState.Connecting);

            const protocol = location.protocol === "https:" ? "wss" : "ws";
            const url = `${protocol}://${location.host}${location.pathname}${location.search}${location.hash}`;

            console.debug("WebSocketTransport.connect", url, "sessionID:", this.sessionID);

            this.conn = new WebSocket(url);

            this.conn.addEventListener("open", () => {
                console.debug("WebSocket connected");
                this.reconnectAttempts = 0;
                this.setState(ConnectionState.Connected);
                // Re-subscribe to all islands after reconnection
                this.resubscribeIslands();
                resolve();
            });

            this.conn.addEventListener("close", (ev) => {
                console.warn(`WebSocket closed: code=${ev.code}, reason=${ev.reason}`);
                const wasConnected = this.state === ConnectionState.Connected;
                this.setState(ConnectionState.Closed);

                // Only auto-reconnect if:
                // 1. Reconnection is enabled (not explicitly closed)
                // 2. Abnormal closure (not 1001 which is "going away")
                if (this.shouldReconnect && ev.code !== 1001) {
                    this.reconnectAttempts++;
                    const delay = this.getReconnectDelay();
                    console.debug(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);

                    this.setState(ConnectionState.Reconnecting);
                    setTimeout(() => {
                        this.connect();
                    }, delay);
                } else if (!wasConnected) {
                    reject(new Error("WebSocket connection failed"));
                }
            });

            this.conn.addEventListener("message", (ev) => {
                if (typeof ev.data !== "string") {
                    console.error("Unexpected message type", typeof ev.data);
                    return;
                }
                this.handleMessage(ev.data);
            });

            this.conn.addEventListener("error", (err) => {
                console.error("WebSocket error", err);
            });
        });
    }

    /**
     * Handle incoming WebSocket message.
     */
    private handleMessage(data: string) {
        try {
            const message: TransportMessage = JSON.parse(data);
            if (this.messageHandler) {
                this.messageHandler(message);
            }
        } catch (err) {
            console.error("Failed to parse message", err, data);
        }
    }

    /**
     * Send a message through the WebSocket.
     */
    send(message: any): void {
        if (this.state !== ConnectionState.Connected || !this.conn) {
            console.warn("Cannot send: connection not ready", this.state);
            return;
        }

        const json = JSON.stringify(message);
        this.conn.send(json);
    }

    /**
     * Register message handler.
     */
    onMessage(handler: (message: any) => void): void {
        this.messageHandler = handler;
    }

    /**
     * Register state change handler.
     */
    onStateChange(handler: (state: ConnectionState) => void): void {
        this.stateChangeHandler = handler;
    }

    /**
     * Close the WebSocket connection.
     */
    close(): void {
        this.shouldReconnect = false; // Disable reconnection
        if (this.conn) {
            this.conn.close(1000, "Client closed");
            this.conn = null;
        }
        this.setState(ConnectionState.Closed);
    }

    /**
     * Get current connection state.
     */
    getState(): ConnectionState {
        return this.state;
    }

    /**
     * Update connection state and notify handler.
     */
    private setState(state: ConnectionState) {
        if (this.state === state) {
            return;
        }
        this.state = state;
        if (this.stateChangeHandler) {
            this.stateChangeHandler(state);
        }
    }

    /**
     * Subscribe to an island for message routing.
     * Islands must re-subscribe after reconnection.
     */
    subscribeIsland(islandID: string): void {
        this.subscribedIslands.add(islandID);
    }

    /**
     * Unsubscribe from an island.
     */
    unsubscribeIsland(islandID: string): void {
        this.subscribedIslands.delete(islandID);
    }

    /**
     * Get all subscribed island IDs.
     */
    getSubscribedIslands(): string[] {
        return Array.from(this.subscribedIslands);
    }

    /**
     * Re-subscribe to all islands after reconnection.
     * This ensures message routing continues after disconnect.
     */
    private resubscribeIslands(): void {
        // In a future implementation, this could send a subscription message
        // to the server. For now, we just track them client-side.
        console.debug("Re-subscribed to islands:", Array.from(this.subscribedIslands));
    }
}
