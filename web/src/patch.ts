import { Event } from "./event";

interface PatchEvent {
    Path: number[];
    HTML: string;
}

/**
 * Handle patches from the backend.
 */
export class Patch {
    static handle(event: Event) {
        const e = event.d as PatchEvent;

        let walkElement: NodeListOf<Element> = document.querySelectorAll(
            ":scope > *"
        );
        let targetElement: any = null;
        e.Path.map((idx) => {
            var currentIDX = 0;
            walkElement.forEach((n) => {
                if (currentIDX == idx) {
                    var proposed = n.querySelectorAll(":scope > *");
                    if (proposed.length !== 0) {
                        walkElement = proposed;
                    } else {
                        targetElement = n as HTMLElement;
                    }
                }
                currentIDX++;
            });
        });
        if (targetElement === null) {
            return;
        }
        targetElement.outerHTML = e.HTML;
    }
}
