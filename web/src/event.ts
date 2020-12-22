import { Socket } from "./socket";
import { LiveElement } from "./element";
import { Hook, Hooks } from "./interop";

/**
 * Represents an event we both receive and
 * send over the socket.
 */
export interface Event {
    t: string;
    d: any;
}

export const EventMounted = "live:mounted";
export const EventBeforeUpdate = "live:beforeupdate";
export const EventUpdated = "live:updated";
export const EventBeforeDestroy = "live:beforedestroy";
export const EventDestroyed = "live:destroyed";
export const EventDisconnected = "live:disconnected";
export const EventReconnected = "live:reconnected";

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
    static handleEvent(ev: Event) {
        if (!(ev.t in this.eventHandlers)) {
            return;
        }
        this.eventHandlers[ev.t].map((h) => {
            h(ev.d);
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
    }

    private static getElementHooks(element: Element): Hook | null {
        const val = LiveElement.hook(element as HTMLElement);
        if (val === null) {
            return val;
        }
        return this.hooks[val];
    }

    private static callHook(
        event: CustomEvent,
        element: Element,
        f: (() => void) | undefined
    ) {
        if (f === undefined) {
            return;
        }
        const pushEvent = (e: Event) => {
            Socket.send(e);
        };
        const handleEvent = (e: string, cb: (d: any) => void) => {
            if (!(e in this.eventHandlers)) {
                this.eventHandlers[e] = [];
            }
            this.eventHandlers[e].push(cb);
        };
        f.bind({ el: element, pushEvent, handleEvent })();
        element.dispatchEvent(event);
    }
}
