import { ConnectionManager } from "./connection";
import { IslandPatch, Patch, PatchAction } from "./transport/message";
import { Forms } from "./forms";
import { EventDispatch } from "./event";
import { HookRegistry } from "./hooks";

/**
 * IslandProps contains the extracted properties from island attributes.
 */
export interface IslandProps {
    type: string;
    id: string;
    [key: string]: any; // Additional data-* attributes
}

/**
 * IslandConfig defines configuration options for island behavior.
 */
export interface IslandConfig {
    autoConnect?: boolean; // Auto-register with ConnectionManager on connect (default: true)
    preserveForms?: boolean; // Preserve form state during patches (default: true)
}

/**
 * LiveIsland is a custom element that represents an autonomous island
 * in the v2 islands architecture. Each island:
 * - Manages its own lifecycle
 * - Registers with the shared ConnectionManager
 * - Receives and applies patches scoped to itself
 * - Extracts props from element attributes
 *
 * Usage:
 * <live-island type="counter" id="counter-1" data-initial-value="5">
 *   <!-- Island content -->
 * </live-island>
 */
export class LiveIsland extends HTMLElement {
    private connectionManager: ConnectionManager;
    private props: IslandProps | null = null;
    private config: IslandConfig;
    private islandId: string | null = null;

    constructor() {
        super();
        this.connectionManager = ConnectionManager.getInstance();
        this.config = {
            autoConnect: true,
            preserveForms: true,
        };
    }

    /**
     * Define which attributes trigger attributeChangedCallback.
     * We observe 'type' and 'id' attributes.
     */
    static get observedAttributes() {
        return ['type', 'id'];
    }

    /**
     * Called when the element is added to the DOM.
     * Extracts props, registers with ConnectionManager, and sets up message handling.
     */
    connectedCallback(): void {
        console.debug('LiveIsland: connectedCallback');

        // Extract props from attributes
        this.props = this.extractProps();

        if (!this.props.id) {
            console.error('LiveIsland: missing required "id" attribute', this);
            return;
        }

        if (!this.props.type) {
            console.error('LiveIsland: missing required "type" attribute', this);
            return;
        }

        this.islandId = this.props.id;

        // Register with ConnectionManager if auto-connect is enabled
        if (this.config.autoConnect) {
            this.registerWithConnectionManager();
        }

        // Execute mounted hooks for all elements with live-hook attribute
        this.executeHooksForLifecycle('mounted');
    }

    /**
     * Called when the element is removed from the DOM.
     * Unregisters from ConnectionManager and cleans up.
     */
    disconnectedCallback(): void {
        console.debug('LiveIsland: disconnectedCallback', this.islandId);

        // Execute destroyed hooks for all elements with live-hook attribute
        this.executeHooksForLifecycle('destroyed');

        if (this.islandId) {
            this.connectionManager.unregisterIsland(this.islandId);
            // Cleanup island-specific event handlers
            HookRegistry.cleanupIsland(this.islandId);
        }

        // Clean up
        this.islandId = null;
        this.props = null;
    }

    /**
     * Called when an observed attribute changes.
     * Re-extracts props and optionally triggers updates.
     */
    attributeChangedCallback(name: string, oldValue: string | null, newValue: string | null): void {
        console.debug('LiveIsland: attributeChangedCallback', name, oldValue, newValue);

        // If the element is not yet connected, do nothing
        if (!this.isConnected) {
            return;
        }

        // Re-extract props
        const oldProps = this.props;
        this.props = this.extractProps();

        // Handle id change - need to re-register
        if (name === 'id' && oldValue !== newValue) {
            if (oldValue) {
                this.connectionManager.unregisterIsland(oldValue);
            }
            if (newValue) {
                this.islandId = newValue;
                this.registerWithConnectionManager();
            }
        }

        // Handle type change - just update props
        // Server should handle any necessary updates via patches
    }

    /**
     * Extract props from element attributes.
     * - 'type' attribute -> props.type
     * - 'id' attribute -> props.id
     * - 'data-*' attributes -> props[key] (camelCase key)
     */
    private extractProps(): IslandProps {
        const props: IslandProps = {
            type: this.getAttribute('type') || '',
            id: this.getAttribute('id') || '',
        };

        // Extract data-* attributes
        Array.from(this.attributes).forEach(attr => {
            if (attr.name.startsWith('data-')) {
                // Convert data-initial-value -> initialValue
                const key = attr.name
                    .substring(5) // Remove 'data-'
                    .replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
                props[key] = attr.value;
            }
        });

        return props;
    }

