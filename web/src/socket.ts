import { EventDispatch, Event } from "./event";
import { Patch } from "./patch";
import { Events } from "./events";

/**
 * Represents the websocket connection to
 * the backend server.
 */
export class Socket {
    private static conn: WebSocket;
    private static ready: boolean = false;

    private static disconnectNotified: boolean = false;

    constructor() {}

    static dial() {
        console.debug("Socket.dial called");
        this.conn = new WebSocket(`ws://${location.host}${location.pathname}`);
        this.conn.addEventListener("close", (ev) => {
            this.ready = false;
            console.warn(
                `WebSocket Disconnected code: ${ev.code}, reason: ${ev.reason}`
            );
            if (ev.code !== 1001) {
                if (this.disconnectNotified === false) {
                    EventDispatch.disconnected();
                    this.disconnectNotified = true;
                }
                setTimeout(() => {
                    Socket.dial();
                }, 1000);
            }
        });
        // Ping on open.
        this.conn.addEventListener("open", (_) => {
            EventDispatch.reconnected();
            this.disconnectNotified = false;
            this.ready = true;
        });
        this.conn.addEventListener("message", (ev) => {
            if (typeof ev.data !== "string") {
                console.error("unexpected message type", typeof ev.data);
                return;
            }
            const e = JSON.parse(ev.data) as Event;
            switch (e.t) {
                case "patch":
                    Patch.handle(e);
                    Events.rewire();
                    break;
                default:
                    EventDispatch.handleEvent(e);
            }
        });
    }

    static send(e: Event) {
        if (this.ready === false) {
            console.warn("connection not ready for send of event", e);
            return;
        }
        this.conn.send(JSON.stringify(e));
    }
}
