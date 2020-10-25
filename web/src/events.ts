import { Socket } from "./socket";
import { LiveValues, LiveElement } from "./element";

class EventHandler {
    constructor(protected event: string, protected attribute: string) {}

    public isWired(element: Element): boolean {
        if (element.hasAttribute(`${this.attribute}-wired`)) {
            return true;
        }
        element.setAttribute(`${this.attribute}-wired`, "");
        return false;
    }

    public attach() {
        document
            .querySelectorAll(`*[${this.attribute}]`)
            .forEach((element: Element) => {
                if (this.isWired(element) == true) {
                    return;
                }
                const values = LiveElement.values(element as HTMLElement);
                element.addEventListener(
                    this.event,
                    this.handler(element as HTMLElement, values)
                );
            });
    }

    protected handler(element: HTMLElement, values: LiveValues) {
        return (_: Event) => {
            const t = element?.getAttribute(this.attribute);
            if (t === null) {
                return;
            }
            Socket.send({ t: t, d: values });
        };
    }
}

/**
 * live-click attribute handling.
 */
export class Click extends EventHandler {
    constructor() {
        super("click", "live-click");
    }
}

/**
 * live-focus event handling.
 */
export class Focus extends EventHandler {
    constructor() {
        super("focus", "live-focus");
    }
}

/**
 * live-blur event handling.
 */
export class Blur extends EventHandler {
    constructor() {
        super("blur", "live-blur");
    }
}

/**
 * live-window-focus event handler.
 */
export class WindowFocus extends EventHandler {
    constructor() {
        super("focus", "live-window-focus");
    }

    public attach() {
        document
            .querySelectorAll(`*[${this.attribute}]`)
            .forEach((element: Element) => {
                if (this.isWired(element) === true) {
                    return;
                }
                const values = LiveElement.values(element as HTMLElement);
                window.addEventListener(
                    this.event,
                    this.handler(element as HTMLElement, values)
                );
            });
    }
}

/**
 * live-window-blur event handler.
 */
export class WindowBlur extends EventHandler {
    constructor() {
        super("blur", "live-window-blur");
    }

    public attach() {
        document
            .querySelectorAll(`*[${this.attribute}]`)
            .forEach((element: Element) => {
                if (this.isWired(element) === true) {
                    return;
                }
                const values = LiveElement.values(element as HTMLElement);
                window.addEventListener(
                    this.event,
                    this.handler(element as HTMLElement, values)
                );
            });
    }
}

/**
 * Handle all events.
 */
export class Events {
    private static clicks: Click;
    private static focus: Focus;
    private static blur: Blur;
    private static windowFocus: WindowFocus;
    private static windowBlur: WindowBlur;

    /**
     * Initialise all the event wiring.
     */
    public static init() {
        this.clicks = new Click();
        this.focus = new Focus();
        this.blur = new Blur();
        this.windowFocus = new WindowFocus();
        this.windowBlur = new WindowBlur();
    }

    /**
     * Re-attach all events when we have re-rendered.
     */
    public static rewire() {
        this.clicks.attach();
        this.focus.attach();
        this.blur.attach();
        this.windowFocus.attach();
        this.windowBlur.attach();
    }
}
