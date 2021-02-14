import { LiveEvent, EventDispatch } from "./event";
import { Forms } from "./forms";

interface PatchEvent {
    Path: number[];
    Action: number;
    HTML: string;
}

/**
 * When the backend wants to patch the frontend, this takes each
 * patch and applies it in sequence.
 */
export function HandleDomPatch(event: LiveEvent) {
    Forms.dehydrate();

    const patches = event.data;
    patches.map(applyPatch);

    Forms.hydrate();
}

function applyPatch(e: PatchEvent) {
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
            target.parentNode.insertBefore(html2Node(e.HTML), target);
            EventDispatch.updated(target);
            break;
        case 2: // REPLACE
            EventDispatch.beforeDestroy(target);
            target.outerHTML = e.HTML;
            EventDispatch.destroyed(target);
            break;
        case 3: // APPEND
            EventDispatch.beforeUpdate(target);
            target.append(html2Node(e.HTML));
            EventDispatch.updated(target);
            break;
        case 4: // PREPEND
            EventDispatch.beforeUpdate(target);
            target.prepend(html2Node(e.HTML));
            EventDispatch.updated(target);
            break;
    }
}

function html2Node(html: string): Node {
    const template = document.createElement("template");
    html = html.trim();
    template.innerHTML = html;
    if (template.content.firstChild === null) {
        return document.createTextNode(html);
    }
    return template.content.firstChild;
}
