import { handleErrors } from "./errors.js";
import { ChatRoom } from "./room.js";

export default {
  async fetch(request, env) {
    return handleErrors(request, async () => {
      const url = new URL(request.url);
      const path = url.pathname.split("/").filter(Boolean);

      if (path[0] === "api" && path[1] === "room" && path[2]) {
        const roomName = path[2];
        if (roomName.length > 64) return new Response("room name too long", { status: 400 });

        const id = env.ChatRoom.idFromName(roomName);
        const room = env.ChatRoom.get(id);
        const nextURL = new URL(request.url);
        nextURL.pathname = "/" + path.slice(3).join("/");
        const headers = new Headers(request.headers);
        headers.set("X-Ratel-Actor-Scope", roomName);
        return room.fetch(new Request(nextURL, request), { headers });
      }

      return new Response("not found", { status: 404 });
    });
  },
};

export { ChatRoom };
