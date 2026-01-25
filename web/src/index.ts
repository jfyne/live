/**
 * Live v2 Islands Architecture - Public API
 *
 * This is the main entry point for the Live v2 framework.
 * Import this module to use Live programmatically.
 *
 * For automatic initialization, use auto.ts instead.
 */

// Island custom element
export { LiveIsland, IslandProps, IslandConfig, registerLiveIsland } from "./island";

// Connection management
export { ConnectionManager, IslandHandler } from "./connection";

// Transport layer
export { Transport, ConnectionState } from "./transport/transport";
export { TransportNegotiator, NegotiatorConfig } from "./transport/negotiator";
export { WebSocketTransport } from "./transport/websocket";
export { SSETransport } from "./transport/sse";
export {
    TransportMessage,
    IslandPatch,
    Patch,
    PatchAction,
    MessageType
} from "./transport/message";

// Hooks system
export {
    HookRegistry,
    HookContext,
    IslandHook,
    IslandHooks,
    autoRegisterHooks
} from "./hooks";

// Hook interface (backward compatible)
export { Hook, Hooks } from "./interop";

// Form utilities
export { Forms } from "./forms";

// Event utilities (for advanced use)
export { EventDispatch, LiveEvent } from "./event";
