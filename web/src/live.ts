import { Socket } from "./socket";
import { Events } from "./events";
import { EventDispatch, LiveEvent } from "./event";
import { Hooks, DOM } from "./interop";

export class Live {
    constructor(private hooks: Hooks, private dom?: DOM) {}

    public init() {
        // Check that this document has been rendered by live.
        if (document.querySelector(`[live-rendered]`) === null) {
            return;
        }
        // Initialise the event dispatch.
        EventDispatch.init(this.hooks, this.dom);

        // Dial the server.
        Socket.dial();

        // Initialise our live bindings.
        Events.init();

        // Rewire all the events.
        Events.rewire();
    }

    public send(typ: string, data: any, id?: number) {
        const e = new LiveEvent(typ, data, id);
        Socket.send(e);
    }
}
