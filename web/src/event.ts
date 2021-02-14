import { Socket } from "./socket";
import { Hook, Hooks } from "./interop";

export const EventMounted = "live:mounted";
export const EventBeforeUpdate = "live:beforeupdate";
export const EventUpdated = "live:updated";
export const EventBeforeDestroy = "live:beforedestroy";
export const EventDestroyed = "live:destroyed";
export const EventDisconnected = "live:disconnected";
export const EventReconnected = "live:reconnected";

export const ClassConnected = "live-connected";
export const ClassDisconnected = "live-disconnected";
export const ClassError = "live-error";

/**
 * LiveEvent an event that is being passed back and forth
 * between the frontend and server.
 */
export class LiveEvent {
    public typ: string;
    public id: number;
    public data: any;
    private static sequence: number = 1;

    constructor(typ: string, data: any, id?: number) {
        this.typ = typ;
        this.data = data;
        if (id !== undefined) {
            this.id = id;
        } else {
            this.id = 0;
        }
    }

    /**
     * Get an ID for an event.
     */
    public static GetID(): number {
        return this.sequence++;
    }

    /**
     * Convert the event onto our wire format
     */
    public serialize(): string {
        return JSON.stringify({
            t: this.typ,
            i: this.id,
            d: this.data,
        });
    }

    /**
     * From an incoming message create a live event.
     */
    public static fromMessage(data: any): LiveEvent {
        const e = JSON.parse(data);
        return new LiveEvent(e.t, e.d, e.i);
    }
}

/**
 * EventDispatch allows the code base to send events
 * to hooked elements. Also handles events coming from
 * the server.
 */
export class EventDispatch {
    private static hooks: Hooks;
    private static eventHandlers: { [e: string]: ((d: any) => void)[] };

    constructor() {}

    /**
     * Must be called before usage.
     */
    static init(hooks: Hooks) {
        this.hooks = hooks;
        this.eventHandlers = {};
    }

    /**
     * Handle an event pushed from the server.
     */
    static handleEvent(ev: LiveEvent) {
        if (!(ev.typ in this.eventHandlers)) {
            return;
        }
        this.eventHandlers[ev.typ].map((h) => {
            h(ev.data);
        });
    }

    /**
     * Handle an element being mounted.
     */
    static mounted(element: Element) {
        const event = new CustomEvent(EventMounted, {});
        const h = this.getElementHooks(element);
        if (h === null) {
            return;
        }
        this.callHook(event, element, h.mounted);
    }

    /**
     * Before an element is updated.
     */
    static beforeUpdate(element: Element) {
        const event = new CustomEvent(EventBeforeUpdate, {});
        const h = this.getElementHooks(element);
        if (h === null) {
            return;
        }
        this.callHook(event, element, h.beforeUpdate);
    }

    /**
     * After and element has been updated.
     */
    static updated(element: Element) {
        const event = new CustomEvent(EventUpdated, {});
        const h = this.getElementHooks(element);
        if (h === null) {
            return;
        }
        this.callHook(event, element, h.updated);
    }

    /**
     * Before an element is destroyed.
     */
    static beforeDestroy(element: Element) {
        const event = new CustomEvent(EventBeforeDestroy, {});
        const h = this.getElementHooks(element);
        if (h === null) {
            return;
        }
        this.callHook(event, element, h.beforeDestroy);
    }

    /**
     * After an element has been destroyed.
     */
    static destroyed(element: Element) {
        const event = new CustomEvent(EventDestroyed, {});
        const h = this.getElementHooks(element);
        if (h === null) {
            return;
        }
        this.callHook(event, element, h.destroyed);
    }

    /**
     * Handle a disconnection event.
     */
    static disconnected() {
        const event = new CustomEvent(EventDisconnected, {});
        document.querySelectorAll(`[live-hook]`).forEach((element: Element) => {
            const h = this.getElementHooks(element);
            if (h === null) {
                return;
            }
            this.callHook(event, element, h.disconnected);
        });
        document.body.classList.add(ClassDisconnected);
        document.body.classList.remove(ClassConnected);
    }

    /**
     * Handle a reconnection event.
     */
    static reconnected() {
        const event = new CustomEvent(EventReconnected, {});
        document.querySelectorAll(`[live-hook]`).forEach((element: Element) => {
            const h = this.getElementHooks(element);
            if (h === null) {
                return;
            }
            this.callHook(event, element, h.reconnected);
        });
        document.body.classList.remove(ClassDisconnected);
        document.body.classList.add(ClassConnected);
    }

    /**
     * Handle an error event.
     */
    static error() {
        document.body.classList.add(ClassError);
    }

    private static getElementHooks(element: Element): Hook | null {
        if (element.getAttribute === undefined) {
            return null;
        }
        const val = element.getAttribute("live-hook");
        if (val === null) {
            return val;
        }
        return this.hooks[val];
    }

    private static callHook(
        event: CustomEvent,
        el: Element,
        f: (() => void) | undefined
    ) {
        if (f === undefined) {
            return;
        }
        const pushEvent = (e: LiveEvent) => {
            Socket.send(e);
        };
        const handleEvent = (e: string, cb: (d: any) => void) => {
            if (!(e in this.eventHandlers)) {
                this.eventHandlers[e] = [];
            }
            this.eventHandlers[e].push(cb);
        };
        f.bind({ el, pushEvent, handleEvent })();
        el.dispatchEvent(event);
    }
}
