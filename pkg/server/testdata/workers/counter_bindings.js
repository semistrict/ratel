export class Counter extends DurableObject {
  constructor(state) {
    this.state = state;
  }
}

export default {
  async fetch() {
    return new Response("ok");
  }
};
