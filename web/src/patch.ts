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

        switch (e.Action) {
            case 0: // NOOP
                return;
            case 1: // INSERT
                if (target.parentNode === null) {
                    return;
                }
                Patch.mergeEmptyNodes(target.parentNode.childNodes);
                EventDispatch.beforeUpdate(target);
                target.parentNode.insertBefore(Patch.html2Node(e.HTML), target);
                EventDispatch.updated(target);
                break;
            case 2: // REPLACE
                if (target.parentNode) {
                    Patch.mergeEmptyNodes(target.parentNode.childNodes);
                }
                EventDispatch.beforeDestroy(target);
                target.outerHTML = e.HTML;
                EventDispatch.destroyed(target);
                break;
            case 3: // APPEND
                Patch.mergeEmptyNodes(target.childNodes);
                EventDispatch.beforeUpdate(target);
                target.append(Patch.html2Node(e.HTML));
                EventDispatch.updated(target);
                break;
            case 4: // PREPEND
                Patch.mergeEmptyNodes(target.childNodes);
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

    /**
     * Firefox appears to leave dangling empty nodes next to each other
     * unlike Chrome. This should check for such nodes and remove them
     * so that our paths continue to work.
     */
    private static mergeEmptyNodes(nodeList: NodeList) {
        let done = false;
        while (done === false) {
            done = true;
            let p: Text = nodeList[0] as Text;
            for (let i = 0; i < nodeList.length; i++) {
                if (i == 0) {
                    p = nodeList[i] as Text;
                    continue;
                }
                if (p.nodeType === 3 && p.data.trim() === "") {
                    if (
                        nodeList[i].nodeType === 3 &&
                        (nodeList[i] as Text).data.trim() === ""
                    ) {
                        (nodeList[i] as ChildNode).remove();
                        done = false;
                        break;
                    }
                }
                p = nodeList[i] as Text;
            }
        }
    }
}