    /**
     * Register this island with the ConnectionManager.
     * Sets up the patch handler.
     */
    private registerWithConnectionManager(): void {
        if (!this.islandId) {
            console.error('LiveIsland: cannot register without island ID');
            return;
        }

        console.debug('LiveIsland: registering with ConnectionManager', this.islandId);

        this.connectionManager.registerIsland(this.islandId, this.props!.type, (patch: IslandPatch) => {
            this.handlePatch(patch);
        });
    }

    /**
     * Handle incoming patches for this island.
     * Applies patches to the island's inner DOM.
     */
    private handlePatch(islandPatch: IslandPatch): void {
        console.debug('LiveIsland: received patch', islandPatch);

        // Preserve form state if configured
        if (this.config.preserveForms) {
            Forms.dehydrate();
        }

        // Apply each patch
        islandPatch.patches.forEach(patch => {
            this.applyPatch(patch);
        });

        // Restore form state if configured
        if (this.config.preserveForms) {
            Forms.hydrate();
        }

        // Execute updated hooks for all elements with live-hook attribute
        this.executeHooksForLifecycle('updated');
    }

    /**
     * Apply a single patch to the island's DOM.
     * This is scoped to the island's subtree.
     */
    private applyPatch(patch: Patch): void {
        console.debug('LiveIsland: applying patch', patch);

        // Find the target element within this island
        const target = this.querySelector(`*[${patch.Anchor}]`);
        if (target === null) {
            console.warn('LiveIsland: patch target not found', patch.Anchor, this.islandId);
            return;
        }

        const newElement = this.html2Node(patch.HTML);

        switch (patch.Action) {
            case PatchAction.Noop:
                // No operation
                return;

            case PatchAction.Replace:
                if (patch.HTML === "") {
                    EventDispatch.beforeDestroy(target);
                } else {
                    EventDispatch.beforeUpdate(target, newElement as Element);
                }
                target.outerHTML = patch.HTML;
                if (patch.HTML === "") {
                    EventDispatch.destroyed(target);
                } else {
                    EventDispatch.updated(target);
                }
                break;

            case PatchAction.Append:
                EventDispatch.beforeUpdate(target, newElement as Element);
                target.append(newElement);
                EventDispatch.updated(target);
                break;

            case PatchAction.Prepend:
                EventDispatch.beforeUpdate(target, newElement as Element);
                target.prepend(newElement);
                EventDispatch.updated(target);
                break;

            default:
                console.warn('LiveIsland: unknown patch action', patch.Action);
        }
    }

    /**
     * Convert HTML string to a DOM node.
     */
    private html2Node(html: string): Node {
        const template = document.createElement("template");
        html = html.trim();
        template.innerHTML = html;
        if (template.content.firstChild === null) {
            return document.createTextNode(html);
        }
        return template.content.firstChild;
    }

    /**
     * Get the current props of this island.
     * Useful for debugging and testing.
     */
    public getProps(): IslandProps | null {
        return this.props;
    }

    /**
     * Send an event to the server for this island.
     * This is a convenience method for event handling.
     */
    public sendEvent(eventType: string, data?: any): void {
        if (!this.islandId) {
            console.warn('LiveIsland: cannot send event without island ID');
            return;
        }

        this.connectionManager.sendEvent(this.islandId, eventType, data);
    }

    /**
     * Execute hooks for all elements with live-hook attribute in this island.
     * @param lifecycle - Lifecycle method name
     */
    private executeHooksForLifecycle(lifecycle: 'mounted' | 'updated' | 'destroyed'): void {
        const elements = this.querySelectorAll('[live-hook]');
        elements.forEach(element => {
            HookRegistry.executeElementHook(element, this, lifecycle);
        });
    }
}

/**
 * Register the LiveIsland custom element.
 * This makes <live-island> available in the DOM.
 */
export function registerLiveIsland(): void {
    if (!customElements.get('live-island')) {
        customElements.define('live-island', LiveIsland);
        console.debug('LiveIsland: custom element registered');
    }
}

// Auto-register on module load
registerLiveIsland();
