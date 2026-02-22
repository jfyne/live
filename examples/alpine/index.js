import { Live } from "@jfyne/live";
import "alpinejs";

document.addEventListener("DOMContentLoaded", (_) => {
    const hooks = {
        "example-hook": {
            mounted: () => {
                console.log(
                    "This is an example of passing hooks into live anywhere you want."
                );
            },
        },
    };
    const dom = {
        onBeforeElUpdated: (from, to) => {
            if (from.__x) {
                window.Alpine.clone(from.__x, to);
            }
        },
    };
    const l = new Live(hooks, dom);
    l.init();
});
