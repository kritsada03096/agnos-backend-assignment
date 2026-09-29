package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agnos-backend-assignment/internal/model"
)

type Hospital struct{ http *http.Client }

func New(timeout time.Duration) *Hospital {
	return &Hospital{&http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (h *Hospital) Search(ctx context.Context, baseURL, id string) (model.Patient, error) {
	var p model.Patient
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return p, model.ErrUpstream
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/patient/search/" + url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return p, model.ErrUpstream
	}
	req.Header.Set("Accept", "application/json")
	resp, err := h.http.Do(req)
	if err != nil {
		var ne net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
			return p, model.ErrTimeout
		}
		return p, model.ErrUpstream
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return p, model.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return p, model.ErrUpstream
	}
	const maxBody = 1 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil || len(body) > maxBody || json.Unmarshal(body, &p) != nil || strings.TrimSpace(p.PatientHN) == "" || len(p.PatientHN) > 100 {
		return p, model.ErrUpstream
	}
	for _, value := range []*string{p.NationalID, p.PassportID, p.FirstNameTH, p.MiddleNameTH, p.LastNameTH, p.FirstNameEN, p.MiddleNameEN, p.LastNameEN, p.PhoneNumber, p.Email} {
		if value != nil && len(*value) > 254 {
			return p, model.ErrUpstream
		}
	}
	if p.DateOfBirth != nil {
		if _, err := time.Parse("2006-01-02", *p.DateOfBirth); err != nil {
			return p, model.ErrUpstream
		}
	}
	if p.Gender != nil && *p.Gender != "M" && *p.Gender != "F" {
		return p, model.ErrUpstream
	}
	return p, nil
}
