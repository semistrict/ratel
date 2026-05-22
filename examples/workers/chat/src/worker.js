import { handleErrors } from "./errors.js";
import { ChatRoom } from "./room.js";

export default {
  async fetch(request, env) {
    return handleErrors(request, async () => {
      const url = new URL(request.url);
      const path = url.pathname.split("/").filter(Boolean);

      if (!path[0]) {
        return env.ASSETS.fetch(new URL("/index.html", request.url));
      }

      if (path[0] === "api" && path[1] === "room" && path[2]) {
        const roomName = path[2];
        if (roomName.length > 64) return new Response("room name too long", { status: 400 });

        const id = env.ChatRoom.idFromName(roomName);
        const room = env.ChatRoom.get(id);
        const nextURL = new URL(request.url);
        nextURL.pathname = "/" + path.slice(3).join("/");
        return room.fetch(nextURL, request);
      }

      return env.ASSETS.fetch(request);
    });
  },
};

export { ChatRoom };
