let conn;

function newEvent(type, data) {
  return JSON.stringify({
    t: type,
    d: data,
  });
}

function handlePatch(e) {
    var walkElement = document.querySelectorAll(":scope > *");
    var targetElement = null;
    e.Path.map(idx => {
        var currentIDX = 0;
        walkElement.forEach(n => {
            if (currentIDX == idx) {
                var proposed = n.querySelectorAll(":scope > *");
                if (proposed.length !== 0) {
                    walkElement = proposed;
                } else {
                    targetElement = n;
                }
            }
            currentIDX++;
        })
    });
    targetElement.outerHTML = e.HTML;
}

function dial() {
  conn = new WebSocket(`ws://${location.host}/socket${location.pathname}`);

  conn.addEventListener("close", (ev) => {
    console.warn(
      `WebSocket Disconnected code: ${ev.code}, reason: ${ev.reason}`
    );
    if (ev.code !== 1001) {
      console.warn("Reconnecting in 1s");
      setTimeout(dial, 1000);
    }
  });
  conn.addEventListener("open", (ev) => {
    console.info("websocket connected", ev);
    conn.send(newEvent("ping", location.pathname));
  });

  // This is where we handle messages received.
  conn.addEventListener("message", (ev) => {
    if (typeof ev.data !== "string") {
      console.error("unexpected message type", typeof ev.data);
      return;
    }
    e = JSON.parse(ev.data);
    switch(e.t) {
        case "patch":
            handlePatch(e.d);
            break;
        default:
            console.log(e);
    }
  });
}

function attachClickHandlers() {
    document.querySelectorAll("*[live-click]").forEach((element) => {
        element.addEventListener("click", e => {
            conn.send(newEvent(element.getAttribute("live-click")));
        });
    });
}

document.addEventListener("DOMContentLoaded", _ => {
    dial();
    attachClickHandlers();
});
