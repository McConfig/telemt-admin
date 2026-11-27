package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/configx"
	"github.com/McConfig/telemt-admin/internal/store"
	"github.com/McConfig/telemt-admin/internal/version"
	"github.com/go-chi/chi/v5"
)

const maxConfigFile = 4 << 20

var (
	errSecretsRequired = errors.New("secrets_confirmation_required")
	errBadCredentials  = errors.New("bad_credentials")
)

// requireConfigAdmin разрешает только администратора с cookie-сессией.
func (s *Server) requireConfigAdmin(w http.ResponseWriter, r *http.Request) *store.Session {
	sess := s.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusForbidden, "session_required", "доступно только из интерфейса")
		return nil
	}
	if !s.isAdmin(r) {
		writeError(w, http.StatusForbidden, "forbidden", "нет прав")
		return nil
	}
	return sess
}

type configBlockView struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

var configBlockTitles = map[configx.Block]string{
	configx.BlockSettings:  "Системные настройки",
	configx.BlockTemplates: "Шаблоны лимитов",
	configx.BlockProxy:     "Конфигурация прокси",
	configx.BlockUsers:     "Пользователи UI",
	configx.BlockServices:  "Сервисные аккаунты",
	configx.BlockClients:   "Клиенты telemt",
}

func (s *Server) apiConfigBlocks(w http.ResponseWriter, r *http.Request) {
	if s.requireConfigAdmin(w, r) == nil {
		return
	}
	out := make([]configBlockView, 0, len(configBlockTitles))
	for _, id := range []configx.Block{
		configx.BlockSettings, configx.BlockTemplates, configx.BlockProxy,
		configx.BlockUsers, configx.BlockServices, configx.BlockClients,
	} {
		out = append(out, configBlockView{ID: string(id), Title: configBlockTitles[id]})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) confirmSecrets(ctx context.Context, user *store.User, password, code string) error {
	ok, err := auth.Verify(user.Hash, password)
	if err != nil || !ok {
		return errBadCredentials
	}
	if !user.TOTPEnabled {
		return nil
	}
	ok, counter, recovery := s.checkTOTPCode(ctx, user, code)
	if !ok {
		return errBadCredentials
	}
	if !recovery && counter > user.TOTPLastCounter {
		_ = s.store.SetTOTPLastCounter(ctx, user.Name, counter)
	}
	return nil
}

type exportRequest struct {
	Blocks   []string `json:"blocks"`
	Secrets  bool     `json:"secrets"`
	Password string   `json:"password"`
	Code     string   `json:"code"`
	Name     string   `json:"name"`
}

func (s *Server) apiConfigExport(w http.ResponseWriter, r *http.Request) {
	sess := s.requireConfigAdmin(w, r)
	if sess == nil {
		return
	}
	if !s.allowConfigRate("export", sess.User, 10, time.Minute) {
		writeError(w, http.StatusTooManyRequests, "too_many", "слишком много экспортов, подождите минуту")
		return
	}
	var in exportRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	u, err := s.store.GetUser(r.Context(), sess.User)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if in.Secrets {
		if err := s.confirmSecrets(r.Context(), u, in.Password, in.Code); err != nil {
			writeError(w, http.StatusForbidden, "bad_credentials", "неверный пароль или код")
			return
		}
	}
	blocks := make([]configx.Block, 0, len(in.Blocks))
	for _, raw := range in.Blocks {
		block := configx.Block(raw)
		if _, ok := configBlockTitles[block]; !ok {
			writeError(w, http.StatusBadRequest, "bad_request", "неизвестный блок: "+raw)
			return
		}
		blocks = append(blocks, block)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = s.cfg.BaseURL.Hostname()
	}
	bundle, err := s.configx.Export(r.Context(), configx.ExportOptions{
		Blocks: blocks, Secrets: in.Secrets, Name: name,
		AppVersion: version.Version, BaseURL: s.cfg.BaseURL.String(),
	})
	if err != nil {
		s.writeConfigError(w, r, err, in.Secrets)
		return
	}
	raw, err := configx.Marshal(bundle)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	filename := "tgadmin-" + s.now().UTC().Format("20060102-1504") + ".tgadmin.yaml"
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(raw); err != nil {
		return
	}
	action := "config.export"
	if in.Secrets {
		action = "config.export.secrets"
	}
	s.audit(r, action, strings.Join(in.Blocks, ","), "ok")
}

func (s *Server) apiConfigPrecheck(w http.ResponseWriter, r *http.Request) {
	sess := s.requireConfigAdmin(w, r)
	if sess == nil {
		return
	}
	if !s.allowConfigRate("import", sess.User, 5, time.Minute) {
		writeError(w, http.StatusTooManyRequests, "too_many", "слишком много импортов, подождите минуту")
		return
	}
	var in struct {
		YAML     string `json:"yaml"`
		FileName string `json:"fileName"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	limited := http.MaxBytesReader(w, r.Body, maxConfigFile+64<<10)
	if err := json.NewDecoder(limited).Decode(&in); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "файл больше 4 МиБ или неверный JSON")
		return
	}
	raw := []byte(in.YAML)
	if len(raw) > maxConfigFile {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "файл больше 4 МиБ")
		return
	}
	bundle, err := configx.Decode(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_yaml", err.Error())
		return
	}
	if bundle.Metadata.Secrets == configx.SecretsIncluded {
		if in.Password == "" {
			writeError(w, http.StatusForbidden, "secrets_confirmation_required", "нужен пароль и код для файла с секретами")
			return
		}
		u, err := s.store.GetUser(r.Context(), sess.User)
		if err != nil {
			s.serverErrorJSON(w, r, err)
			return
		}
		if err := s.confirmSecrets(r.Context(), u, in.Password, in.Code); err != nil {
			writeError(w, http.StatusForbidden, "bad_credentials", "неверный пароль или код")
			return
		}
	}
	fileName := in.FileName
	if fileName == "" {
		fileName = "uploaded.tgadmin.yaml"
	}
	sum := sha256.Sum256(raw)
	plan, err := s.configx.Precheck(r.Context(), configx.PrecheckOptions{
		FileName: fileName, FileHash: hex.EncodeToString(sum[:]),
		Bundle: bundle, CurrentUser: sess.User,
		TargetSettingsVersion: s.settings.Version(r.Context()),
	})
	if err != nil {
		s.writeConfigError(w, r, err, false)
		return
	}
	id, err := randomID()
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	plan.ID, plan.Owner, plan.Session = id, sess.User, sess.ID
	plan.CreatedAt = s.now().UTC()
	plan.ExpiresAt = plan.CreatedAt.Add(15 * time.Minute)
	payload, err := json.Marshal(plan)
	if err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	if err := s.store.PutConfigPlan(r.Context(), store.ConfigPlan{
		ID: id, Owner: sess.User, Session: sess.ID, Payload: payload, CreatedAt: plan.CreatedAt,
	}, 15*time.Minute); err != nil {
		s.serverErrorJSON(w, r, err)
		return
	}
	s.audit(r, "config.import.precheck", strings.Join(blockIDs(bundle), ","), "ok")
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) apiConfigPlan(w http.ResponseWriter, r *http.Request) {
	sess := s.requireConfigAdmin(w, r)
	if sess == nil {
		return
	}
	id := chi.URLParam(r, "id")
	p, err := s.store.GetConfigPlan(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "plan_not_found", "план истёк или не найден")
		return
	}
	if p.Owner != sess.User || p.Session != sess.ID {
		writeError(w, http.StatusNotFound, "plan_not_found", "план истёк или не найден")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(p.Payload)
}

type applyRequest struct {
	PlanID        string                       `json:"planId"`
	Blocks        []string                     `json:"blocks"`
	Modes         map[string]string            `json:"modes"`
	Conflicts     map[string]map[string]string `json:"conflicts"`
	DeleteMissing bool                         `json:"deleteMissing"`
	ConfirmName   string                       `json:"confirmName"`
}

func (s *Server) apiConfigApply(w http.ResponseWriter, r *http.Request) {
	sess := s.requireConfigAdmin(w, r)
	if sess == nil {
		return
	}
	if !s.allowConfigRate("import", sess.User, 5, time.Minute) {
		writeError(w, http.StatusTooManyRequests, "too_many", "слишком много импортов, подождите минуту")
		return
	}
	var in applyRequest
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "неверный JSON")
		return
	}
	p, err := s.store.TakeConfigPlan(r.Context(), in.PlanID)
	if err != nil {
		writeError(w, http.StatusNotFound, "plan_not_found", "план истёк или не найден")
		return
	}
	if p.Owner != sess.User || p.Session != sess.ID {
		writeError(w, http.StatusNotFound, "plan_not_found", "план истёк или не найден")
		return
	}
	var plan configx.Plan
	if err := json.Unmarshal(p.Payload, &plan); err != nil {
		writeError(w, http.StatusBadRequest, "plan_not_found", "план повреждён")
		return
	}
	blocks := make([]configx.Block, 0, len(in.Blocks))
	for _, raw := range in.Blocks {
		blocks = append(blocks, configx.Block(raw))
	}
	modes := map[configx.Block]string{}
	for key, value := range in.Modes {
		modes[configx.Block(key)] = value
	}
	results, err := s.configx.Apply(r.Context(), &plan, configx.ApplyOptions{
		Blocks: blocks, Modes: modes, Conflicts: in.Conflicts,
		DeleteMissing: in.DeleteMissing, ConfirmName: in.ConfirmName,
	})
	if err != nil && len(results) == 0 {
		s.writeConfigError(w, r, err, false)
		return
	}
	result := "ok"
	if err != nil {
		result = "error"
	}
	s.audit(r, "config.import.apply", strings.Join(in.Blocks, ","), result)
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "error": errorText(err)})
}

func (s *Server) writeConfigError(w http.ResponseWriter, r *http.Request, err error, secrets bool) {
	switch {
	case errors.Is(err, configx.ErrStalePlan):
		writeError(w, http.StatusConflict, "stale_plan", "настройки изменились, повторите предпросмотр")
	case errors.Is(err, errSecretsRequired):
		writeError(w, http.StatusForbidden, "secrets_confirmation_required", "нужен пароль и код")
	case errors.Is(err, errBadCredentials):
		writeError(w, http.StatusForbidden, "bad_credentials", "неверный пароль или код")
	default:
		msg := err.Error()
		code := "bad_request"
		if strings.HasPrefix(msg, "bad_yaml:") {
			code = "bad_yaml"
		} else if strings.HasPrefix(msg, "unsupported_version:") {
			code = "unsupported_version"
		} else if strings.HasPrefix(msg, "proxy_unavailable:") {
			code = "proxy_unavailable"
			msg = "Control API telemt недоступен"
		}
		if secrets {
			msg = "не удалось проверить файл"
		}
		writeError(w, http.StatusBadRequest, code, msg)
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func blockIDs(b *configx.Bundle) []string {
	out := []string{}
	for id := range configBlockTitles {
		if b.HasBlock(id) {
			out = append(out, string(id))
		}
	}
	return out
}

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
