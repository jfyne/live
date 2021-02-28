import { LiveEvent, EventDispatch } from "./event";
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
    static handle(event: LiveEvent) {
        Forms.dehydrate();

        const patches = event.data;
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
        const newElement = Patch.html2Node(e.HTML);

        switch (e.Action) {
            case 0: // NOOP
                return;
            case 1: // INSERT
                if (target.parentNode === null) {
                    return;
                }
                EventDispatch.beforeUpdate(target, newElement as Element);
                target.parentNode.insertBefore(newElement, target);
                EventDispatch.updated(target);
                break;
            case 2: // REPLACE
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
            case 3: // APPEND
                EventDispatch.beforeUpdate(target, newElement as Element);
                target.append(newElement);
                EventDispatch.updated(target);
                break;
            case 4: // PREPEND
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
