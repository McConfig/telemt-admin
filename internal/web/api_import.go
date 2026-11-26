package web

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/McConfig/telemt-admin/internal/telemt"
	"github.com/McConfig/telemt-admin/internal/userimport"
)

type importRowView struct {
	Line    int    `json:"line"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type importResponse struct {
	DryRun bool            `json:"dryRun"`
	Total  int             `json:"total"`
	Create int             `json:"create"`
	Skip   int             `json:"skip"`
	Error  int             `json:"error"`
	Rows   []importRowView `json:"rows"`
}

func (s *Server) apiImportUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAPI(w, r) {
		return
	}
	dryRun := r.URL.Query().Get("dryRun") != "0"
	body := r.Body
	if body == nil {
		body = http.NoBody
	}
	defer func() { _ = body.Close() }()
	items, err := userimport.Parse(http.MaxBytesReader(w, body, 1<<20))
	if err != nil {
		message := err.Error()
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			message = "файл больше 1 МБ"
		}
		writeError(w, http.StatusBadRequest, "bad_request", message)
		return
	}
	users, err := s.telemt.ListUsers(r.Context())
	if err != nil {
		s.telemtAPIError(w, r, err)
		return
	}
	existing := make(map[string]bool, len(users))
	for _, u := range users {
		existing[u.Username] = true
	}

	out := importResponse{DryRun: dryRun, Rows: make([]importRowView, 0, len(items))}
	for _, item := range items {
		row := importRowView{Line: item.Line, Name: item.Name}
		switch {
		case item.Err != "":
			row.Status = "error"
			row.Message = item.Err
			out.Error++
		case existing[item.Name]:
			row.Status = "skip"
			row.Message = "клиент уже существует"
			out.Skip++
		case dryRun:
			row.Status = "create"
			out.Create++
		default:
			if err := s.createImportedUser(r, item.Row); err != nil {
				row.Status = "error"
				row.Message = telemtErrorMessage(err)
				out.Error++
			} else {
				row.Status = "create"
				out.Create++
				s.audit(r, "user.create", item.Name, "ok")
			}
		}
		out.Rows = append(out.Rows, row)
	}
	out.Total = len(out.Rows)
	if !dryRun {
		s.audit(r, "user.import", "csv", fmt.Sprintf(
			"create=%d skip=%d error=%d", out.Create, out.Skip, out.Error,
		))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createImportedUser(r *http.Request, row userimport.Row) error {
	_, err := s.telemt.CreateUser(r.Context(), telemt.CreateInput{
		Username: row.Name, Secret: row.Secret,
		MaxUniqueIPs: row.MaxUniqueIPs, MaxTCPConns: row.MaxTCPConns,
		DataQuotaBytes: row.DataQuotaBytes, QuotaPeriod: row.QuotaPeriod,
		RateLimitUpBps: row.RateLimitUpBps, RateLimitDownBps: row.RateLimitDownBps,
		ExpirationRFC3339: row.ExpirationRFC3339, Enabled: row.Enabled,
	})
	return err
}

func telemtErrorMessage(err error) string {
	var apiErr *telemt.APIError
	if errors.As(err, &apiErr) && apiErr.Message != "" {
		return apiErr.Message
	}
	return err.Error()
}
