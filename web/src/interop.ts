/**
 * Hooks supplied for interop.
 */
export interface Hooks {
    [id: string]: Hook;
}

/**
 * A hook for running external JS.
 */
export interface Hook {
    /**
     * The element has been added to the DOM and its server
     * LiveHandler has finished mounting
     */
    mounted?: () => void;

    /**
     * The element is about to be updated in the DOM.
     * Note: any call here must be synchronous as the operation
     * cannot be deferred or cancelled.
     */
    beforeUpdate?: () => void;

    /**
     * The element has been updated in the DOM by the server
     */
    updated?: () => void;

    /**
     * The element is about to be removed from the DOM.
     * Note: any call here must be synchronous as the operation
     * cannot be deferred or cancelled.
     */
    beforeDestroy?: () => void;

    /**
     * The element has been removed from the page, either by
     * a parent update, or by the parent being removed entirely
     */
    destroyed?: () => void;

    /**
     * The element's parent LiveHandler has disconnected from
     * the server
     */
    disconnected?: () => void;

    /**
     * The element's parent LiveHandler has reconnected to the
     * server
     */
    reconnected?: () => void;
}

/**
 * The DOM management interace. This allows external JS libraries to
 * interop with Live.
 */
export interface DOM {
    /**
     * The fromEl and toEl DOM nodes are passed to the function
     * just before the DOM patch operations occurs in Live. This
     * allows external libraries to (re)initialize DOM elements
     * or copy attributes as necessary as Live performs its own
     * patch operations. The update operation cannot be cancelled
     * or deferred, and the return value is ignored.
     */
    onBeforeElUpdated?: (fromEl: Element, toEl: Element) => void;
}
