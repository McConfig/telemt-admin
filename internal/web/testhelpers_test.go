package web

import (
	"context"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/alicebob/miniredis/v2"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	mr := miniredis.RunT(t)
	st, err := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}
