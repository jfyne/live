import { Socket } from "./socket";
import { LiveValues, LiveElement } from "./element";
import { EventDispatch } from "./event";

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

    protected windowAttach() {
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
                key: ke.key,
                altKey: ke.altKey,
                ctrlKey: ke.ctrlKey,
                shiftKey: ke.shiftKey,
                metaKey: ke.metaKey,
            };
            Socket.send({ t: t, d: { ...values, ...keyData } });
        };
    }
}

/**
 * live-click attribute handling.
 */
class Click extends LiveHandler {
    constructor() {
        super("click", "live-click");
    }
}

/**
 * live-focus event handling.
 */
class Focus extends LiveHandler {
    constructor() {
        super("focus", "live-focus");
    }
}

/**
 * live-blur event handling.
 */
class Blur extends LiveHandler {
    constructor() {
        super("blur", "live-blur");
    }
}

/**
 * live-window-focus event handler.
 */
class WindowFocus extends LiveHandler {
    constructor() {
        super("focus", "live-window-focus");
    }

    public attach() {
        this.windowAttach();
    }
}

/**
 * live-window-blur event handler.
 */
class WindowBlur extends LiveHandler {
    constructor() {
        super("blur", "live-window-blur");
    }

    public attach() {
        this.windowAttach();
    }
}

/**
 * live-keydown event handler.
 */
class Keydown extends KeyHandler {
    constructor() {
        super("keydown", "live-keydown");
    }
}

/**
 * live-keyup event handler.
 */
class Keyup extends KeyHandler {
    constructor() {
        super("keyup", "live-keyup");
    }
}

/**
 * live-window-keydown event handler.
 */
class WindowKeydown extends KeyHandler {
    constructor() {
        super("keydown", "live-window-keydown");
    }

    public attach() {
        this.windowAttach();
    }
}

/**
 * live-window-keyup event handler.
 */
class WindowKeyup extends KeyHandler {
    constructor() {
        super("keyup", "live-window-keyup");
    }

    public attach() {
        this.windowAttach();
    }
}

/**
 * live-change form handler.
 */
class Change {
    protected attribute = "live-change";

    constructor() {}

    public isWired(element: Element): boolean {
        if (element.hasAttribute(`${this.attribute}-wired`)) {
            return true;
        }
        element.setAttribute(`${this.attribute}-wired`, "");
        return false;
    }

    public attach() {
        document
            .querySelectorAll(`form[${this.attribute}]`)
            .forEach((element: Element) => {
                element
                    .querySelectorAll("input,select,textarea")
                    .forEach((childElement: Element) => {
                        if (this.isWired(childElement) == true) {
                            return;
                        }
                        childElement.addEventListener("input", (_) => {
                            this.handler(element as HTMLFormElement);
                        });
                    });
            });
    }

    private handler(element: HTMLFormElement) {
        const t = element?.getAttribute(this.attribute);
        if (t === null) {
            return;
        }
        const formData = new FormData(element);
        const values: { [key: string]: any } = {};
        formData.forEach((value, key) => {
            if (!Reflect.has(values, key)) {
                values[key] = value;
                return;
            }
            if (!Array.isArray(values[key])) {
                values[key] = [values[key]];
            }
            values[key].push(value);
        });
        Socket.send({ t: t, d: values });
    }
}

/**
 * live-submit form handler.
 */
class Submit extends LiveHandler {
    constructor() {
        super("submit", "live-submit");
    }

    protected handler(element: HTMLElement, values: LiveValues): EventListener {
        return (e: Event) => {
            if (e.preventDefault) e.preventDefault();

            const t = element?.getAttribute(this.attribute);
            if (t === null) {
                return;
            }
            const data = new FormData(element as HTMLFormElement);
            data.forEach((value: any, name: string) => {
                values[name] = value;
            });
            Socket.send({ t: t, d: values });

            return false;
        };
    }
}

/**
 * live-hook event handler.
 */
class Hook extends LiveHandler {
    constructor() {
        super("", "live-hook");
    }

    public attach() {
        document
            .querySelectorAll(`[${this.attribute}]`)
            .forEach((element: Element) => {
                if (this.isWired(element) == true) {
                    return;
                }
                EventDispatch.mounted(element);
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
    private static keydown: Keydown;
    private static keyup: Keyup;
    private static windowKeydown: WindowKeydown;
    private static windowKeyup: WindowKeyup;
    private static change: Change;
    private static submit: Submit;
    private static hook: Hook;

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
        this.windowKeydown = new WindowKeydown();
        this.windowKeyup = new WindowKeyup();
        this.change = new Change();
        this.submit = new Submit();
        this.hook = new Hook();
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
        this.windowKeyup.attach();
        this.windowKeydown.attach();
        this.change.attach();
        this.submit.attach();
        this.hook.attach();
    }
}
