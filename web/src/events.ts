import { Socket } from "./socket";
import { LiveValues, LiveElement } from "./element";

/**
 * Standard event handler class. Clicks, focus and blur.
 */
class LiveHandler {
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

    protected handler(element: HTMLElement, values: LiveValues): EventListener {
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
 * KeyHandler handle key events.
 */
export class KeyHandler extends LiveHandler {
    protected handler(element: HTMLElement, values: LiveValues): EventListener {
        return (ev: Event) => {
            const ke = ev as KeyboardEvent;
            const t = element?.getAttribute(this.attribute);
            if (t === null) {
                return;
            }
            const filter = element.getAttribute("live-key");
            if (filter !== null) {
                if (ke.key !== filter) {
                    return;
                }
            }
            const keyData = {
                "key": ke.key,
                "altKey": ke.altKey,
                "ctrlKey": ke.ctrlKey,
                "shiftKey": ke.shiftKey,
                "metaKey": ke.metaKey
            };
            Socket.send({ t: t, d: { ...values, ...keyData}});
        };
    }
}

/**
 * live-click attribute handling.
 */
export class Click extends LiveHandler {
    constructor() {
        super("click", "live-click");
    }
}

/**
 * live-focus event handling.
 */
export class Focus extends LiveHandler {
    constructor() {
        super("focus", "live-focus");
    }
}

/**
 * live-blur event handling.
 */
export class Blur extends LiveHandler {
    constructor() {
        super("blur", "live-blur");
    }
}

/**
 * live-window-focus event handler.
 */
export class WindowFocus extends LiveHandler {
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
export class WindowBlur extends LiveHandler {
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
 * live-keydown event handler.
 */
export class Keydown extends KeyHandler {
    constructor() {
        super("keydown", "live-keydown");
    }
}

/**
 * live-keyup event handler.
 */
export class Keyup extends KeyHandler {
    constructor() {
        super("keyup", "live-keyup");
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
    private static keydown: Keydown;
    private static keyup: Keyup;

    /**
     * Initialise all the event wiring.
     */
    public static init() {
        this.clicks = new Click();
        this.focus = new Focus();
        this.blur = new Blur();
        this.windowFocus = new WindowFocus();
        this.windowBlur = new WindowBlur();
        this.keydown = new Keydown();
        this.keyup = new Keyup();
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
        this.keydown.attach();
        this.keyup.attach();
    }
}
