// Package webauthn связывает go-webauthn с Valkey и пользователями UI.
package webauthn

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/go-webauthn/webauthn/protocol"
	lib "github.com/go-webauthn/webauthn/webauthn"
)

const challengeTTL = 5 * time.Minute

type Protocol interface {
	BeginRegistration(user lib.User) (*protocol.CredentialCreation, *lib.SessionData, error)
	FinishRegistration(user lib.User, session lib.SessionData, r *http.Request) (*lib.Credential, error)
	BeginLogin(user lib.User) (*protocol.CredentialAssertion, *lib.SessionData, error)
	FinishLogin(user lib.User, session lib.SessionData, r *http.Request) (*lib.Credential, error)
}

type Service struct {
	st    *store.Store
	proto Protocol
}

type registrationChallenge struct {
	Session lib.SessionData
	Name    string
}

type user struct {
	name     string
	passkeys []store.Passkey
}

func (u user) WebAuthnID() []byte {
	sum := sha256.Sum256([]byte(u.name))
	return sum[:16]
}
func (u user) WebAuthnName() string        { return u.name }
func (u user) WebAuthnDisplayName() string { return u.name }
func (u user) WebAuthnCredentials() []lib.Credential {
	out := make([]lib.Credential, 0, len(u.passkeys))
	for _, p := range u.passkeys {
		out = append(out, p.Credential)
	}
	return out
}

// New создаёт сервис с реальным go-webauthn.
func New(cfg *config.Config, st *store.Store) (*Service, error) {
	if cfg.BaseURL == nil {
		return nil, errors.New("не задан TGADMIN_BASE_URL")
	}
	if cfg.BaseURL.Scheme != "https" && !isLocal(cfg.BaseURL) {
		return nil, errors.New("WebAuthn требует HTTPS или localhost")
	}
	w, err := lib.New(&lib.Config{
		RPID:          cfg.BaseURL.Hostname(),
		RPDisplayName: "TeleMT Admin",
		RPOrigins:     []string{cfg.BaseURL.Scheme + "://" + cfg.BaseURL.Host},
	})
	if err != nil {
		return nil, err
	}
	return &Service{st: st, proto: &adapter{w: w}}, nil
}

func isLocal(u *url.URL) bool {
	return u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
}

type adapter struct{ w *lib.WebAuthn }

func (a *adapter) BeginRegistration(u lib.User) (*protocol.CredentialCreation, *lib.SessionData, error) {
	return a.w.BeginRegistration(u)
}
func (a *adapter) FinishRegistration(u lib.User, s lib.SessionData, r *http.Request) (*lib.Credential, error) {
	return a.w.FinishRegistration(u, s, r)
}
func (a *adapter) BeginLogin(u lib.User) (*protocol.CredentialAssertion, *lib.SessionData, error) {
	return a.w.BeginLogin(u)
}
func (a *adapter) FinishLogin(u lib.User, s lib.SessionData, r *http.Request) (*lib.Credential, error) {
	return a.w.FinishLogin(u, s, r)
}

func (s *Service) loadUser(ctx context.Context, name string) (user, error) {
	passkeys, err := s.st.ListPasskeys(ctx, name)
	if err != nil {
		return user{}, err
	}
	return user{name: name, passkeys: passkeys}, nil
}

// BeginRegistration создаёт options регистрации.
func (s *Service) BeginRegistration(ctx context.Context, userName, sessionID, name string) (any, error) {
	count, err := s.st.CountPasskeys(ctx, userName)
	if err != nil {
		return nil, err
	}
	if count >= store.MaxPasskeys {
		return nil, store.ErrLimit
	}
	u, err := s.loadUser(ctx, userName)
	if err != nil {
		return nil, err
	}
	options, session, err := s.proto.BeginRegistration(u)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(registrationChallenge{Session: *session, Name: name})
	if err != nil {
		return nil, err
	}
	if err := s.st.SaveWebAuthnChallenge(ctx, "register", sessionID, raw, challengeTTL); err != nil {
		return nil, err
	}
	return options, nil
}

// FinishRegistration проверяет attestation и сохраняет passkey.
func (s *Service) FinishRegistration(ctx context.Context, userName, sessionID string, r *http.Request) (*store.Passkey, error) {
	raw, err := s.st.TakeWebAuthnChallenge(ctx, "register", sessionID)
	if err != nil {
		return nil, err
	}
	var challenge registrationChallenge
	if err := json.Unmarshal(raw, &challenge); err != nil {
		return nil, err
	}
	u, err := s.loadUser(ctx, userName)
	if err != nil {
		return nil, err
	}
	credential, err := s.proto.FinishRegistration(u, challenge.Session, r)
	if err != nil {
		return nil, err
	}
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	p := &store.Passkey{
		ID: id, User: userName, Name: challenge.Name, Credential: *credential,
		CreatedAt: time.Now().UTC(), SignCount: credential.Authenticator.SignCount,
		BackupEligible: credential.Flags.BackupEligible, BackupState: credential.Flags.BackupState,
	}
	if err := s.st.CreatePasskey(ctx, *p); err != nil {
		return nil, err
	}
	return p, nil
}

// BeginLogin создаёт options входа.
func (s *Service) BeginLogin(ctx context.Context, userName, pendingToken string) (any, error) {
	u, err := s.loadUser(ctx, userName)
	if err != nil {
		return nil, err
	}
	if len(u.passkeys) == 0 {
		return nil, store.ErrNotFound
	}
	options, session, err := s.proto.BeginLogin(u)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	if err := s.st.SaveWebAuthnChallenge(ctx, "login", pendingToken, raw, challengeTTL); err != nil {
		return nil, err
	}
	return options, nil
}

// FinishLogin проверяет assertion и обновляет passkey.
func (s *Service) FinishLogin(ctx context.Context, userName, pendingToken string, r *http.Request) (*store.Passkey, error) {
	raw, err := s.st.TakeWebAuthnChallenge(ctx, "login", pendingToken)
	if err != nil {
		return nil, err
	}
	var session lib.SessionData
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, err
	}
	u, err := s.loadUser(ctx, userName)
	if err != nil {
		return nil, err
	}
	credential, err := s.proto.FinishLogin(u, session, r)
	if err != nil {
		return nil, err
	}
	for i := range u.passkeys {
		if string(u.passkeys[i].Credential.ID) == string(credential.ID) {
			p := u.passkeys[i]
			_ = s.st.UpdatePasskeyUsage(ctx, p.ID, credential.Authenticator.SignCount, time.Now().UTC())
			return &p, nil
		}
	}
	return nil, fmt.Errorf("credential не найден")
}

func randomID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// NewWithProtocol используется тестами.
func NewWithProtocol(st *store.Store, proto Protocol) *Service {
	return &Service{st: st, proto: proto}
}
