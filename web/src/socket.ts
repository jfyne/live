import { EventDispatch, LiveEvent } from "./event";
import { Patch } from "./patch";
import { Events } from "./events";
import { UpdateURLParams } from "./params";

const privateSocketID = "_psid"

/**
 * Represents the websocket connection to
 * the backend server.
 */
export class Socket {
    private static id: string | undefined;
    private static conn: WebSocket;
    private static ready: boolean = false;
    private static disconnectNotified: boolean = false;

    private static trackedEvents: {
        [id: number]: { ev: LiveEvent; el: HTMLElement };
    };

    constructor() {}

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

        console.debug("Socket.dial called", this.id);
        this.conn = new WebSocket(
            `${location.protocol === "https:" ? "wss" : "ws"}://${
                location.host
            }${location.pathname}${location.search}${location.hash}`
        );
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
            const e = LiveEvent.fromMessage(ev.data);
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
        });
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
        this.conn.send(e.serialize());
    }

    static send(e: LiveEvent) {
        if (this.ready === false) {
            console.warn("connection not ready for send of event", e);
            return;
        }
        this.conn.send(e.serialize());
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
