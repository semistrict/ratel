import { handleErrors } from "./errors.js";

export class ChatRoom {
  constructor(state, env) {
    this.state = state;
    this.storage = state.storage;
    this.sessions = new Map();
    this.lastTimestamp = 0;
  }

  async fetch(request) {
    return handleErrors(request, async () => {
      const url = new URL(request.url);
      if (url.pathname !== "/websocket") return new Response("not found", { status: 404 });
      if (request.headers.get("Upgrade") !== "websocket") {
        return new Response("expected websocket", { status: 400 });
      }

      const pair = new WebSocketPair();
      await this.handleSession(pair[1]);
      return new Response(null, { status: 101, webSocket: pair[0] });
    });
  }

  async handleSession(webSocket) {
    webSocket.accept();
    const session = { blockedMessages: [] };
    this.sessions.set(webSocket, session);

    for (const other of this.sessions.values()) {
      if (other.name) session.blockedMessages.push(JSON.stringify({ joined: other.name }));
    }

    const history = await this.storage.list({ reverse: true, limit: 100 });
    const backlog = Array.from(history.values()).reverse();
    for (const value of backlog) session.blockedMessages.push(value);

    webSocket.addEventListener("message", event => {
      this.webSocketMessage(webSocket, event.data);
    });
    webSocket.addEventListener("close", () => this.closeOrErrorHandler(webSocket));
    webSocket.addEventListener("error", () => this.closeOrErrorHandler(webSocket));
  }

  async webSocketMessage(webSocket, msg) {
    try {
      const session = this.sessions.get(webSocket);
      if (!session || session.quit) return;

      let data = JSON.parse(msg);
      if (!session.name) {
        session.name = String(data.name || "anonymous").slice(0, 32);
        for (const queued of session.blockedMessages) webSocket.send(queued);
        delete session.blockedMessages;
        this.broadcast({ joined: session.name });
        webSocket.send(JSON.stringify({ ready: true }));
        return;
      }

      if (data.name) session.name = String(data.name).slice(0, 32);

      const message = String(data.message || "");
      if (!message) return;
      if (message.length > 256) {
        webSocket.send(JSON.stringify({ error: "message too long" }));
        return;
      }

      const timestamp = Math.max(Date.now(), this.lastTimestamp + 1);
      this.lastTimestamp = timestamp;
      data = { name: session.name, message, timestamp };
      const dataStr = JSON.stringify(data);
      this.broadcast(dataStr);
      await this.storage.put(new Date(timestamp).toISOString(), dataStr);
    } catch (err) {
      webSocket.send(JSON.stringify({ error: err.stack || String(err) }));
    }
  }

  closeOrErrorHandler(webSocket) {
    const session = this.sessions.get(webSocket);
    if (!session) return;
    session.quit = true;
    this.sessions.delete(webSocket);
    if (session.name) this.broadcast({ quit: session.name });
  }

  broadcast(message) {
    const data = typeof message === "string" ? message : JSON.stringify(message);
    const quitters = [];
    this.sessions.forEach((session, webSocket) => {
      if (session.name) {
        try {
          webSocket.send(data);
        } catch (err) {
          session.quit = true;
          quitters.push(session);
          this.sessions.delete(webSocket);
        }
      } else {
        session.blockedMessages.push(data);
      }
    });
    for (const quitter of quitters) {
      if (quitter.name) this.broadcast({ quit: quitter.name });
    }
  }
}
