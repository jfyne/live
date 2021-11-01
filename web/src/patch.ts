import { LiveEvent, EventDispatch } from "./event";
import { Forms } from "./forms";

interface PatchEvent {
    Anchor: string;
    Action: number;
    HTML: string;
}

/**
 * Handle patches from the backend.
 */
export class Patch {
    static handle(event: LiveEvent) {
        Forms.dehydrate();

        const patches = event.data;
        patches.map(Patch.applyPatch);

        Forms.hydrate();
    }

    private static applyPatch(e: PatchEvent) {
        const target = document.querySelector(`*[${e.Anchor}]`);
        if (target === null) {
            return;
        }

        const newElement = Patch.html2Node(e.HTML);
        switch (e.Action) {
            case 0: // NOOP
                return;
            case 1: // REPLACE
                if (e.HTML === "") {
                    EventDispatch.beforeDestroy(target);
                } else {
                    EventDispatch.beforeUpdate(target, newElement as Element);
                }
                target.outerHTML = e.HTML;
                if (e.HTML === "") {
                    EventDispatch.destroyed(target);
                } else {
                    EventDispatch.updated(target);
                }
                break;
            case 2: // APPEND
                EventDispatch.beforeUpdate(target, newElement as Element);
                target.append(newElement);
                EventDispatch.updated(target);
                break;
            case 3: // PREPEND
                EventDispatch.beforeUpdate(target, newElement as Element);
                target.prepend(newElement);
                EventDispatch.updated(target);
                break;
        }
    }

    private static html2Node(html: string): Node {
        const template = document.createElement("template");
        html = html.trim();
        template.innerHTML = html;
        if (template.content.firstChild === null) {
            return document.createTextNode(html);
        }
        return template.content.firstChild;
    }
}
