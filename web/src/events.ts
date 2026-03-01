import { ConnectionManager } from "./connection";
import { Forms } from "./forms";

/**
 * A values from the "live-value-" attributes.
 * As well as values from the query string in the URL.
 */
export interface Params {
    [key: string]: any;
}

/**
 * GetParams gets the current parameters for an event. This includes
 * any from an element passed in and the URL search string.
 */
function GetParams(element?: HTMLElement): Params {
    const output: Params = {};

    const urlParams = new URLSearchParams(window.location.search);
    urlParams.forEach((value, key) => {
        output[key] = value;
    });

    if (element === undefined) {
        return output;
    }

    if (!element.hasAttributes()) {
        return output;
    }
    const attrs = element.attributes;
    for (let i = 0; i < attrs.length; i++) {
        if (!attrs[i].name.startsWith("live-value-")) {
            continue;
        }
        output[attrs[i].name.split("live-value-")[1]] = attrs[i].value;
    }
    return output;
}

/**
 * Throttler for rate-limiting events on a per-element basis.
 * Fires immediately on the first call, then rate-limits subsequent calls.
 * A trailing call is fired after the interval elapses if there were suppressed calls.
 */
class Throttler {
    private throttleAttr = "live-throttle";
    private lastFire: WeakMap<Element, number> = new WeakMap();
    private pendingTimers: WeakMap<Element, ReturnType<typeof setTimeout>> = new WeakMap();

    public hasThrottle(element: Element): boolean {
        return element.hasAttribute(this.throttleAttr);
    }

    public throttle(element: Element, e: Event, fn: EventListener): void {
        if (!this.hasThrottle(element)) {
            fn(e);
            return;
        }

        const intervalStr = element.getAttribute(this.throttleAttr);
        if (intervalStr === null) {
            fn(e);
            return;
        }

        const interval = parseInt(intervalStr);
        const now = Date.now();
        const last = this.lastFire.get(element);

        if (last === undefined) {
            // First call: fire immediately
            this.lastFire.set(element, now);
            fn(e);

            // No pending timer yet -- set one to allow a trailing fire if needed
            return;
        }

        // Subsequent call within throttle window: arm a trailing fire timer
        // Cancel any existing pending trailing timer
        const existing = this.pendingTimers.get(element);
        if (existing !== undefined) {
            clearTimeout(existing);
        }

        const elapsed = now - last;
        const remaining = interval - elapsed;

        if (remaining <= 0) {
            // Outside the throttle window: fire immediately
            this.lastFire.set(element, now);
            fn(e);
        } else {
            // Within the throttle window: schedule a trailing fire
            const timer = setTimeout(() => {
                this.lastFire.set(element, Date.now());
                this.pendingTimers.delete(element);
                fn(e);
            }, remaining);
            this.pendingTimers.set(element, timer);
        }
    }

    public cleanup(element: Element): void {
        const timer = this.pendingTimers.get(element);
        if (timer !== undefined) {
            clearTimeout(timer);
            this.pendingTimers.delete(element);
        }
        this.lastFire.delete(element);
    }
}

/**
 * Debouncer for rate-limiting events on a per-element basis.
 */
class Debouncer {
    private debounceAttr = "live-debounce";
    private timers: WeakMap<Element, number> = new WeakMap();

    public hasDebounce(element: Element): boolean {
        return element.hasAttribute(this.debounceAttr);
    }

    public debounce(element: Element, e: Event, fn: EventListener): void {
        // Clear existing timer for this element
        const existingTimer = this.timers.get(element);
        if (existingTimer !== undefined) {
            clearTimeout(existingTimer);
        }

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
            // Store the function to call on blur
            const blurHandler = () => {
                fn(e);
                element.removeEventListener("blur", blurHandler);
            };
            element.addEventListener("blur", blurHandler);
            return;
        }

