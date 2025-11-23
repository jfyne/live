import { EventDispatch, LiveEvent } from "./event";
import { Patch } from "./patch";
import { Events } from "./events";
import { UpdateURLParams } from "./params";
import { Transport, WebSocketTransport, SSETransport } from "./transport";

const privateSocketID = "_psid"

/**
 * Represents the connection to the backend server.
 */
export class Socket {
    private static id: string | undefined;
    private static transport: Transport;
    private static ready: boolean = false;
    private static disconnectNotified: boolean = false;
    private static transportType: "websocket" | "sse" = "sse"; // Default to SSE

    private static trackedEvents: {
        [id: number]: { ev: LiveEvent; el: HTMLElement };
    };

    constructor() {}

    static setTransportType(t: "websocket" | "sse") {
        this.transportType = t;
    }

    static getID() {
        if (this.id) {
            return this.id;
        }
        const value = `; ${document.cookie}`;
        const parts = value.split(`; ${privateSocketID}=`);
        if (parts && parts.length === 2) {
            const val = parts.pop()
            if (!val) {
                return ""
            }
            return val.split(';').shift();
        }
        return "";
    }

    static setCookie() {
        var date = new Date();
        date.setTime(date.getTime() + (60*1000));
        document.cookie = `${privateSocketID}=${this.id}; expires=${date.toUTCString()}; path=/`;
    }

    static dial() {
        this.trackedEvents = {};
        this.id = this.getID();
        this.setCookie();

        console.debug("Socket.dial called", this.id, this.transportType);

        const url = `${location.protocol}//${location.host}${location.pathname}${location.search}${location.hash}`;

        if (this.transportType === "websocket") {
            this.transport = new WebSocketTransport();
        } else {
            this.transport = new SSETransport();
        }

        this.transport.connect(
            url,
            // onOpen
            () => {
                EventDispatch.reconnected();
                this.disconnectNotified = false;
                this.ready = true;
            },
            // onClose
            (code, reason) => {
                this.ready = false;
                console.warn(
                    `Transport Disconnected code: ${code}, reason: ${reason}`
                );
                if (code !== 1001) {
                    if (this.disconnectNotified === false) {
                        EventDispatch.disconnected();
                        this.disconnectNotified = true;
                    }
                    setTimeout(() => {
                        Socket.dial();
                    }, 1000);
                }
            },
            // onMessage
            (e: LiveEvent) => {
                switch (e.typ) {
                    case "patch":
                        Patch.handle(e);
                        Events.rewire();
                        break;
                    case "params":
                        UpdateURLParams(`${window.location.pathname}?${e.data}`);
                        break;
                    case "redirect":
                        window.location.replace(e.data);
                        break;
                    case "ack":
                        this.ack(e);
                        break;
                    case "err":
                        EventDispatch.error();
                    // Fallthrough here.
                    default:
                        EventDispatch.handleEvent(e);
                }
            },
            // onError
            () => {
                // Typically handled in close or specific logging
            }
        );
    }

    /**
     * Send an event and keep track of it until
     * the ack event comes back.
     */
    static sendAndTrack(e: LiveEvent, element: HTMLElement) {
        if (this.ready === false) {
            console.warn("connection not ready for send of event", e);
            return;
        }
        this.trackedEvents[e.id] = {
            ev: e,
            el: element,
        };
        this.transport.send(e);
    }

    static send(e: LiveEvent) {
        if (this.ready === false) {
            console.warn("connection not ready for send of event", e);
            return;
        }
        this.transport.send(e);
    }

    /**
     * Called when a ack event comes in. Complete the loop
     * with any outstanding tracked events.
     */
    static ack(e: LiveEvent) {
        if (!(e.id in this.trackedEvents)) {
            return;
        }
        this.trackedEvents[e.id].el.dispatchEvent(new Event("ack"));
        delete this.trackedEvents[e.id];
    }
}
