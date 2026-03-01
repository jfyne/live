import { EventDispatch } from "./event";
import { Forms } from "./forms";

interface PatchEvent {
    Anchor: string;
    Action: number;
    HTML: string;
}

/**
 * Apply patches within an island's scope only.
 * This ensures patches never affect other islands or the document.
 */
export function applyIslandPatch(
    islandElement: HTMLElement,
    patch: PatchEvent,
    rewireEvents: (element: HTMLElement) => void
) {
    // Find anchor within island element only - NOT document-wide
    const target = islandElement.querySelector(`*[${patch.Anchor}]`);
    if (target === null) {
        return;
    }

    const newElement = html2Node(patch.HTML);

    switch (patch.Action) {
        case 0: // NOOP
            return;
        case 1: // REPLACE
            if (patch.HTML === "") {
                EventDispatch.beforeDestroy(target);
            } else {
                EventDispatch.beforeUpdate(target, newElement as Element);
            }
            target.outerHTML = patch.HTML;
            if (patch.HTML === "") {
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

    // Re-wire events for newly added elements within this island
    rewireEvents(islandElement);
}

/**
 * Apply patches to a specific island, preserving form state within that island.
 */
export function applyIslandPatches(
    islandElement: HTMLElement,
    patches: PatchEvent[],
    rewireEvents: (element: HTMLElement) => void
) {
    // Dehydrate forms within this island only
    const formState = dehydrateIslandForms(islandElement);

    // Apply each patch within island scope
    patches.forEach((patch) => {
        applyIslandPatch(islandElement, patch, rewireEvents);
    });

    // Restore form state within this island only
    hydrateIslandForms(islandElement, formState);
}

/**
 * Dehydrate forms within island scope only.
 * Returns a map of form ID to form state.
 */
function dehydrateIslandForms(islandElement: HTMLElement): Map<string, any[]> {
    const formState = new Map<string, any[]>();
    const forms = islandElement.querySelectorAll("form");

    forms.forEach((f) => {
        if (f.id === "") {
            console.error(
                "form does not have an ID. DOM updates may be affected",
                f
            );
            return;
        }

        // Store form state scoped to this island
        const state: any[] = [];
        new FormData(f).forEach((value: any, name: string) => {
            const input = {
                name: name,
                value: value,
                focus:
                    f.querySelector(`[name="${name}"]`) ==
                    document.activeElement,
            };
            state.push(input);
        });

        formState.set(f.id, state);
    });

    return formState;
}

/**
 * Hydrate forms within island scope only.
 */
function hydrateIslandForms(islandElement: HTMLElement, formState: Map<string, any[]>) {
    formState.forEach((state, formId) => {
        const form = islandElement.querySelector(`#${formId}`);
        if (form === null) {
            return;
        }

        state.forEach((i: any) => {
            const input = form.querySelector(
                `[name="${i.name}"]`
            ) as HTMLInputElement;
            if (input === null) {
                return;
            }
            switch (input.type) {
                case "file":
                    break;
                case "checkbox":
                    if (i.value === "on") {
                        input.checked = true;
                    }
                    break;
                default:
                    input.value = i.value;
                    if (i.focus === true) {
                        input.focus();
                    }
                    break;
            }
        });
    });
}

/**
 * Convert HTML string to DOM node.
 */
function html2Node(html: string): Node {
    const template = document.createElement("template");
    html = html.trim();
    template.innerHTML = html;
    if (template.content.firstChild === null) {
        return document.createTextNode(html);
    }
    return template.content.firstChild;
}

/**
 * Legacy Patch class for backward compatibility.
 * This maintains the v1 API but internally uses island-scoped logic.
 * @deprecated Use applyIslandPatches instead
 */
export class Patch {
    static handle(event: any) {
        // Preserve form state globally for v1 compatibility
        Forms.dehydrate();

        const patches = event.data;
        patches.map(Patch.applyPatch);

        // Restore form state globally
        Forms.hydrate();
    }

    private static applyPatch(e: PatchEvent) {
        const target = document.querySelector(`*[${e.Anchor}]`);
        if (target === null) {
            return;
        }

        const newElement = html2Node(e.HTML);
        switch (e.Action) {
            case 0: // NOOP
                return;
            case 1: // REPLACE
                if (e.HTML === "") {
                    EventDispatch.beforeDestroy(target);
                    target.remove();
                    EventDispatch.destroyed(target);
                } else {
                    EventDispatch.beforeUpdate(target, newElement as Element);
                    target.replaceWith(newElement);
                    EventDispatch.updated(newElement as Element);
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
}