        // Set a timer for this element
        const timer = window.setTimeout(() => {
            fn(e);
            this.timers.delete(element);
        }, parseInt(debounce));
        this.timers.set(element, timer);
    }

    public cleanup(element: Element): void {
        const timer = this.timers.get(element);
        if (timer !== undefined) {
            clearTimeout(timer);
            this.timers.delete(element);
        }
    }
}

/**
 * EventWiring manages event handlers for a single island.
 * Each island gets its own instance to ensure isolation.
 */
export class EventWiring {
    private islandElement: HTMLElement;
    private islandId: string;
    private debouncer: Debouncer = new Debouncer();
    private throttler: Throttler = new Throttler();
    private cleanupFunctions: Array<() => void> = [];
    private connectionManager: ConnectionManager;

    constructor(islandElement: HTMLElement, islandId: string) {
        this.islandElement = islandElement;
        this.islandId = islandId;
        this.connectionManager = ConnectionManager.getInstance();
    }

    /**
     * Wire all event handlers for this island.
     * This should be called after the island is mounted.
     */
    public wire(): void {
        this.wireClickEvents();
        this.wireContextmenuEvents();
        this.wireMousedownEvents();
        this.wireMouseupEvents();
        this.wireFocusEvents();
        this.wireBlurEvents();
        this.wireKeydownEvents();
        this.wireKeyupEvents();
        this.wireChangeEvents();
        this.wireSubmitEvents();
        this.wireFileInputEvents();
        this.wireWindowFocusEvents();
        this.wireWindowBlurEvents();
        this.wireWindowKeydownEvents();
        this.wireWindowKeyupEvents();
        this.wirePatchEvents();
    }

    /**
     * Clean up all event handlers for this island.
     * This should be called when the island is unmounted.
     */
    public cleanup(): void {
        // Call all registered cleanup functions
        this.cleanupFunctions.forEach(fn => fn());
        this.cleanupFunctions = [];

        // Clean up any pending debounce timers
        const debouncedElements = this.islandElement.querySelectorAll("[live-debounce]");
        debouncedElements.forEach(element => {
            this.debouncer.cleanup(element);
        });

        // Clean up any pending throttle timers
        const throttledElements = this.islandElement.querySelectorAll("[live-throttle]");
        throttledElements.forEach(element => {
            this.throttler.cleanup(element);
        });
    }

    /**
     * Wire click event handlers within the island.
     */
    private wireClickEvents(): void {
        this.wireStandardEvent("click", "live-click");
    }

    /**
     * Wire contextmenu event handlers within the island.
     */
    private wireContextmenuEvents(): void {
        this.wireStandardEvent("contextmenu", "live-contextmenu");
    }

    /**
     * Wire mousedown event handlers within the island.
     */
    private wireMousedownEvents(): void {
        this.wireStandardEvent("mousedown", "live-mousedown");
    }

    /**
     * Wire mouseup event handlers within the island.
     */
    private wireMouseupEvents(): void {
        this.wireStandardEvent("mouseup", "live-mouseup");
    }

    /**
     * Wire focus event handlers within the island.
     */
    private wireFocusEvents(): void {
        this.wireStandardEvent("focus", "live-focus");
    }

    /**
     * Wire blur event handlers within the island.
     */
    private wireBlurEvents(): void {
        this.wireStandardEvent("blur", "live-blur");
    }

    /**
     * Wire keydown event handlers within the island.
     */
    private wireKeydownEvents(): void {
        this.wireKeyEvent("keydown", "live-keydown");
    }

    /**
     * Wire keyup event handlers within the island.
     */
    private wireKeyupEvents(): void {
        this.wireKeyEvent("keyup", "live-keyup");
    }

