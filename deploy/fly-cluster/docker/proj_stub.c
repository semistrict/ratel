// Stub implementations for PROJ functions used by pkg/geo/geoproj/proj.cc.
// Ratel does not use geospatial features; these stubs satisfy the linker.
#include <stddef.h>
void *pj_ctx_alloc(void) { return NULL; }
void pj_ctx_free(void *ctx) {}
void *pj_init_plus_ctx(void *ctx, const char *def) { return NULL; }
int pj_is_latlong(void *pj) { return 0; }
void pj_get_spheroid_defn(void *pj, double *a, double *b) {}
void pj_free(void *pj) {}
int pj_ctx_get_errno(void *ctx) { return 0; }
const char *pj_strerrno(int err) { return "geo not supported"; }
int pj_transform(void *src, void *dst, long n, int offset, double *x, double *y, double *z) { return -1; }
