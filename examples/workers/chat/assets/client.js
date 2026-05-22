const room = new URLSearchParams(location.search).get("room") || "lobby";
const log = document.getElementById("log");
const form = document.getElementById("form");
const nameInput = document.getElementById("name");
const messageInput = document.getElementById("message");
const proto = location.protocol === "https:" ? "wss:" : "ws:";
const basePath = location.pathname.endsWith("/")
  ? location.pathname.slice(0, -1)
  : location.pathname;
const ws = new WebSocket(
  proto +
    "//" +
    location.host +
    basePath +
    "/api/room/" +
    encodeURIComponent(room) +
    "/websocket",
);

function append(text, cls) {
  const div = document.createElement("div");
  if (cls) div.className = cls;
  div.textContent = text;
  log.appendChild(div);
  log.scrollTop = log.scrollHeight;
}

ws.addEventListener("open", () => {
  ws.send(JSON.stringify({ name: nameInput.value || "guest" }));
});

ws.addEventListener("message", event => {
  const data = JSON.parse(event.data);
  if (data.ready) append("connected to room " + room, "meta");
  else if (data.joined) append(data.joined + " joined", "meta");
  else if (data.quit) append(data.quit + " left", "meta");
  else if (data.error) append(data.error, "err");
  else append(data.name + ": " + data.message);
});

ws.addEventListener("close", () => append("disconnected", "meta"));

form.addEventListener("submit", event => {
  event.preventDefault();
  const message = messageInput.value;
  if (!message) return;
  ws.send(JSON.stringify({ name: nameInput.value || "guest", message }));
  messageInput.value = "";
});
