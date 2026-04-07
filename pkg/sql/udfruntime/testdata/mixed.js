function invoke(x) {
  return x > 5 ? Promise.resolve(x * 10) : x * 2;
}
