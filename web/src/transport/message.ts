/**
 * TransportMessage is the base message structure for all transport messages.
 * This matches the Go Event struct on the server side.
 */
export interface TransportMessage {
    t: string;           // type
    i?: number;          // id
    island?: string;     // island identifier for routing
    d?: any;            // data payload
    s?: any;            // self data
}

/**
 * IslandPatch wraps patches with island routing information.
 * This matches the Go IslandPatch struct.
 */
export interface IslandPatch {
    island_id: string;
    patches: Patch[];
}

/**
 * Patch represents a single DOM update operation.
 * This matches the Go Patch struct.
 */
export interface Patch {
    Anchor: string;
    Action: PatchAction;
    HTML: string;
    island_id?: string;
}

/**
 * PatchAction defines the type of patch operation.
 */
export enum PatchAction {
    Noop = 0,
    Replace = 1,
    Append = 2,
    Prepend = 3,
}

/**
 * Message type constants matching server-side event types.
 */
export const MessageType = {
    Error: "err",
    Patch: "patch",
    Ack: "ack",
    Connect: "connect",
    Params: "params",
    Redirect: "redirect",
} as const;
