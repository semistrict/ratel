export default {
  async fetch(request) {
    const url = new URL(request.url);
    return new Response("echo: " + url.pathname);
  }
};