    /**
     * Wire change event handlers within the island.
     */
    private wireChangeEvents(): void {
        const attribute = "live-change";
        const forms = this.islandElement.querySelectorAll(`form[${attribute}]`);

        forms.forEach((form: Element) => {
            // Set up ACK listener to remove loading state
            const ackHandler = () => {
                form.classList.remove(`${attribute}-loading`);
            };
            form.addEventListener("ack", ackHandler);
            this.cleanupFunctions.push(() => {
                form.removeEventListener("ack", ackHandler);
            });

            // Find all input elements within the form
            const inputs = form.querySelectorAll("input, select, textarea");
            inputs.forEach((input: Element) => {
                const inputHandler = (e: Event) => {
                    if (this.throttler.hasThrottle(input)) {
                        this.throttler.throttle(input, e, () => {
                            this.handleChangeEvent(form as HTMLFormElement, attribute);
                        });
                    } else if (this.debouncer.hasDebounce(input)) {
                        this.debouncer.debounce(input, e, () => {
                            this.handleChangeEvent(form as HTMLFormElement, attribute);
                        });
                    } else {
                        this.handleChangeEvent(form as HTMLFormElement, attribute);
                    }
                };

                input.addEventListener("input", inputHandler);
                this.cleanupFunctions.push(() => {
                    input.removeEventListener("input", inputHandler);
                });
            });

            // Also handle inputs associated via form attribute
            const formId = form.getAttribute("id");
            if (formId) {
                const associatedInputs = this.islandElement.querySelectorAll(
                    `[form="${formId}"]`
                );
                associatedInputs.forEach((input: Element) => {
                    const inputHandler = (e: Event) => {
                        if (this.throttler.hasThrottle(input)) {
                            this.throttler.throttle(input, e, () => {
                                this.handleChangeEvent(form as HTMLFormElement, attribute);
                            });
                        } else if (this.debouncer.hasDebounce(input)) {
                            this.debouncer.debounce(input, e, () => {
                                this.handleChangeEvent(form as HTMLFormElement, attribute);
                            });
                        } else {
                            this.handleChangeEvent(form as HTMLFormElement, attribute);
                        }
                    };

                    input.addEventListener("input", inputHandler);
                    this.cleanupFunctions.push(() => {
                        input.removeEventListener("input", inputHandler);
                    });
                });
            }
        });
    }

    /**
     * Wire submit event handlers within the island.
     */
    private wireSubmitEvents(): void {
        const attribute = "live-submit";
        const forms = this.islandElement.querySelectorAll(`form[${attribute}]`);

        forms.forEach((form: Element) => {
            const submitHandler = (e: Event) => {
                if (e.preventDefault) e.preventDefault();

                const params = GetParams(form as HTMLElement);
                const hasFiles = Forms.hasFiles(form as HTMLFormElement);

                if (hasFiles) {
                    // Handle file upload before sending event
                    const request = new XMLHttpRequest();
                    request.open("POST", "");

                    // Track upload progress and dispatch a custom event on the form.
                    request.upload.addEventListener("progress", (progressEvent: ProgressEvent) => {
                        (form as HTMLFormElement).dispatchEvent(
                            new CustomEvent("live-upload-progress", {
                                bubbles: true,
                                detail: {
                                    loaded: progressEvent.loaded,
                                    total: progressEvent.total,
                                },
                            })
                        );
                    });

                    request.addEventListener("load", () => {
                        this.handleSubmitEvent(form as HTMLFormElement, params, attribute);
                    });

                    request.addEventListener("error", () => {
                        form.classList.remove(`${attribute}-loading`);
                    });

                    request.send(new FormData(form as HTMLFormElement));
                } else {
                    this.handleSubmitEvent(form as HTMLFormElement, params, attribute);
                }

                return false;
            };

            form.addEventListener("submit", submitHandler);
            this.cleanupFunctions.push(() => {
                form.removeEventListener("submit", submitHandler);
            });

            // Set up ACK listener
            const ackHandler = () => {
                form.classList.remove(`${attribute}-loading`);
            };
            form.addEventListener("ack", ackHandler);
            this.cleanupFunctions.push(() => {
                form.removeEventListener("ack", ackHandler);
            });
        });
    }

