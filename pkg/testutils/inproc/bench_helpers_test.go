package inproc_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/testutils/inproc"
)

func benchCluster(b *testing.B) (*inproc.Cluster, context.Context) {
	b.Helper()
	c := inproc.StartCluster(b, 1)
	b.Cleanup(func() { c.Stop() })
	return c, context.Background()
}
