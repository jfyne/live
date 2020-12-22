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
        patches.map((e: PatchEvent) => {
            const html = document.querySelector("html");
            if (html === null) {
                throw "could not find html node";
            }

            let parent: Element = html;
            let siblings = html.childNodes;
            let target: Element | undefined = undefined;
            for (let i = 0; i < e.Path.length; i++) {
                target = siblings[e.Path[i]] as Element;
                if (target === undefined) {
                    if (e.HTML !== "") {
                        parent.appendChild(Patch.html2Node(e.HTML));
                    }
                    return;
                }
                if (target.childNodes.length) {
                    siblings = target.childNodes;
                }
                parent = target;
            }
            if (target === undefined) {
                return;
            }
            if (e.Action == 0) {
                // NOOP
                return;
            }
            if (e.Action == 1) {
                // INSERT
                if (target.parentNode === null) {
                    return;
                }
                EventDispatch.beforeUpdate(target);
                target.parentNode.insertBefore(Patch.html2Node(e.HTML), target);
                EventDispatch.updated(target);
            }
            if (e.Action == 2) {
                // REPLACE
                EventDispatch.beforeDestroy(target);
                target.outerHTML = e.HTML;
                EventDispatch.destroyed(target);
            }
        });

        Forms.hydrate();
    }

    private static html2Node(html: string): Node {
        const template = document.createElement("template");
        html = html.trim();
        template.innerHTML = html;
        if (template.content.firstChild === null) {
            throw `${html} node not generated`;
        }
        return template.content.firstChild;
    }
}
