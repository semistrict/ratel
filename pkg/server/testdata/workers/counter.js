export class Counter {
  constructor(state, env) {
    this.state = state;
  }

  async fetch(request) {
    let val = await this.state.storage.get("count") || 0;
    val++;
    await this.state.storage.put("count", val);
    return new Response(String(val));
  }
}

export default {
  async fetch(request, env) {
    const id = env.Counter.idFromName("singleton");
    const stub = env.Counter.get(id);
    return stub.fetch(request);
  }
};
