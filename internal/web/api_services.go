package web

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/go-chi/chi/v5"
)

var serviceNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,31}$`)
var serviceTokenIDRe = regexp.MustCompile(`^[0-9a-f]{16}$`)

type serviceView struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
	Disabled    bool   `json:"disabled"`
	Tokens      int    `json:"tokens"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	CreatedBy   string `json:"createdBy"`
}

type tokenView struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Prefix     string `json:"prefix"`
	CreatedAt  string `json:"createdAt"`
	CreatedBy  string `json:"createdBy"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
}

type issuedTokenView struct {
	ID        string `json:"id"`
	Token     string `json:"token"`
	Label     string `json:"label"`
	Prefix    string `json:"prefix"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

type serviceCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
}

type serviceUpdateRequest struct {
	Description *string `json:"description"`
	Scope       *string `json:"scope"`
	Disabled    *bool   `json:"disabled"`
}

type tokenIssueRequest struct {
	Label         string `json:"label"`
	ExpiresInDays *int   `json:"expiresInDays"`
}

func validScope(scope string) bool {
	switch store.ServiceScope(scope) {
	case store.ScopeRead, store.ScopeWrite, store.ScopeAdmin:
		return true
	default:
		return false
	}
}

func (s *Server) apiServices(w http.ResponseWriter, r *http.Request) {
	if !s.requireSessionAdmin(w, r) {
		return
	}
	services, err := s.store.ListServices(r.Context())
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	names := make([]string, 0, len(services))
	for _, svc := range services {
		names = append(names, svc.Name)
	}
	counts, err := s.store.CountServiceTokensMany(r.Context(), names)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	out := make([]serviceView, 0, len(services))
	for _, svc := range services {
		out = append(out, makeServiceView(svc, counts[svc.Name]))
	}
	writeJSON(w, http.StatusOK, out)
}

func makeServiceView(svc store.ServiceAccount, tokens int) serviceView {
	return serviceView{
		Name: svc.Name, Description: svc.Description, Scope: string(svc.Scope),
		Disabled: svc.Disabled, Tokens: tokens,
		CreatedAt: svc.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: svc.UpdatedAt.UTC().Format(time.RFC3339),
		CreatedBy: svc.CreatedBy,
	}
}

func (s *Server) apiCreateService(w http.ResponseWriter, r *http.Request) {
	if !s.requireSessionAdmin(w, r) {
		return
	}
	var in serviceCreateRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if !serviceNameRe.MatchString(in.Name) {
		writeError(w, http.StatusBadRequest, "bad_request",
			"имя: строчная латиница, цифры, точка, дефис, подчёркивание, до 32 символов")
		return
	}
	if utf8.RuneCountInString(in.Description) > 200 {
		writeError(w, http.StatusBadRequest, "bad_request", "описание: не больше 200 символов")
		return
	}
	if !validScope(in.Scope) {
		writeError(w, http.StatusBadRequest, "bad_request", "права: read, write или admin")
		return
	}
	svc := store.ServiceAccount{
		Name: in.Name, Description: in.Description,
		Scope: store.ServiceScope(in.Scope), CreatedBy: s.actor(r),
	}
	if err := s.store.CreateService(r.Context(), svc); err != nil {
		s.serviceStoreError(w, r, err)
		return
	}
	s.audit(r, "service.create", in.Name, in.Scope)
	created, err := s.store.GetService(r.Context(), in.Name)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, makeServiceView(*created, 0))
}

func (s *Server) apiUpdateService(w http.ResponseWriter, r *http.Request) {
	if !s.requireSessionAdmin(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !serviceNameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	var in serviceUpdateRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	if in.Description == nil && in.Scope == nil && in.Disabled == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "нет изменений")
		return
	}
	upd := store.ServiceUpdate{Scope: nil, Disabled: in.Disabled}
	if in.Description != nil {
		desc := strings.TrimSpace(*in.Description)
		if utf8.RuneCountInString(desc) > 200 {
			writeError(w, http.StatusBadRequest, "bad_request", "описание: не больше 200 символов")
			return
		}
		upd.Description = &desc
	}
	if in.Scope != nil {
		if !validScope(*in.Scope) {
			writeError(w, http.StatusBadRequest, "bad_request", "права: read, write или admin")
			return
		}
		scope := store.ServiceScope(*in.Scope)
		upd.Scope = &scope
	}
	before, err := s.store.GetService(r.Context(), name)
	if err != nil {
		s.serviceStoreError(w, r, err)
		return
	}
	updated, err := s.store.UpdateService(r.Context(), name, upd)
	if err != nil {
		s.serviceStoreError(w, r, err)
		return
	}
	count, err := s.store.CountServiceTokens(r.Context(), name)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "service.update", name, serviceUpdateResult(*before, *updated))
	writeJSON(w, http.StatusOK, makeServiceView(*updated, count))
}

