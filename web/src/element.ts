/**
 * A value from the "live-value-" attributes.
 */
export interface LiveValues {
    [key: string]: any;
}

/**
 * Element helper class.
 */
export class LiveElement {
    /**
     * Pull the values from an HTMLElement.
     */
    static values(element: HTMLElement): LiveValues {
        if (!element.hasAttributes()) {
            return {};
        }
        const attrs = element.attributes;
        const values: LiveValues = {};
        for (let i = 0; i < attrs.length; i++) {
            if (!attrs[i].name.startsWith("live-value-")) {
                continue;
            }
            values[attrs[i].name.split("live-value-")[1]] = attrs[i].value;
        }
        return values;
    }
}
