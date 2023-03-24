import { Socket } from "./socket";
import { Forms } from "./forms";
import { UpdateURLParams, GetParams, GetURLParams, Params } from "./params";
import { EventDispatch, LiveEvent } from "./event";

/**
 * Standard event handler class. Clicks, focus and blur.
 */
class LiveHandler {
    protected limiter = new Limiter();

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
                const params = GetParams(element as HTMLElement);
                element.addEventListener(this.event, (e) => {
                    if (this.limiter.hasDebounce(element)) {
                        this.limiter.debounce(
                            element,
                            e,
                            this.handler(element as HTMLFormElement, params)
                        );
                    } else {
                        this.handler(element as HTMLFormElement, params)(e);
                    }
                });
                element.addEventListener("ack", (_) => {
                    element.classList.remove(`${this.attribute}-loading`);
                });
            });
    }

    protected windowAttach() {
        document
            .querySelectorAll(`*[${this.attribute}]`)
            .forEach((element: Element) => {
                if (this.isWired(element) === true) {
                    return;
                }
                const params = GetParams(element as HTMLElement);
                window.addEventListener(
                    this.event,
                    this.handler(element as HTMLElement, params)
                );
                window.addEventListener("ack", (_) => {
                    element.classList.remove(`${this.attribute}-loading`);
                });
            });
    }

    protected handler(element: HTMLElement, params: Params): EventListener {
        return (_: Event) => {
            const t = element?.getAttribute(this.attribute);
            if (t === null) {
                return;
            }
            element.classList.add(`${this.attribute}-loading`);
            Socket.sendAndTrack(
                new LiveEvent(t, params, LiveEvent.GetID()),
                element
            );
        };
    }
}

/**
 * KeyHandler handle key events.
 */
export class KeyHandler extends LiveHandler {
    protected handler(element: HTMLElement, params: Params): EventListener {
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
            element.classList.add(`${this.attribute}-loading`);
            const keyData = {
                key: ke.key,
                altKey: ke.altKey,
                ctrlKey: ke.ctrlKey,
                shiftKey: ke.shiftKey,
                metaKey: ke.metaKey,
            };
            Socket.sendAndTrack(
                new LiveEvent(t, { ...params, ...keyData }, LiveEvent.GetID()),
                element
            );
        };
    }
}

class Limiter {
    private debounceAttr = "live-debounce";
    private debounceEvent: any;

    public hasDebounce(element: Element): boolean {
        return element.hasAttribute(this.debounceAttr);
    }

