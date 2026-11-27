package webauthn

import (
	"context"
	"net/http"
	"testing"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-webauthn/webauthn/protocol"
	lib "github.com/go-webauthn/webauthn/webauthn"
)

type fakeProtocol struct{}

func (fakeProtocol) BeginRegistration(lib.User) (*protocol.CredentialCreation, *lib.SessionData, error) {
	return &protocol.CredentialCreation{}, &lib.SessionData{Challenge: "c"}, nil
}
func (fakeProtocol) FinishRegistration(lib.User, lib.SessionData, *http.Request) (*lib.Credential, error) {
	return &lib.Credential{ID: []byte{1, 2, 3}}, nil
}
func (fakeProtocol) BeginLogin(lib.User) (*protocol.CredentialAssertion, *lib.SessionData, error) {
	return &protocol.CredentialAssertion{}, &lib.SessionData{Challenge: "c"}, nil
}
func (fakeProtocol) FinishLogin(lib.User, lib.SessionData, *http.Request) (*lib.Credential, error) {
	return &lib.Credential{ID: []byte{1, 2, 3}}, nil
}

func newService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	mr := miniredis.RunT(t)
	st, err := store.Open(context.Background(), "redis://"+mr.Addr()+"/0", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewWithProtocol(st, fakeProtocol{}), st
}

func TestRegisterAndLogin(t *testing.T) {
	svc, st := newService(t)
	ctx := context.Background()
	if _, err := svc.BeginRegistration(ctx, "admin", "sess", "MacBook"); err != nil {
		t.Fatal(err)
	}
	p, err := svc.FinishRegistration(ctx, "admin", "sess", nil)
	if err != nil || p.Name != "MacBook" {
		t.Fatalf("%+v %v", p, err)
	}
	if n, _ := st.CountPasskeys(ctx, "admin"); n != 1 {
		t.Fatalf("count=%d", n)
	}
	if _, err := svc.FinishRegistration(ctx, "admin", "sess", nil); err == nil {
		t.Fatal("challenge должен быть одноразовым")
	}
	if _, err := svc.BeginLogin(ctx, "admin", "pending"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.FinishLogin(ctx, "admin", "pending", nil)
	if err != nil || got.ID != p.ID {
		t.Fatalf("%+v %v", got, err)
	}
}
