function invoke(x) {
  if (x < 0) throw new Error("negative input: " + x);
  return x * x;
}
