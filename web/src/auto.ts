import { Live } from "./live";
import { Hooks } from "./interop";

declare global {
    interface Window {
        Hooks: Hooks;
        Live: Live;
    }
}

document.addEventListener("DOMContentLoaded", (_) => {
    if (window.Live !== undefined) {
        console.error("window.Live already defined");
    }
    const hooks = window.Hooks || {};
    window.Live = new Live(hooks);
    window.Live.init();
});
