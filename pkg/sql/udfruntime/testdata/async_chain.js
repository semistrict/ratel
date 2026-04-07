// Returns a Promise that chains through multiple .then() calls,
// requiring multiple microtask pumps to resolve.
function invoke(x) {
  return Promise.resolve(x)
    .then(function(v) { return v + 1; })
    .then(function(v) { return v * 2; })
    .then(function(v) { return v - 1; });
}
