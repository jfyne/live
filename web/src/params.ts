import { Socket } from "./socket";
import { LiveEvent } from "./event";

/**
 * A values from the "live-value-" attributes. As
 * well as values from the query string in the URL.
 */
export interface Params {
    [key: string]: any;
}

/**
 * GetParams gets the current parameters for an event. This includes
 * any from an element passed in and the URL search string.
 */
export function GetParams(element?: HTMLElement): Params {
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
 * GetURLParams get the params from a url path.
 */
export function GetURLParams(path: string): Params {
    const url = new URL(path, location.origin);
    const urlParams = new URLSearchParams(url.search);

    const output: Params = {};
    urlParams.forEach((value, key) => {
        output[key] = value;
    });

    return output;
}

/**
 * UpdateURLParams update the URL using the push state api, then
 * notify the backend.
 */
export function UpdateURLParams(path: string, element?: HTMLElement) {
    window.history.pushState({}, "", path);
    if (element === undefined) {
        Socket.send(new LiveEvent("params", { ...GetURLParams(path) }));
    } else {
        const params = GetParams(element);
        Socket.sendAndTrack(
            new LiveEvent(
                "params",
                { ...params, ...GetURLParams(path) },
                LiveEvent.GetID()
            ),
            element
        );
    }
}
