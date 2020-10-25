import { Socket } from "./socket";
import { Events } from "./events";

document.addEventListener("DOMContentLoaded", (_) => {
    Events.init();
    Events.rewire();
    Socket.dial();
});
