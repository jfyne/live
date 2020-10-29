/**
 * Represents an event we both receive and
 * send over the socket.
 */
export interface Event {
    t: string;
    d: { [key: string]: any };
}
