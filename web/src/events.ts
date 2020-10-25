import { Socket } from "./socket";
import { LiveElement } from "./element";

/**
 * handles a "live-click" click event.
 */
function clickHandler(this: HTMLElement, _: Event) {
    //const element = e.target as HTMLElement;
    const t = this?.getAttribute("live-click");
    if (t === null) {
        return;
    }
    const values = LiveElement.values(this);
    Socket.send({ t: t, d: values });
}

/**
 * live-click attribute handling.
 */
export class Click {
    /**
     * Attaches handlers to all live-click attributes in
     * the DOM
     */
    public static attach() {
        document.querySelectorAll("*[live-click]").forEach((element) => {
            element.addEventListener("click", clickHandler);
        });
    }
}

/**
 * Handle all events.
 */
export class Events {
    /**
     * Re-attach all events when we have re-rendered.
     */
    public static rewire() {
        Click.attach();
    }
}
