// Router worker for Ratel's workers platform. Dispatches incoming HTTP
// requests to the appropriate worker service by reading X-Worker-Name.
export default {
  async fetch(request, env) {
    const name = request.headers.get("X-Worker-Name");
    if (!name) {
      return new Response("missing X-Worker-Name header", {status: 400});
    }
    const worker = env[name];
    if (!worker) {
      return new Response("worker not found: " + name, {status: 404});
    }
    return worker.fetch(request);
  }
};
