package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agnos-backend-assignment/internal/model"
)

func TestHospitalSearch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"success", 200, `{"patient_hn":"A001","national_id":"111","date_of_birth":"1990-01-01","gender":"F"}`, nil},
		{"not found", 404, ``, model.ErrNotFound}, {"failure", 500, `private upstream details`, model.ErrUpstream},
		{"bad JSON", 200, `{`, model.ErrUpstream}, {"empty object", 200, `{}`, model.ErrUpstream},
		{"null", 200, `null`, model.ErrUpstream}, {"wrong date", 200, `{"patient_hn":"A","date_of_birth":"not-a-date"}`, model.ErrUpstream},
		{"wrong gender", 200, `{"patient_hn":"A","gender":"X"}`, model.ErrUpstream},
		{"too large", 200, strings.Repeat("x", (1<<20)+1), model.ErrUpstream},
		{"trailing JSON", 200, `{"patient_hn":"A"}{}`, model.ErrUpstream},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/patient/search/111" || r.Header.Get("Accept") != "application/json" {
					t.Error("incorrect request")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			p, err := New(time.Second).Search(context.Background(), server.URL, "111")
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if err == nil && p.PatientHN != "A001" {
				t.Fatal(p)
			}
		})
	}
}
func TestTimeoutAndRedirect(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		_, err := New(20*time.Millisecond).Search(context.Background(), server.URL, "111")
		if !errors.Is(err, model.ErrTimeout) {
			t.Fatal(err)
		}
	})
	t.Run("redirect", func(t *testing.T) {
		called := false
		destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
		defer destination.Close()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
		defer server.Close()
		_, err := New(time.Second).Search(context.Background(), server.URL, "111")
		if !errors.Is(err, model.ErrUpstream) || called {
			t.Fatal("redirect followed")
		}
	})
	for _, base := range []string{"file:///tmp/data", "http://user:pass@example.com", "bad", "https://example.com?x=y"} {
		_, err := New(time.Second).Search(context.Background(), base, "111")
		if !errors.Is(err, model.ErrUpstream) {
			t.Fatal(base)
		}
	}
}
