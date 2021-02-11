/**
 * Element helper class.
 */
export class LiveElement {
    static hook(element: HTMLElement): string | null {
        if (element.getAttribute === undefined) {
            return null;
        }
        return element.getAttribute("live-hook");
    }
}
