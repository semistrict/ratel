// Router worker for Ratel's workers platform. Dispatches incoming HTTP
// requests to the appropriate worker service by reading X-Worker-Name.
export default {
  async fetch(request, env) {
    const workerConfig = JSON.parse(env.__RATEL_WORKERS || "{}");
    const name = request.headers.get("X-Worker-Name");
    if (!name) {
      return new Response("missing X-Worker-Name header", {status: 400});
    }
    const worker = env[name];
    if (!worker) {
      return new Response("worker not found: " + name, {status: 404});
    }
    const config = workerConfig[name] || {};
    const assets = config.assets ? env[`${name}-assets`] : undefined;
    if (assets && !shouldRunWorkerFirst(config, new URL(request.url).pathname)) {
      const assetResponse = await assets.fetch(request);
      if (assetResponse.status !== 404 || assetResponse.headers.get("X-Ratel-Asset") === "hit") {
        const response = new Response(assetResponse.body, assetResponse);
        response.headers.delete("X-Ratel-Asset");
        return response;
      }
    }
    return worker.fetch(request);
  }
};

function shouldRunWorkerFirst(config, path) {
  if (config.run_worker_first) return true;
  const routes = config.run_worker_first_routes || [];
  let matched = false;
  for (const route of routes) {
    const negated = route.startsWith("!");
    const pattern = negated ? route.slice(1) : route;
    if (globMatches(pattern, path)) {
      if (negated) return false;
      matched = true;
    }
  }
  return matched;
}

function globMatches(pattern, path) {
  const escaped = pattern
    .replace(/[.+?^${}()|[\]\\]/g, "\\$&")
    .replace(/\*/g, ".*");
  return new RegExp(`^${escaped}$`).test(path);
}
