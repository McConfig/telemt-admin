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

func cliStore(t *testing.T) (*config.Config, *store.Store) {
	t.Helper()
	mr := miniredis.RunT(t)
	cfg := &config.Config{ValkeyURL: "redis://" + mr.Addr() + "/0"}
	st, err := store.Open(context.Background(), cfg.ValkeyURL, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return cfg, st
}

func TestUserCommandsCoverage(t *testing.T) {
	cfg, st := cliStore(t)
	var out bytes.Buffer
	add := func() {
		in := strings.NewReader("long password 123\nlong password 123\n")
		if err := Run(context.Background(), []string{"user", "add", "alice"}, cfg, in, &out); err != nil {
			t.Fatal(err)
		}
	}
	add()

	if err := Run(context.Background(), []string{"user", "passwd", "alice"}, cfg,
		strings.NewReader("Another-long-password-1\nAnother-long-password-1\n"), &out); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"user", "disable", "alice"}, cfg, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if u, _ := st.GetUser(context.Background(), "alice"); !u.Disabled {
		t.Fatal("не отключён")
	}
	if err := Run(context.Background(), []string{"user", "enable", "alice"}, cfg, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if u, _ := st.GetUser(context.Background(), "alice"); u.Disabled {
		t.Fatal("не включён")
	}

	if err := st.EnableTOTP(context.Background(), "alice", "secret", 0, []string{"hash"}); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"user", "totp-reset", "alice"}, cfg, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if u, _ := st.GetUser(context.Background(), "alice"); u.TOTPEnabled {
		t.Fatal("2FA не сброшена")
	}
	out.Reset()
	if err := Run(context.Background(), []string{"user", "list"}, cfg, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "alice") {
		t.Fatalf("вывод списка: %s", out.String())
	}
}

func TestUserCommandErrors(t *testing.T) {
	cfg, _ := cliStore(t)
	var out bytes.Buffer
	cases := []struct {
		args []string
		in   string
	}{
		{[]string{"bad"}, ""},
		{[]string{"user", "unknown"}, ""},
		{[]string{"user", "add"}, ""},
		{[]string{"user", "add", "alice", "--role", "root"}, ""},
		{[]string{"user", "add", "alice", "--role"}, ""},
		{[]string{"user", "add", "alice"}, "short\nshort\n"},
		{[]string{"user", "add", "alice"}, "long password 123\nother password 123\n"},
		{[]string{"user", "passwd"}, ""},
		{[]string{"user", "passwd", "missing"}, "Another-long-password-1\nAnother-long-password-1\n"},
		{[]string{"user", "disable"}, ""},
		{[]string{"user", "totp-reset"}, ""},
	}
	for _, c := range cases {
		if err := Run(context.Background(), c.args, cfg, strings.NewReader(c.in), &out); err == nil {
			t.Errorf("%v: ждали ошибку", c.args)
		}
	}
}

func TestTOTPResetWithoutTOTP(t *testing.T) {
	cfg, st := cliStore(t)
	var out bytes.Buffer
	if err := st.CreateUser(context.Background(), store.User{Name: "bob", Hash: "h", Role: store.RoleViewer}); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"user", "totp-reset", "bob"}, cfg, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "не включена") {
		t.Fatalf("вывод: %s", out.String())
	}
}
