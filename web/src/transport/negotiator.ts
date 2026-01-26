import { Transport } from "./transport";
import { WebSocketTransport } from "./websocket";
import { SSETransport } from "./sse";

/**
 * TransportType identifies the type of transport.
 */
export enum TransportType {
    WebSocket = "websocket",
    SSE = "sse",
    Polling = "polling", // Reserved for future implementation
}

/**
 * NegotiatorConfig configures the transport negotiator.
 */
export interface NegotiatorConfig {
    /**
     * Timeout in milliseconds for each transport attempt.
     * Default: 2000ms (2 seconds)
     */
    timeout?: number;

    /**
     * Custom fallback order. If not specified, uses default:
     * [WebSocket, SSE, Polling]
     */
    fallbackOrder?: TransportType[];

    /**
     * Custom WebSocket endpoint path.
     * Default: "/ws"
     */
    wsEndpoint?: string;

    /**
     * Custom SSE endpoint path.
     * Default: "/live/sse"
     */
    sseEndpoint?: string;

    /**
     * Custom POST endpoint path for SSE client-to-server events.
     * Default: "/live/post"
     */
    postEndpoint?: string;
}

/**
 * NegotiationResult contains the selected transport and metadata.
 */
export interface NegotiationResult {
    /**
     * The successfully connected transport.
     */
    transport: Transport;

    /**
     * The type of transport that was selected.
     */
    type: TransportType;

    /**
     * Number of attempts made before success.
     */
    attempts: number;

    /**
     * List of transport types that were tried and failed.
     */
    failedTypes: TransportType[];
}

/**
 * TransportNegotiator automatically selects the best available transport
 * with a fallback chain: WebSocket → SSE → Polling.
 */
export class TransportNegotiator {
    private config: Required<NegotiatorConfig>;

    constructor(config?: NegotiatorConfig) {
        this.config = {
            timeout: config?.timeout ?? 2000,
            fallbackOrder: config?.fallbackOrder ?? [
                TransportType.WebSocket,
                TransportType.SSE,
            ],
            wsEndpoint: config?.wsEndpoint ?? "/ws",
            sseEndpoint: config?.sseEndpoint ?? "/live/sse",
            postEndpoint: config?.postEndpoint ?? "/live/post",
        };
    }

    /**
     * Negotiate the best available transport.
     * Returns a promise that resolves with the connected transport
     * or rejects if all transports fail.
     */
    async negotiate(): Promise<NegotiationResult> {
        const failedTypes: TransportType[] = [];
        let attempts = 0;

        for (const type of this.config.fallbackOrder) {
            attempts++;

            console.debug(`TransportNegotiator: trying ${type} (attempt ${attempts})`);

            try {
                const transport = await this.tryTransport(type, this.config.timeout);
                console.info(`TransportNegotiator: selected ${type} transport`);

                return {
                    transport,
                    type,
                    attempts,
                    failedTypes,
                };
            } catch (err) {
                console.warn(`TransportNegotiator: ${type} failed:`, err);
                failedTypes.push(type);
            }
        }

        // All transports failed
        throw new Error(
            `All transports failed. Tried: ${failedTypes.join(", ")}`
        );
    }

    /**
     * Try to connect a specific transport type with timeout.
     */
    private async tryTransport(
        type: TransportType,
        timeout: number
    ): Promise<Transport> {
        const transport = this.createTransport(type);
        let timedOut = false;
        let timeoutId: ReturnType<typeof setTimeout> | null = null;

        // Race between connection and timeout
        const connectionPromise = transport.connect().catch((err) => {
            // If we timed out, the close was intentional - suppress the error
            if (timedOut) {
                return Promise.reject(new Error(`Transport ${type} timed out after ${timeout}ms`));
            }
            throw err;
        });

        const timeoutPromise = new Promise<never>((_, reject) => {
            timeoutId = setTimeout(() => {
                timedOut = true;
                transport.close();
                reject(new Error(`Transport ${type} timed out after ${timeout}ms`));
            }, timeout);
        });

        try {
            await Promise.race([connectionPromise, timeoutPromise]);
            // Clear timeout if connection succeeded
            if (timeoutId) {
                clearTimeout(timeoutId);
            }
            return transport;
        } catch (err) {
            // Clear timeout if it hasn't fired yet
            if (timeoutId && !timedOut) {
                clearTimeout(timeoutId);
                transport.close();
            }
            throw err;
        }
    }

    /**
     * Create a transport instance of the specified type.
     */
    private createTransport(type: TransportType): Transport {
        switch (type) {
            case TransportType.WebSocket:
                return new WebSocketTransport({
                    wsEndpoint: this.config.wsEndpoint,
                });

            case TransportType.SSE:
                return new SSETransport({
                    sseEndpoint: this.config.sseEndpoint,
                    postEndpoint: this.config.postEndpoint,
                });

            case TransportType.Polling:
                // Reserved for future implementation
                throw new Error("Polling transport not yet implemented");

            default:
                throw new Error(`Unknown transport type: ${type}`);
        }
    }

    /**
     * Check if a specific transport type is available in the browser.
     * This can be used to customize fallback order based on browser capabilities.
     */
    static isTransportSupported(type: TransportType): boolean {
        switch (type) {
            case TransportType.WebSocket:
                return typeof WebSocket !== "undefined";

            case TransportType.SSE:
                return typeof EventSource !== "undefined";

            case TransportType.Polling:
                // fetch is always available in modern browsers
                return typeof fetch !== "undefined";

            default:
                return false;
        }
    }

    /**
     * Get the default fallback order, filtering out unsupported transports.
     */
    static getDefaultFallbackOrder(): TransportType[] {
        const allTypes = [
            TransportType.WebSocket,
            TransportType.SSE,
            TransportType.Polling,
        ];

        return allTypes.filter((type) => TransportNegotiator.isTransportSupported(type));
    }
}