func serviceUpdateResult(before, after store.ServiceAccount) string {
	return "scope=" + string(before.Scope) + "->" + string(after.Scope) +
		" disabled=" + strconv.FormatBool(before.Disabled) + "->" + strconv.FormatBool(after.Disabled)
}

func (s *Server) apiDeleteService(w http.ResponseWriter, r *http.Request) {
	if !s.requireSessionAdmin(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !serviceNameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	var in struct {
		Confirm string `json:"confirm"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	if in.Confirm != name {
		writeError(w, http.StatusBadRequest, "bad_request", "подтвердите имя сервиса")
		return
	}
	if err := s.store.DeleteService(r.Context(), name); err != nil {
		s.serviceStoreError(w, r, err)
		return
	}
	s.audit(r, "service.delete", name, "ok")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiServiceTokens(w http.ResponseWriter, r *http.Request) {
	if !s.requireSessionAdmin(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !serviceNameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	tokens, err := s.store.ListServiceTokens(r.Context(), name)
	if err != nil {
		s.serviceStoreError(w, r, err)
		return
	}
	out := make([]tokenView, 0, len(tokens))
	for _, tok := range tokens {
		out = append(out, makeTokenView(tok))
	}
	writeJSON(w, http.StatusOK, out)
}

func makeTokenView(tok store.ServiceToken) tokenView {
	view := tokenView{
		ID: tok.ID, Label: tok.Label, Prefix: "tgadm_" + tok.ID,
		CreatedAt: tok.CreatedAt.UTC().Format(time.RFC3339),
		CreatedBy: tok.CreatedBy,
	}
	if tok.ExpiresAt != nil {
		view.ExpiresAt = tok.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if tok.LastUsedAt != nil {
		view.LastUsedAt = tok.LastUsedAt.UTC().Format(time.RFC3339)
	}
	return view
}

func (s *Server) apiIssueServiceToken(w http.ResponseWriter, r *http.Request) {
	if !s.requireSessionAdmin(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	if !serviceNameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	var in tokenIssueRequest
	if err := decodeJSON(r, &in); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	in.Label = strings.TrimSpace(in.Label)
	if utf8.RuneCountInString(in.Label) > 64 {
		writeError(w, http.StatusBadRequest, "bad_request", "метка: не больше 64 символов")
		return
	}
	days := 90
	if in.ExpiresInDays != nil {
		days = *in.ExpiresInDays
	}
	if days < 0 || days > 3650 {
		writeError(w, http.StatusBadRequest, "bad_request", "срок: 0 или от 1 до 3650 дней")
		return
	}
	var expiresAt *time.Time
	if days > 0 {
		exp := time.Now().UTC().AddDate(0, 0, days)
		expiresAt = &exp
	}
	full, tok, err := s.store.CreateServiceToken(
		r.Context(), name, in.Label, s.actor(r), expiresAt,
	)
	if err != nil {
		s.serviceStoreError(w, r, err)
		return
	}
	s.audit(r, "token.create", name+"/"+tok.ID, tokenCreateResult(tok))
	view := issuedTokenView{
		ID: tok.ID, Token: full, Label: tok.Label, Prefix: "tgadm_" + tok.ID,
		CreatedAt: tok.CreatedAt.UTC().Format(time.RFC3339),
	}
	if tok.ExpiresAt != nil {
		view.ExpiresAt = tok.ExpiresAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusCreated, view)
}

func tokenCreateResult(tok store.ServiceToken) string {
	result := "label=" + tok.Label
	if tok.ExpiresAt != nil {
		result += " expires=" + tok.ExpiresAt.UTC().Format(time.RFC3339)
	} else {
		result += " expires=never"
	}
	return result
}

func (s *Server) apiRevokeServiceToken(w http.ResponseWriter, r *http.Request) {
	if !s.requireSessionAdmin(w, r) {
		return
	}
	name := chi.URLParam(r, "name")
	id := chi.URLParam(r, "id")
	if !serviceNameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "недопустимое имя")
		return
	}
	if !serviceTokenIDRe.MatchString(id) {
		writeError(w, http.StatusNotFound, "not_found", "токен не найден")
		return
	}
	if err := s.store.DeleteServiceToken(r.Context(), name, id); err != nil {
		s.serviceStoreError(w, r, err)
		return
	}
	s.audit(r, "token.revoke", name+"/"+id, "ok")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) serviceStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrExists):
		writeError(w, http.StatusConflict, "conflict", "сервис уже существует")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "сервис или токен не найден")
	case errors.Is(err, store.ErrLimit):
		writeError(w, http.StatusTooManyRequests, "too_many", "превышен лимит")
	default:
		s.serverErrorJSON(w, r, err)
	}
}
