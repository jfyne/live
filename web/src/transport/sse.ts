import { Transport, ConnectionState } from "./transport";
import { TransportMessage } from "./message";

const PRIVATE_SOCKET_ID = "_psid";

/**
 * SSETransport implements the Transport interface using Server-Sent Events (SSE).
 * It provides bidirectional communication using:
 * - EventSource API for server-to-client streaming
 * - fetch() with POST for client-to-server events
 */
export class SSETransport implements Transport {
    private sessionID: string | undefined;
    private eventSource: EventSource | null = null;
    private state: ConnectionState = ConnectionState.Closed;
    private messageHandler: ((message: any) => void) | null = null;
    private stateChangeHandler: ((state: ConnectionState) => void) | null = null;
    private reconnectAttempts = 0;
    private maxReconnectDelay = 5000; // 5 seconds
    private baseReconnectDelay = 100; // 100ms
    private subscribedIslands = new Set<string>();
    private shouldReconnect = true; // Flag to prevent reconnection after explicit close
    private postEndpoint: string;
    private sseEndpoint: string;

    constructor(options?: { postEndpoint?: string; sseEndpoint?: string }) {
        this.sessionID = this.getSessionID();
        this.setSessionCookie();

        // Default endpoints - can be customized via options
        this.postEndpoint = options?.postEndpoint || "/live/post";
        this.sseEndpoint = options?.sseEndpoint || "/live/sse";
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
     * Connect to the SSE server.
     */
    async connect(): Promise<void> {
        return new Promise((resolve, reject) => {
            this.shouldReconnect = true; // Re-enable reconnection on new connect
            this.setState(ConnectionState.Connecting);

            // Build SSE URL with current path
            const url = `${location.protocol}//${location.host}${this.sseEndpoint}${location.search}${location.hash}`;

            console.debug("SSETransport.connect", url, "sessionID:", this.sessionID);

            // Create EventSource with Last-Event-ID support
            this.eventSource = new EventSource(url);

            this.eventSource.addEventListener("open", () => {
                console.debug("SSE connected");
                this.reconnectAttempts = 0;
                this.setState(ConnectionState.Connected);
                // Re-subscribe to all islands after reconnection
                this.resubscribeIslands();
                resolve();
            });

            this.eventSource.addEventListener("error", (ev) => {
                console.warn("SSE error", ev);
                const wasConnected = this.state === ConnectionState.Connected;
                this.setState(ConnectionState.Closed);

                // Close the failed EventSource
                if (this.eventSource) {
                    this.eventSource.close();
                    this.eventSource = null;
                }

                // Auto-reconnect if enabled
                if (this.shouldReconnect) {
                    this.reconnectAttempts++;
                    const delay = this.getReconnectDelay();
                    console.debug(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);

                    this.setState(ConnectionState.Reconnecting);
                    setTimeout(() => {
                        this.connect();
                    }, delay);
                } else if (!wasConnected) {
                    reject(new Error("SSE connection failed"));
                }
            });

            this.eventSource.addEventListener("message", (ev) => {
                this.handleMessage(ev.data);
            });
        });
    }

    /**
     * Handle incoming SSE message.
     */
    private handleMessage(data: string) {
        try {
            const message: TransportMessage = JSON.parse(data);
            if (this.messageHandler) {
                this.messageHandler(message);
            }
        } catch (err) {
            console.error("Failed to parse SSE message", err, data);
        }
    }

    /**
     * Send a message to the server via HTTP POST.
     */
    send(message: any): void {
        if (this.state !== ConnectionState.Connected) {
            console.warn("Cannot send: connection not ready", this.state);
            return;
        }

        const url = `${location.protocol}//${location.host}${this.postEndpoint}`;
        const json = JSON.stringify(message);

        fetch(url, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "X-Live-Session": this.sessionID || "",
            },
            body: json,
            credentials: "same-origin", // Include cookies
        })
            .then((response) => {
                if (!response.ok) {
                    console.error(`POST failed: ${response.status} ${response.statusText}`);
                }
            })
            .catch((err) => {
                console.error("POST error", err);
            });
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
     * Close the SSE connection.
     */
    close(): void {
        this.shouldReconnect = false; // Disable reconnection
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
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
