package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/alicebob/miniredis/v2"
)

func TestUserAddAndList(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := &config.Config{ValkeyURL: "redis://" + mr.Addr() + "/0"}
	var out bytes.Buffer
	in := strings.NewReader("long password 123\nlong password 123\n")
	if err := Run(context.Background(), []string{"user", "add", "admin", "--role", "admin"}, cfg, in, &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	st, _ := store.Open(context.Background(), cfg.ValkeyURL, "")
	defer func() { _ = st.Close() }()
	u, err := st.GetUser(context.Background(), "admin")
	if err != nil || u.Role != store.RoleAdmin {
		t.Fatalf("пользователь: %+v, %v", u, err)
	}
	out.Reset()
	if err := Run(context.Background(), []string{"user", "list"}, cfg, strings.NewReader(""), &out); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out.String(), "admin") {
		t.Fatalf("вывод: %s", out.String())
	}
}