    /**
     * Wire file input change events within the island.
     * When a file input with the attribute live-upload="<config-name>" changes,
     * a "validate" event is sent to the server with the upload metadata so the
     * server can validate the selected files against the UploadConfig.
     */
    private wireFileInputEvents(): void {
        const fileInputs = this.islandElement.querySelectorAll(`input[type="file"][live-upload]`);

        fileInputs.forEach((input: Element) => {
            const changeHandler = () => {
                const configName = input.getAttribute("live-upload");
                if (configName === null) {
                    return;
                }

                const fileInput = input as HTMLInputElement;
                const files = fileInput.files;
                if (!files || files.length === 0) {
                    return;
                }

                const fileList: Array<{ name: string; size: number; type: string; lastModified: number }> = [];
                for (let i = 0; i < files.length; i++) {
                    const f = files[i];
                    fileList.push({
                        name: f.name,
                        size: f.size,
                        type: f.type,
                        lastModified: f.lastModified,
                    });
                }

                const params = {
                    uploads: {
                        [configName]: fileList,
                    },
                };

                this.connectionManager.sendEvent(this.islandId, "validate", params);
            };

            input.addEventListener("change", changeHandler);
            this.cleanupFunctions.push(() => {
                input.removeEventListener("change", changeHandler);
            });
        });
    }

    /**
     * Wire window focus event handlers for elements with live-window-focus within the island.
     */
    private wireWindowFocusEvents(): void {
        this.wireWindowStandardEvent("focus", "live-window-focus");
    }

    /**
     * Wire window blur event handlers for elements with live-window-blur within the island.
     */
    private wireWindowBlurEvents(): void {
        this.wireWindowStandardEvent("blur", "live-window-blur");
    }

    /**
     * Wire window keydown event handlers for elements with live-window-keydown within the island.
     */
    private wireWindowKeydownEvents(): void {
        this.wireWindowKeyEvent("keydown", "live-window-keydown");
    }

    /**
     * Wire window keyup event handlers for elements with live-window-keyup within the island.
     */
    private wireWindowKeyupEvents(): void {
        this.wireWindowKeyEvent("keyup", "live-window-keyup");
    }

    /**
     * Wire patch navigation event handlers for [live-patch] anchors within the island.
     */
    private wirePatchEvents(): void {
        const elements = this.islandElement.querySelectorAll("[live-patch]");

        elements.forEach((element: Element) => {
            const handler = (e: Event) => {
                e.preventDefault();

                const href = element.getAttribute("href");
                if (href === null || href === "") {
                    return;
                }

                history.pushState({}, "", href);

                // Extract search params from the href
                const params: Params = {};
                // Parse the href to extract search params
                const questionMark = href.indexOf("?");
                if (questionMark !== -1) {
                    const searchString = href.substring(questionMark);
                    const urlParams = new URLSearchParams(searchString);
                    urlParams.forEach((value, key) => {
                        params[key] = value;
                    });
                }

                // Use attribute value as event name, or default to "params"
                const eventName = element.getAttribute("live-patch") || "params";

                this.connectionManager.sendEvent(this.islandId, eventName, params);
            };

            element.addEventListener("click", handler);
            this.cleanupFunctions.push(() => {
                element.removeEventListener("click", handler);
            });
        });
    }

    /**
     * Generic handler for standard events bound to window (focus, blur).
     * Queries elements with the given attribute within the island and attaches
     * the listener to the window.
     */
    private wireWindowStandardEvent(eventType: string, attribute: string): void {
        const elements = this.islandElement.querySelectorAll(`[${attribute}]`);

        elements.forEach((element: Element) => {
            const params = GetParams(element as HTMLElement);

            const handler = (_e: Event) => {
                this.handleWindowStandardEvent(element as HTMLElement, params, attribute);
            };

            window.addEventListener(eventType, handler);
            this.cleanupFunctions.push(() => {
                window.removeEventListener(eventType, handler);
            });
        });
    }

