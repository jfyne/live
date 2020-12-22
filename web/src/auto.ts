import { Live } from "./live";
import { Hooks } from "./interop";

declare global {
    interface Window {
        Hooks: Hooks;
        Live: Live;
    }
}

document.addEventListener("DOMContentLoaded", (_) => {
    const hooks = window.Hooks || {};
    window.Live = new Live(hooks);
});
