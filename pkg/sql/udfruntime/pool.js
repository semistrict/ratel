// Copyright 2026 The Ratel Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// __pool manages in-flight UDF invocations with flow control.
// Go submits work via __pool.submit(jsonArgs) and collects results
// via __pool.collect(). Sync results go directly to completed;
// async results (Promises) are tracked in pending until resolved.
const __pool = {
  pending: [],
  completed: [],
  nextIdx: 0,
  // Indices of args that need new Date() hydration (timestamps).
  dateArgs: [],

  // submit(argsJSON) — invoke() each row, track sync vs async results.
  // argsJSON is a JSON string: [[arg0_0, arg0_1, ...], [arg1_0, ...], ...]
  submit(argsJSON) {
    const argsList = JSON.parse(argsJSON);
    const da = this.dateArgs;
    for (let i = 0; i < argsList.length; i++) {
      const a = argsList[i];
      // Hydrate Date args from milliseconds.
      for (let d = 0; d < da.length; d++) {
        const k = da[d];
        if (a[k] !== null) a[k] = new Date(a[k]);
      }
      const idx = this.nextIdx++;
      let r;
      try {
        r = invoke(...a);
      } catch (e) {
        const msg = (e instanceof Error && e.stack) ? e.stack : String(e);
        this.completed.push({ idx: idx, value: null, error: msg });
        continue;
      }
      if (r instanceof Promise) {
        const entry = { idx: idx, resolved: false, value: undefined, error: undefined };
        r.then(
          function(v) { entry.value = v; entry.resolved = true; },
          function(e) {
            entry.error = (e instanceof Error && e.stack) ? e.stack : String(e);
            entry.resolved = true;
          }
        );
        this.pending.push(entry);
      } else {
        this.completed.push({ idx: idx, value: r });
      }
    }
  },

  // collect() — return resolved results as JSON: [[idx, value], ...] or
  // [[idx, null, "error msg"], ...] for errors. Returns null if nothing ready.
  collect() {
    const stillPending = [];
    for (let i = 0; i < this.pending.length; i++) {
      const e = this.pending[i];
      if (e.resolved) {
        this.completed.push(e);
      } else {
        stillPending.push(e);
      }
    }
    this.pending = stillPending;

    if (this.completed.length === 0) return null;

    const batch = this.completed;
    this.completed = [];
    const out = new Array(batch.length);
    for (let i = 0; i < batch.length; i++) {
      const e = batch[i];
      if (e.error !== undefined) {
        out[i] = [e.idx, null, e.error];
      } else {
        out[i] = [e.idx, e.value];
      }
    }
    return JSON.stringify(out);
  },

  inflight() { return this.pending.length; },

  // reset() — clear all state for reuse across calls.
  reset() {
    this.pending = [];
    this.completed = [];
    this.nextIdx = 0;
  }
};

// Top-level wrappers so Go can call via Global().MethodCall().
function __pool_submit(s) { __pool.submit(s); }
function __pool_collect() { return __pool.collect(); }
function __pool_inflight() { return __pool.inflight(); }
function __pool_reset() { __pool.reset(); }
