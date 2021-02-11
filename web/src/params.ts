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
        output[attrs[i].name.split("live-value-")[1]] = output[i].value;
    }
    return output;
}

/**
 * GetURLParams get the params from a url path.
 */
export function GetURLParams(path: string): Params {
    const output: Params = {};

    const urlParams = new URLSearchParams(path);
    urlParams.forEach((value, key) => {
        output[key] = value;
    });

    return output;
}
