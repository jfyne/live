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
        const elements = this.islandElement.querySelectorAll("[live-debounce]");
        elements.forEach(element => {
            this.debouncer.cleanup(element);
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
                    if (this.debouncer.hasDebounce(input)) {
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
                        if (this.debouncer.hasDebounce(input)) {
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
                    request.addEventListener("load", () => {
                        this.handleSubmitEvent(form as HTMLFormElement, params, attribute);
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
     * Generic handler for standard events (click, focus, blur, etc.)
     */
    private wireStandardEvent(eventType: string, attribute: string): void {
        const elements = this.islandElement.querySelectorAll(`[${attribute}]`);

        elements.forEach((element: Element) => {
            const params = GetParams(element as HTMLElement);

            const handler = (e: Event) => {
                if (this.debouncer.hasDebounce(element)) {
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

                if (this.debouncer.hasDebounce(element)) {
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
