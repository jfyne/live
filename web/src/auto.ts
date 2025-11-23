import { Live, LiveConfig } from "./live";
import { Hooks } from "./interop";

declare global {
    interface Window {
        Hooks: Hooks;
        Live: Live;
        LiveConfig: LiveConfig;
    }
}

document.addEventListener("DOMContentLoaded", (_) => {
    if (window.Live !== undefined) {
        console.error("window.Live already defined");
    }
    const hooks = window.Hooks || {};
    const config = window.LiveConfig || {};
    window.Live = new Live(hooks, undefined, config);
    window.Live.init();
});
