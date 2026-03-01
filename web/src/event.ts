import { LiveElement } from "./element";
import { Hook, Hooks, DOM } from "./interop";

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
 *
 * NOTE: This is kept for backward compatibility with tests and internal use.
 * In v2, events are handled via TransportMessage types.
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
 * EventDispatch provides lifecycle event dispatching for elements.
 * In v2, this is primarily used for backward compatibility with existing hooks
 * and for dispatching DOM lifecycle events during patching.
 */
export class EventDispatch {
    private static hooks: Hooks = {};
    private static dom?: DOM;

    constructor() {}

    /**
     * Must be called before usage.
     */
    static init(hooks: Hooks, dom?: DOM) {
        this.hooks = hooks;
        this.dom = dom;
    }

    /**
     * Handle an element being mounted.
     */
    static mounted(element: Element) {
        const event = new CustomEvent(EventMounted, {});
        const h = this.getElementHooks(element);
        if (h !== null && h.mounted) {
            h.mounted.bind({ el: element })();
        }
        element.dispatchEvent(event);
    }

    /**
     * Before an element is updated.
     */
    static beforeUpdate(fromEl: Element, toEl: Element) {
        const event = new CustomEvent(EventBeforeUpdate, {});

        const h = this.getElementHooks(fromEl);
        if (h !== null && h.beforeUpdate) {
            h.beforeUpdate.bind({ el: fromEl })();
        }

        if (
            this.dom !== undefined &&
            this.dom.onBeforeElUpdated !== undefined
        ) {
            this.dom.onBeforeElUpdated(fromEl, toEl);
        }

        fromEl.dispatchEvent(event);
    }

    /**
     * After an element has been updated.
     */
    static updated(element: Element) {
        const event = new CustomEvent(EventUpdated, {});
        const h = this.getElementHooks(element);
        if (h !== null && h.updated) {
            h.updated.bind({ el: element })();
        }
        element.dispatchEvent(event);
    }

    /**
     * Before an element is destroyed.
     */
    static beforeDestroy(element: Element) {
        const event = new CustomEvent(EventBeforeDestroy, {});
        const h = this.getElementHooks(element);
        if (h !== null && h.beforeDestroy) {
            h.beforeDestroy.bind({ el: element })();
        }
        element.dispatchEvent(event);
    }

    /**
     * After an element has been destroyed.
     */
    static destroyed(element: Element) {
        const event = new CustomEvent(EventDestroyed, {});
        const h = this.getElementHooks(element);
        if (h !== null && h.destroyed) {
            h.destroyed.bind({ el: element })();
        }
        element.dispatchEvent(event);
    }

    /**
     * Handle a disconnection event.
     */
    static disconnected() {
        const event = new CustomEvent(EventDisconnected, {});
        document.querySelectorAll(`[live-hook]`).forEach((element: Element) => {
            const h = this.getElementHooks(element);
            if (h !== null && h.disconnected) {
                h.disconnected.bind({ el: element })();
            }
            element.dispatchEvent(event);
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
            if (h !== null && h.reconnected) {
                h.reconnected.bind({ el: element })();
            }
            element.dispatchEvent(event);
        });
        document.body.classList.remove(ClassDisconnected);
        document.body.classList.remove(ClassError);
        document.body.classList.add(ClassConnected);
    }

    /**
     * Handle an error event.
     */
    static error() {
        document.body.classList.add(ClassError);
    }

    private static getElementHooks(element: Element): Hook | null {
        const val = LiveElement.hook(element as HTMLElement);
        if (val === null) {
            return null;
        }
        return this.hooks[val] || null;
    }
}
