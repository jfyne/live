import { LiveEvent } from "./event";

export interface Transport {
    connect(
        url: string,
        onOpen: () => void,
        onClose: (code: number, reason: string) => void,
        onMessage: (ev: LiveEvent) => void,
        onError: () => void
    ): void;

    send(e: LiveEvent): void;

    close(): void;
}

export class WebSocketTransport implements Transport {
    private conn: WebSocket | null = null;

    connect(
        url: string,
        onOpen: () => void,
        onClose: (code: number, reason: string) => void,
        onMessage: (ev: LiveEvent) => void,
        onError: () => void
    ) {
        // WebSocket URL should be ws:// or wss://
        const wsUrl = url.replace(/^http/, "ws");

        this.conn = new WebSocket(wsUrl);

        this.conn.addEventListener("open", () => onOpen());
        this.conn.addEventListener("close", (ev) => onClose(ev.code, ev.reason));
        this.conn.addEventListener("error", () => onError());
        this.conn.addEventListener("message", (ev) => {
            if (typeof ev.data !== "string") {
                console.error("unexpected message type", typeof ev.data);
                return;
            }
            const e = LiveEvent.fromMessage(ev.data);
            onMessage(e);
        });
    }

    send(e: LiveEvent) {
        if (this.conn && this.conn.readyState === WebSocket.OPEN) {
            this.conn.send(e.serialize());
        } else {
            console.warn("WebSocket not ready for send");
        }
    }

    close() {
        if (this.conn) {
            this.conn.close();
            this.conn = null;
        }
    }
}

export class SSETransport implements Transport {
    private es: EventSource | null = null;
    private postUrl: string = "";

    connect(
        url: string,
        onOpen: () => void,
        onClose: (code: number, reason: string) => void,
        onMessage: (ev: LiveEvent) => void,
        onError: () => void
    ) {
        // SSE URL stays http(s)://
        this.postUrl = url;

        this.es = new EventSource(url);

        this.es.onopen = () => onOpen();
        this.es.onerror = (e) => {
            // EventSource doesn't give detailed error codes like WS.
            // If readyState is CLOSED (2), it's an error/close.
            if (this.es?.readyState === EventSource.CLOSED) {
                 onClose(1006, "EventSource connection closed");
            } else {
                 onError();
            }
        };

        this.es.onmessage = (ev) => {
             const e = LiveEvent.fromMessage(ev.data);
             onMessage(e);
        };
    }

    send(e: LiveEvent) {
        if (!this.postUrl) {
             console.warn("SSETransport not connected");
             return;
        }

        fetch(this.postUrl, {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: e.serialize()
        }).catch(err => {
            console.error("SSE Post Error", err);
        });
    }

    close() {
        if (this.es) {
            this.es.close();
            this.es = null;
        }
    }
}
