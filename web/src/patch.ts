import { Event, EventDispatch } from "./event";
import { Forms } from "./forms";

interface PatchEvent {
    Path: number[];
    Action: number;
    HTML: string;
}

/**
 * Handle patches from the backend.
 */
export class Patch {
    static handle(event: Event) {
        Forms.dehydrate();

        const patches = event.d;
        patches.map(Patch.applyPatch);

        Forms.hydrate();
    }

    private static applyPatch(e: PatchEvent) {
        const html = document.querySelector("html");
        if (html === null) {
            throw "could not find html node";
        }

        let siblings = html.childNodes;
        let target: Element | undefined = undefined;

        for (let i = 0; i < e.Path.length; i++) {
            target = siblings[e.Path[i]] as Element;
            if (target === undefined) {
                console.warn("unhandled patch, path target undefined", e);
                return;
            }
            if (target.childNodes.length) {
                siblings = target.childNodes;
            }
        }

        if (target === undefined) {
            return;
        }

        switch (e.Action) {
            case 0: // NOOP
                return;
            case 1: // INSERT
                if (target.parentNode === null) {
                    return;
                }
                EventDispatch.beforeUpdate(target);
                target.parentNode.insertBefore(Patch.html2Node(e.HTML), target);
                EventDispatch.updated(target);
                break;
            case 2: // REPLACE
                EventDispatch.beforeDestroy(target);
                target.outerHTML = e.HTML;
                EventDispatch.destroyed(target);
                break;
            case 3: // APPEND
                EventDispatch.beforeUpdate(target);
                target.append(Patch.html2Node(e.HTML));
                EventDispatch.updated(target);
                break;
            case 4: // PREPEND
                EventDispatch.beforeUpdate(target);
                target.prepend(Patch.html2Node(e.HTML));
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