    /**
     * Generic handler for keyboard events bound to window (keydown, keyup).
     * Queries elements with the given attribute within the island and attaches
     * the listener to the window.
     */
    private wireWindowKeyEvent(eventType: string, attribute: string): void {
        const elements = this.islandElement.querySelectorAll(`[${attribute}]`);

        elements.forEach((element: Element) => {
            const params = GetParams(element as HTMLElement);

            const handler = (e: Event) => {
                const ke = e as KeyboardEvent;

                // Check for key filter
                const filter = element.getAttribute("live-key");
                if (filter !== null && ke.key !== filter) {
                    return;
                }

                this.handleWindowKeyEvent(element as HTMLElement, params, attribute, ke);
            };

            window.addEventListener(eventType, handler);
            this.cleanupFunctions.push(() => {
                window.removeEventListener(eventType, handler);
            });
        });
    }

    /**
     * Handle a window standard event by sending it to the server.
     */
    private handleWindowStandardEvent(element: HTMLElement, params: Params, attribute: string): void {
        const eventName = element.getAttribute(attribute);
        if (eventName === null) {
            return;
        }

        element.classList.add(`${attribute}-loading`);

        this.connectionManager.sendEvent(this.islandId, eventName, params);
    }

    /**
     * Handle a window keyboard event by sending it to the server.
     */
    private handleWindowKeyEvent(
        element: HTMLElement,
        params: Params,
        attribute: string,
        keyEvent: KeyboardEvent
    ): void {
        const eventName = element.getAttribute(attribute);
        if (eventName === null) {
            return;
        }

        element.classList.add(`${attribute}-loading`);

        const keyData = {
            key: keyEvent.key,
            altKey: keyEvent.altKey,
            ctrlKey: keyEvent.ctrlKey,
            shiftKey: keyEvent.shiftKey,
            metaKey: keyEvent.metaKey,
        };

        const eventData = { ...params, ...keyData };

        this.connectionManager.sendEvent(this.islandId, eventName, eventData);
    }

    /**
     * Generic handler for standard events (click, focus, blur, etc.)
     */
    private wireStandardEvent(eventType: string, attribute: string): void {
        const elements = this.islandElement.querySelectorAll(`[${attribute}]`);

        elements.forEach((element: Element) => {
            const params = GetParams(element as HTMLElement);

            const handler = (e: Event) => {
                if (this.throttler.hasThrottle(element)) {
                    this.throttler.throttle(element, e, () => {
                        this.handleStandardEvent(element as HTMLElement, params, attribute);
                    });
                } else if (this.debouncer.hasDebounce(element)) {
                    this.debouncer.debounce(element, e, () => {
                        this.handleStandardEvent(element as HTMLElement, params, attribute);
                    });
                } else {
                    this.handleStandardEvent(element as HTMLElement, params, attribute);
                }
            };

            element.addEventListener(eventType, handler);
            this.cleanupFunctions.push(() => {
                element.removeEventListener(eventType, handler);
            });

            // Set up ACK listener
            const ackHandler = () => {
                element.classList.remove(`${attribute}-loading`);
            };
            element.addEventListener("ack", ackHandler);
            this.cleanupFunctions.push(() => {
                element.removeEventListener("ack", ackHandler);
            });
        });
    }

    /**
     * Generic handler for keyboard events (keydown, keyup).
     */
    private wireKeyEvent(eventType: string, attribute: string): void {
        const elements = this.islandElement.querySelectorAll(`[${attribute}]`);

        elements.forEach((element: Element) => {
            const params = GetParams(element as HTMLElement);

            const handler = (e: Event) => {
                const ke = e as KeyboardEvent;

                // Check for key filter
                const filter = element.getAttribute("live-key");
                if (filter !== null && ke.key !== filter) {
                    return;
                }

                if (this.throttler.hasThrottle(element)) {
                    this.throttler.throttle(element, e, () => {
                        this.handleKeyEvent(element as HTMLElement, params, attribute, ke);
                    });
                } else if (this.debouncer.hasDebounce(element)) {
                    this.debouncer.debounce(element, e, () => {
                        this.handleKeyEvent(element as HTMLElement, params, attribute, ke);
                    });
                } else {
                    this.handleKeyEvent(element as HTMLElement, params, attribute, ke);
                }
            };

            element.addEventListener(eventType, handler);
            this.cleanupFunctions.push(() => {
                element.removeEventListener(eventType, handler);
            });

            // Set up ACK listener
            const ackHandler = () => {
                element.classList.remove(`${attribute}-loading`);
            };
            element.addEventListener("ack", ackHandler);
            this.cleanupFunctions.push(() => {
                element.removeEventListener("ack", ackHandler);
            });
        });
    }

