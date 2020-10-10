let conn;

function newEvent(type, data) {
  return JSON.stringify({
    t: type,
    d: data,
  });
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
    console.log(ev.data);
  });
}
dial();
