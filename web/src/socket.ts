import { Event } from "./event";
import { Patch } from "./patch";
import { Events } from "./events";

/**
 * Represents the websocket connection to
 * the backend server.
 */
export class Socket {
    private static conn: WebSocket;

    constructor() {}

    static dial() {
        this.conn = new WebSocket(
            `ws://${location.host}/socket${location.pathname}`
        );
        this.conn.addEventListener("close", (ev) => {
            console.warn(
                `WebSocket Disconnected code: ${ev.code}, reason: ${ev.reason}`
            );
            if (ev.code !== 1001) {
                console.warn("Reconnecting in 1s");
                setTimeout(() => {
                    Socket.dial();
                }, 1000);
            }
        });
        // Ping on open.
        this.conn.addEventListener("open", (ev) => {
            console.info("websocket connected", ev);
            this.send({ t: "ping", d: location.pathname });
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
                    console.log(e);
            }
        });
    }

    static send(e: Event) {
        this.conn.send(JSON.stringify(e));
    }
}