    /**
     * Handle a standard event by sending it to the server.
     */
    private handleStandardEvent(element: HTMLElement, params: Params, attribute: string): void {
        const eventName = element.getAttribute(attribute);
        if (eventName === null) {
            return;
        }

        element.classList.add(`${attribute}-loading`);

        // Send event with island ID
        this.connectionManager.sendEvent(this.islandId, eventName, params);
    }

    /**
     * Handle a keyboard event by sending it to the server.
     */
    private handleKeyEvent(
        element: HTMLElement,
        params: Params,
        attribute: string,
        keyEvent: KeyboardEvent
    ): void {
        const eventName = element.getAttribute(attribute);
        if (eventName === null) {
            return;
        }

        element.classList.add(`${attribute}-loading`);

        // Include keyboard event data
        const keyData = {
            key: keyEvent.key,
            altKey: keyEvent.altKey,
            ctrlKey: keyEvent.ctrlKey,
            shiftKey: keyEvent.shiftKey,
            metaKey: keyEvent.metaKey,
        };

        const eventData = { ...params, ...keyData };

        // Send event with island ID
        this.connectionManager.sendEvent(this.islandId, eventName, eventData);
    }

    /**
     * Handle a change event by sending form data to the server.
     */
    private handleChangeEvent(form: HTMLFormElement, attribute: string): void {
        const eventName = form.getAttribute(attribute);
        if (eventName === null) {
            return;
        }

        const values = Forms.serialize(form);
        form.classList.add(`${attribute}-loading`);

        // Send event with island ID
        this.connectionManager.sendEvent(this.islandId, eventName, values);
    }

    /**
     * Handle a submit event by sending form data to the server.
     */
    private handleSubmitEvent(
        form: HTMLFormElement,
        params: Params,
        attribute: string
    ): void {
        const eventName = form.getAttribute(attribute);
        if (eventName === null) {
            return;
        }

        const formData = Forms.serialize(form);
        const eventData = { ...params, ...formData };

        form.classList.add(`${attribute}-loading`);

        // Send event with island ID
        this.connectionManager.sendEvent(this.islandId, eventName, eventData);
    }
}

/**
 * Create and wire event handlers for an island.
 * Returns a cleanup function to remove all event handlers.
 *
 * @param islandElement - The island's root DOM element
 * @param islandId - Unique identifier for the island
 * @returns Cleanup function to remove all event handlers
 */
export function wireIslandEvents(
    islandElement: HTMLElement,
    islandId: string
): () => void {
    const wiring = new EventWiring(islandElement, islandId);
    wiring.wire();
    return () => wiring.cleanup();
}

/**
 * DEPRECATED: Legacy Events class for v1 compatibility.
 * This class is kept temporarily for backward compatibility with live.ts and socket.ts.
 * It will be removed when those files are removed (task 18).
 *
 * @deprecated Use wireIslandEvents for v2 island-scoped events
 */
export class Events {
    /**
     * @deprecated v1 compatibility - does nothing in v2
     */
    public static init() {
        // No-op for v2 - islands wire their own events
        console.warn("Events.init() is deprecated - use wireIslandEvents for v2");
    }

    /**
     * @deprecated v1 compatibility - does nothing in v2
     */
    public static rewire() {
        // No-op for v2 - islands wire their own events
        console.warn("Events.rewire() is deprecated - use wireIslandEvents for v2");
    }
}
