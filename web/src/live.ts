import { Socket } from "./socket";
import { Events } from "./events";
import { EventDispatch } from "./event";
import { Hooks } from "./interop";

export class Live {
    constructor(private hooks: Hooks) {}

    public init() {
        // Check that this document has been rendered by live.
        if (document.querySelector(`[live-rendered]`) === null) {
            return;
        }
        // Initialise the event dispatch.
        EventDispatch.init(this.hooks);

        // Dial the server.
        Socket.dial();

        // Initialise our live bindings.
        Events.init();

        // Rewire all the events.
        Events.rewire();
    }
}
