import { handleErrors } from "./errors.js";
import { createRatelSQL } from "./ratel-sql.js";

export class ChatRoom {
  constructor(state, env) {
    this.state = state;
    this.env = env;
    this.sessions = new Map();
    this.lastTimestamp = 0;
  }

  async fetch(request) {
    return handleErrors(request, async () => {
      const url = new URL(request.url);
      if (url.pathname !== "/websocket") return new Response("not found", { status: 404 });
      const actor = request.headers.get("X-Ratel-Actor-Scope");
      if (!actor) return new Response("missing actor scope", { status: 400 });
      this.sql = createRatelSQL(this.env.__RATEL_SQL, actor);
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

    const history = (await this.sql
      .exec(
        "SELECT name, message, timestamp FROM system.ratel_chat_messages WHERE actor_id = $1 ORDER BY timestamp DESC LIMIT 100",
      )
      .toArray())
      .reverse();
    for (const row of history) {
      session.blockedMessages.push(JSON.stringify({
        name: row.name,
        message: row.message,
        timestamp: Number(row.timestamp),
      }));
    }

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
      await this.sql.exec(
        "INSERT INTO system.ratel_chat_messages (actor_id, timestamp, name, message) VALUES ($1, $2, $3, $4)",
        timestamp,
        session.name,
        message,
      ).toArray();
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
