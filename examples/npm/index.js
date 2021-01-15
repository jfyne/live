import { Live } from "@jfyne/live";

document.addEventListener("DOMContentLoaded", (_) => {
    const hooks = {
        "example-hook": {
            mounted: () => {
                console.log("This is an example of passing hooks into live anywhere you want.");
            }
        }
    };
    const l = new Live(hooks);
    l.init();
});
