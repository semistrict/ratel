export default {
  async fetch(request) {
    return new Response("hello from workerd");
  }
};
