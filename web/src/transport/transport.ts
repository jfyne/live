/**
 * Transport abstracts the communication layer between client and server.
 * Multiple transport implementations can exist (WebSocket, SSE, polling)
 * and all share the same interface.
 */
export interface Transport {
    /**
     * Connect establishes the connection to the server.
     * Returns a promise that resolves when connected.
     */
    connect(): Promise<void>;

    /**
     * Send a message to the server.
     * @param message - The message to send
     */
    send(message: any): void;

    /**
     * Register a handler for incoming messages.
     * @param handler - Callback for received messages
     */
    onMessage(handler: (message: any) => void): void;

    /**
     * Register a handler for connection state changes.
     * @param handler - Callback for state changes
     */
    onStateChange(handler: (state: ConnectionState) => void): void;

    /**
     * Close the transport connection.
     */
    close(): void;

    /**
     * Get current connection state.
     */
    getState(): ConnectionState;
}

/**
 * ConnectionState represents the current state of the transport connection.
 */
export enum ConnectionState {
    Connecting = "connecting",
    Connected = "connected",
    Reconnecting = "reconnecting",
    Closed = "closed",
}
