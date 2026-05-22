export function errorResponse(err, request) {
  if (request.headers.get("Upgrade") === "websocket") {
    const pair = new WebSocketPair();
    pair[1].accept();
    pair[1].send(JSON.stringify({ error: err.stack || String(err) }));
    pair[1].close(1011, "Uncaught exception during session setup");
    return new Response(null, { status: 101, webSocket: pair[0] });
  }
  return new Response(err.stack || String(err), { status: 500 });
}

export async function handleErrors(request, fn) {
  try {
    return await fn();
  } catch (err) {
    return errorResponse(err, request);
  }
}
