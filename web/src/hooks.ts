import { LiveIsland } from "./island";
import { Hook } from "./interop";

/**
 * HookContext provides the context for hook execution.
 * Each hook receives access to the element, the island instance,
 * and utility functions for interacting with the island.
 */
export interface HookContext {
    /** The DOM element this hook is attached to */
    el: Element;
    /** The LiveIsland instance this element belongs to */
    island: LiveIsland;
    /** Send an event to the server for this island */
    pushEvent: (eventType: string, data?: any) => void;
    /** Register a handler for server-sent events */
    handleEvent: (eventType: string, callback: (data: any) => void) => void;
}

/**
 * IslandHook wraps a Hook with island context.
 * When hook lifecycle methods are called, they receive the HookContext.
 */
export type IslandHook = {
    [K in keyof Hook]: Hook[K] extends (() => void) | undefined
        ? ((this: HookContext) => void) | undefined
        : Hook[K];
};

/**
 * IslandHooks is a collection of named hooks for an island.
 */
export interface IslandHooks {
    [id: string]: IslandHook;
}

/**
 * HookRegistry manages hook registration and execution for islands.
 * Hooks are registered globally but executed with island-specific context.
 */
export class HookRegistry {
    private static hooks: IslandHooks = {};
    private static eventHandlers: Map<string, Map<string, ((data: any) => void)[]>> = new Map();

    /**
     * Register hooks for use with islands.
     * Typically called at application startup:
     *
     * ```typescript
     * HookRegistry.register({
     *   MyComponent: {
     *     mounted() {
     *       console.log('Mounted:', this.el);
     *       this.pushEvent('init', {});
     *     },
     *     updated() {
     *       console.log('Updated:', this.el);
     *     }
     *   }
     * });
     * ```
     *
     * @param hooks - Object mapping hook names to hook implementations
     */
    static register(hooks: IslandHooks): void {
        Object.assign(this.hooks, hooks);
    }

    /**
     * Get a registered hook by name.
     * @param name - Hook name
     * @returns The hook or undefined if not found
     */
    static getHook(name: string): IslandHook | undefined {
        return this.hooks[name];
    }

    /**
     * Execute a hook lifecycle method with the given context.
     * @param hookName - Name of the registered hook
     * @param lifecycle - Lifecycle method name (mounted, updated, etc.)
     * @param element - DOM element
     * @param island - LiveIsland instance
     */
    static executeHook(
        hookName: string,
        lifecycle: keyof Hook,
        element: Element,
        island: LiveIsland
    ): void {
        const hook = this.hooks[hookName];
        if (!hook) {
            return;
        }

        const lifecycleMethod = hook[lifecycle];
        if (typeof lifecycleMethod !== 'function') {
            return;
        }

        // Create context for this hook execution
        const context = this.createContext(element, island);

        try {
            // Call the hook method with the context as 'this'
            lifecycleMethod.call(context);
        } catch (err) {
            console.error(`HookRegistry: error executing ${lifecycle} hook for ${hookName}`, err);
        }
    }

    /**
     * Find and execute hooks for an element within an island.
     * Looks for the 'live-hook' attribute on the element.
     * @param element - DOM element
     * @param island - LiveIsland instance
     * @param lifecycle - Lifecycle method name
     */
    static executeElementHook(
        element: Element,
        island: LiveIsland,
        lifecycle: keyof Hook
    ): void {
        const hookName = element.getAttribute('live-hook');
        if (!hookName) {
            return;
        }

        this.executeHook(hookName, lifecycle, element, island);
    }

    /**
     * Handle an event pushed from the server for a specific island.
     * @param islandId - Island ID
     * @param eventType - Event type
     * @param data - Event data
     */
    static handleServerEvent(islandId: string, eventType: string, data: any): void {
        const islandHandlers = this.eventHandlers.get(islandId);
        if (!islandHandlers) {
            return;
        }

        const handlers = islandHandlers.get(eventType);
        if (!handlers) {
            return;
        }

        handlers.forEach(handler => {
            try {
                handler(data);
            } catch (err) {
                console.error(`HookRegistry: error handling event ${eventType} for island ${islandId}`, err);
            }
        });
    }

    /**
     * Create a HookContext for hook execution.
     * @param element - DOM element
     * @param island - LiveIsland instance
     * @returns HookContext object
     */
    private static createContext(element: Element, island: LiveIsland): HookContext {
        const props = island.getProps();
        const islandId = props?.id || '';

        return {
            el: element,
            island: island,
            pushEvent: (eventType: string, data?: any) => {
                island.sendEvent(eventType, data);
            },
            handleEvent: (eventType: string, callback: (data: any) => void) => {
                if (!this.eventHandlers.has(islandId)) {
                    this.eventHandlers.set(islandId, new Map());
                }

                const islandHandlers = this.eventHandlers.get(islandId)!;
                if (!islandHandlers.has(eventType)) {
                    islandHandlers.set(eventType, []);
                }

                islandHandlers.get(eventType)!.push(callback);
            }
        };
    }

    /**
     * Clean up event handlers for an island when it's removed.
     * @param islandId - Island ID
     */
    static cleanupIsland(islandId: string): void {
        this.eventHandlers.delete(islandId);
    }

    /**
     * Get all registered hook names.
     * Useful for debugging and testing.
     */
    static getRegisteredHooks(): string[] {
        return Object.keys(this.hooks);
    }

    /**
     * Clear all hooks and event handlers.
     * Mainly used for testing.
     */
    static clear(): void {
        this.hooks = {};
        this.eventHandlers.clear();
    }
}

/**
 * Global hooks registration interface.
 * Allows registering hooks on window.Hooks before initialization.
 */
declare global {
    interface Window {
        Hooks?: IslandHooks;
    }
}

/**
 * Auto-register hooks from window.Hooks if available.
 * This maintains backward compatibility with v1 hook registration patterns.
 */
export function autoRegisterHooks(): void {
    if (typeof window !== 'undefined' && window.Hooks) {
        HookRegistry.register(window.Hooks);
        console.debug('HookRegistry: auto-registered window.Hooks');
    }
}