    public debounce(element: Element, e: Event, fn: EventListener) {
        clearTimeout(this.debounceEvent);
        if (!this.hasDebounce(element)) {
            fn(e);
            return;
        }
        const debounce = element.getAttribute(this.debounceAttr);
        if (debounce === null) {
            fn(e);
            return;
        }
        if (debounce === "blur") {
            this.debounceEvent = fn;
            element.addEventListener("blur", () => {
                this.debounceEvent();
            });
            return;
        }
        this.debounceEvent = setTimeout(() => {
            fn(e);
        }, parseInt(debounce));
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
 * live-contextmenu attribute handling.
 */
class Contextmenu extends LiveHandler {
    constructor() {
        super("contextmenu", "live-contextmenu");
    }
}

/**
 * live-mousedown attribute handling.
 */
class Mousedown extends LiveHandler {
    constructor() {
        super("mousedown", "live-mousedown");
    }
}

/**
 * live-mouseup attribute handling.
 */
class Mouseup extends LiveHandler {
    constructor() {
        super("mouseup", "live-mouseup");
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
    protected limiter = new Limiter();

    constructor() {}

    public isWired(element: Element): boolean {
        if (element.hasAttribute(`${this.attribute}-wired`)) {
            return true;
        }
        element.setAttribute(`${this.attribute}-wired`, "");
        return false;
    }
    
    public attach() {
        let forms: Element[] = [];
        document
            .querySelectorAll(`form[${this.attribute}]`)
            .forEach((element: Element) => {
                element.addEventListener("ack", (_) => {
                    element.classList.remove(`${this.attribute}-loading`);
                });
                forms.push(element);
                element
                    .querySelectorAll(`input,select,textarea`)
                    .forEach((childElement: Element) => {
                        this.addEvent(element, childElement);
                    });
            });
        forms.forEach((element: Element) => {
            document
                .querySelectorAll(`[form=${element.getAttribute("id")}]`)
                .forEach((childElement) => {
                    this.addEvent(element, childElement);
                });
        });
    };

    private addEvent(element: Element, childElement: Element) {
        if (this.isWired(childElement)) {
            return;
        }
        childElement.addEventListener("input", (e) => {
            if (this.limiter.hasDebounce(childElement)) {
                this.limiter.debounce(childElement, e, () => {
                    this.handler(element as HTMLFormElement);
                });
            } else {
                this.handler(element as HTMLFormElement);
            }
        });
    }

    private handler(element: HTMLFormElement) {
        const t = element?.getAttribute(this.attribute);
        if (t === null) {
            return;
        }
        const values: { [key: string]: any } = Forms.serialize(element);
        element.classList.add(`${this.attribute}-loading`);
        Socket.sendAndTrack(
            new LiveEvent(t, values, LiveEvent.GetID()),
            element
        );
    }
}

/**
 * live-submit form handler.
 */
class Submit extends LiveHandler {
    constructor() {
        super("submit", "live-submit");
    }

    protected handler(element: HTMLElement, params: Params): EventListener {
        return (e: Event) => {
            if (e.preventDefault) e.preventDefault();

            const hasFiles = Forms.hasFiles(element as HTMLFormElement);
            if (hasFiles === true) {
                const request = new XMLHttpRequest();
                request.open("POST", "");
                request.addEventListener('load', () => {
                    this.sendEvent(element, params);
                });

                request.send(new FormData(element as HTMLFormElement));
            } else {
                this.sendEvent(element, params);
            }
            return false;
        };
    }

    protected sendEvent(element: HTMLElement, params: Params) {
        const t = element?.getAttribute(this.attribute);
        if (t === null) {
            return;
        }

        var vals = { ...params };

        const data: { [key: string]: any } = Forms.serialize(
            element as HTMLFormElement
        );
        Object.keys(data).map((k) => {
            vals[k] = data[k];
        });
        element.classList.add(`${this.attribute}-loading`);
        Socket.sendAndTrack(
            new LiveEvent(t, vals, LiveEvent.GetID()),
            element
        );
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
 * live-patch event handler.
 */
class Patch extends LiveHandler {
    constructor() {
        super("click", "live-patch");
    }

    protected handler(element: HTMLElement, _: Params): EventListener {
        return (e: Event) => {
            if (e.preventDefault) e.preventDefault();
            const path = element.getAttribute("href");
            if (path === null) {
                return;
            }
            UpdateURLParams(path, element);
            return false;
        };
    }
}

/**
 * Handle all events.
 */
export class Events {
    private static clicks: Click;
    private static contextmenu: Contextmenu;
    private static mousedown: Mousedown;
    private static mouseup: Mouseup;
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
    private static patch: Patch;

    /**
     * Initialise all the event wiring.
     */
    public static init() {
        this.clicks = new Click();
        this.contextmenu = new Contextmenu();
        this.mousedown = new Mousedown();
        this.mouseup = new Mouseup();
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
        this.patch = new Patch();

        this.handleBrowserNav();
    }

    /**
     * Re-attach all events when we have re-rendered.
     */
    public static rewire() {
        this.clicks.attach();
        this.contextmenu.attach();
        this.mousedown.attach();
        this.mouseup.attach();
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
        this.patch.attach();
    }

    /**
     * Watch the browser popstate so that we can send a params
     * change event to the server.
     */
    private static handleBrowserNav() {
        window.onpopstate = function (_: any) {
            Socket.send(
                new LiveEvent(
                    "params",
                    GetURLParams(document.location.search),
                    LiveEvent.GetID()
                )
            );
        };
    }
}
