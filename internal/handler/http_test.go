package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agnos-backend-assignment/internal/auth"
	"agnos-backend-assignment/internal/handler"
	"agnos-backend-assignment/internal/model"
	"agnos-backend-assignment/internal/service"
	"github.com/gin-gonic/gin"
)

type memory struct {
	staff    []model.Staff
	patients []model.Patient
	err      error
}

func (m *memory) HospitalByCode(_ context.Context, code string) (model.Hospital, error) {
	if m.err != nil {
		return model.Hospital{}, m.err
	}
	if code == "hospital-a" {
		return model.Hospital{ID: 1, Code: code, APIURL: "https://a.test"}, nil
	}
	if code == "hospital-b" {
		return model.Hospital{ID: 2, Code: code, APIURL: "https://b.test"}, nil
	}
	return model.Hospital{}, model.ErrNotFound
}
func (m *memory) HospitalByID(ctx context.Context, id int64) (model.Hospital, error) {
	if id == 1 {
		return m.HospitalByCode(ctx, "hospital-a")
	}
	return m.HospitalByCode(ctx, "hospital-b")
}
func (m *memory) CreateStaff(_ context.Context, s model.Staff) (model.Staff, error) {
	for _, old := range m.staff {
		if old.HospitalID == s.HospitalID && old.Username == s.Username {
			return s, model.ErrConflict
		}
	}
	s.ID = int64(len(m.staff) + 1)
	m.staff = append(m.staff, s)
	return s, nil
}
func (m *memory) StaffByUsername(_ context.Context, hospitalID int64, username string) (model.Staff, error) {
	for _, s := range m.staff {
		if s.HospitalID == hospitalID && s.Username == username {
			return s, nil
		}
	}
	return model.Staff{}, model.ErrNotFound
}
func (m *memory) StaffByID(_ context.Context, id int64) (model.Staff, error) {
	if m.err != nil {
		return model.Staff{}, m.err
	}
	for _, s := range m.staff {
		if s.ID == id {
			return s, nil
		}
	}
	return model.Staff{}, model.ErrNotFound
}
func (m *memory) SearchPatients(_ context.Context, hospitalID int64, f model.Filter) ([]model.Patient, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := []model.Patient{}
	for _, p := range m.patients {
		if p.HospitalID != hospitalID {
			continue
		}
		if f.NationalID != "" && (p.NationalID == nil || *p.NationalID != f.NationalID) {
			continue
		}
		if f.PassportID != "" && (p.PassportID == nil || *p.PassportID != f.PassportID) {
			continue
		}
		if f.FirstName != "" && (p.FirstNameEN == nil || !strings.Contains(strings.ToLower(*p.FirstNameEN), strings.ToLower(f.FirstName))) {
			continue
		}
		result = append(result, p)
	}
	start := (f.Page - 1) * f.Limit
	if start >= len(result) {
		return []model.Patient{}, nil
	}
	end := start + f.Limit
	if end > len(result) {
		end = len(result)
	}
	return result[start:end], nil
}
func (m *memory) UpsertPatient(_ context.Context, p model.Patient) error {
	p.ID = int64(len(m.patients) + 1)
	m.patients = append(m.patients, p)
	return nil
}

type his struct {
	patient model.Patient
	err     error
	calls   int
	url, id string
}

func (h *his) Search(_ context.Context, url, id string) (model.Patient, error) {
	h.calls++
	h.url = url
	h.id = id
	return h.patient, h.err
}
func ptr(s string) *string { return &s }

const key = "test-provisioning-key-32-bytes-long"
const secret = "test-jwt-signing-secret-32-bytes-long"

func setup(t *testing.T) (*memory, *his, *service.Service, http.Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	m := &memory{}
	h := &his{err: model.ErrNotFound}
	s := service.New(m, h, auth.New(secret, time.Hour))
	r := handler.Router(s, key, func(context.Context) error { return m.err })
	return m, h, s, r
}
func request(r http.Handler, method, path, body, token, provision string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if provision != "" {
		req.Header.Set("X-Provisioning-Key", provision)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func tokenFor(t *testing.T, s *service.Service, hospital string) string {
	t.Helper()
	_, err := s.CreateStaff(context.Background(), "staff01", "password123", hospital)
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.Login(context.Background(), "staff01", "password123", hospital)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func TestCreateStaff(t *testing.T) {
	m, _, _, r := setup(t)
	body := `{"username":"staff01","password":"password123","hospital":"hospital-a"}`
	for _, tc := range []struct {
		name, body, key string
		status          int
	}{
		{"missing provisioning", body, "", 403}, {"wrong provisioning", body, "wrong", 403},
		{"created", body, key, 201}, {"duplicate", body, key, 409},
		{"same username different hospital", strings.ReplaceAll(body, "hospital-a", "hospital-b"), key, 201},
		{"missing hospital", strings.ReplaceAll(body, "hospital-a", "missing"), key, 404},
		{"short password", strings.ReplaceAll(body, "password123", "short"), key, 400},
		{"password too long", strings.ReplaceAll(body, "password123", strings.Repeat("a", 73)), key, 400},
		{"malformed", `{`, key, 400}, {"unknown field", strings.ReplaceAll(body, `"username"`, `"admin"`), key, 400},
		{"trailing JSON", body + ` {}`, key, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(r, "POST", "/staff/create", tc.body, "", tc.key)
			if w.Code != tc.status {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
			if strings.Contains(w.Body.String(), "password") && w.Code == 201 {
				t.Fatal("password leaked")
			}
		})
	}
	if m.staff[0].PasswordHash == "password123" || !strings.HasPrefix(m.staff[0].PasswordHash, "$2") {
		t.Fatal("password was not bcrypt hashed")
	}
}
func TestLogin(t *testing.T) {
	m, _, s, r := setup(t)
	tokenFor(t, s, "hospital-a")
	body := `{"username":"staff01","password":"password123","hospital":"hospital-a"}`
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"success", body, 200}, {"bad password", strings.ReplaceAll(body, "password123", "wrong"), 401},
		{"wrong hospital", strings.ReplaceAll(body, "hospital-a", "hospital-b"), 401},
		{"unknown hospital", strings.ReplaceAll(body, "hospital-a", "missing"), 401},
		{"unknown staff", strings.ReplaceAll(body, "staff01", "other"), 401}, {"missing fields", `{}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(r, "POST", "/staff/login", tc.body, "", "")
			if w.Code != tc.status {
				t.Fatalf("%d: %s", w.Code, w.Body)
			}
			if w.Code == 200 {
				var response struct {
					Data struct {
						AccessToken string `json:"access_token"`
					} `json:"data"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				claims, err := s.Tokens.Parse(response.Data.AccessToken)
				if err != nil || claims.HospitalID != 1 {
					t.Fatalf("bad claims: %v", err)
				}
			}
		})
	}
	m.err = errors.New("database failure with private details")
	w := request(r, "POST", "/staff/login", body, "", "")
	if w.Code != 500 || strings.Contains(w.Body.String(), "private") {
		t.Fatal(w.Body)
	}
}
func TestSearchAuthorization(t *testing.T) {
	m, h, s, r := setup(t)
	token := tokenFor(t, s, "hospital-a")
	m.patients = []model.Patient{{ID: 1, HospitalID: 1, PatientHN: "A"}, {ID: 2, HospitalID: 2, PatientHN: "B", NationalID: ptr("222")}}
	expired, _ := auth.New(secret, -time.Hour).Issue(m.staff[0])
	wrongKey, _ := auth.New("different-secret-at-least-32-bytes", time.Hour).Issue(m.staff[0])
	for _, tc := range []struct {
		name, token string
		status      int
	}{
		{"no token", "", 401}, {"invalid token", "bad", 401}, {"expired", expired, 401}, {"wrong signature", wrongKey, 401}, {"valid", token, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(r, "GET", "/patient/search", "", tc.token, "")
			if w.Code != tc.status {
				t.Fatalf("%d: %s", w.Code, w.Body)
			}
			if strings.Contains(w.Body.String(), `"patient_hn":"B"`) {
				t.Fatal("cross-hospital leak")
			}
		})
	}
	w := request(r, "GET", "/patient/search?national_id=222", "", token, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"data":[]`) || h.url != "https://a.test" {
		t.Fatalf("cross hospital lookup: %s", w.Body)
	}
	m.staff[0].HospitalID = 2
	if request(r, "GET", "/patient/search", "", token, "").Code != 401 {
		t.Fatal("stale hospital claim accepted")
	}
	m.staff = nil
	if request(r, "GET", "/patient/search", "", token, "").Code != 401 {
		t.Fatal("deleted staff accepted")
	}
}
func TestSearchValidation(t *testing.T) {
	_, h, s, r := setup(t)
	token := tokenFor(t, s, "hospital-a")
	for _, query := range []string{"hospital_id=2", "page=0", "limit=101", "page=1000001", "first_name=A&first_name=B", "date_of_birth=2025-02-30", "email=bad", "national_id=..%2F123", "first_name=%ZZ", "first_name=" + strings.Repeat("x", 255)} {
		t.Run(query, func(t *testing.T) {
			w := request(r, "GET", "/patient/search?"+query, "", token, "")
			if w.Code != 400 {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
		})
	}
	w := request(r, "GET", "/patient/search?first_name=Nobody", "", token, "")
	if w.Code != 200 || h.calls != 0 {
		t.Fatal("name-only search called HIS")
	}
}
func TestSearchHIS(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		upstream    error
		national    string
		status      int
		hasPatient  bool
	}{
		{"import", "national_id=111", nil, "111", 200, true},
		{"passport", "passport_id=PASS", nil, "111", 200, true},
		{"all filters reapplied", "national_id=111&first_name=Other", nil, "111", 200, false},
		{"both identifiers mismatch", "national_id=111&passport_id=OTHER", nil, "111", 502, false},
		{"identity mismatch", "national_id=111", nil, "999", 502, false},
		{"not found", "national_id=111", model.ErrNotFound, "", 200, false},
		{"timeout", "national_id=111", model.ErrTimeout, "", 504, false},
		{"upstream failure", "national_id=111", model.ErrUpstream, "", 502, false},
		{"later page", "national_id=111&page=2", nil, "111", 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, h, s, r := setup(t)
			token := tokenFor(t, s, "hospital-a")
			h.err = tc.upstream
			h.patient = model.Patient{HospitalID: 999, PatientHN: "HIS-HN", NationalID: ptr(tc.national), PassportID: ptr("PASS"), FirstNameEN: ptr("Demo")}
			w := request(r, "GET", "/patient/search?"+tc.query, "", token, "")
			if w.Code != tc.status {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
			if strings.Contains(w.Body.String(), `"patient_hn":"HIS-HN"`) != tc.hasPatient {
				t.Fatal(w.Body)
			}
			if h.calls != 1 || h.url != "https://a.test" {
				t.Fatal("incorrect upstream selection")
			}
			if len(m.patients) > 0 && m.patients[0].HospitalID != 1 {
				t.Fatal("trusted upstream hospital_id")
			}
			if tc.hasPatient {
				_ = request(r, "GET", "/patient/search?"+tc.query, "", token, "")
				if h.calls != 1 {
					t.Fatal("cached identity fetched again")
				}
			}
		})
	}
}
func TestHealthAndRouting(t *testing.T) {
	m, _, _, r := setup(t)
	for path, status := range map[string]int{"/health/live": 200, "/health/ready": 200, "/missing": 404} {
		if w := request(r, "GET", path, "", "", ""); w.Code != status {
			t.Fatal(w.Code)
		}
	}
	if request(r, "GET", "/staff/login", "", "", "").Code != 405 {
		t.Fatal("method not rejected")
	}
	m.err = errors.New("offline")
	if request(r, "GET", "/health/ready", "", "", "").Code != 503 {
		t.Fatal("unready")
	}
}

func TestRequestBodyAndAuthFailureBoundaries(t *testing.T) {
	m, _, s, r := setup(t)
	token := tokenFor(t, s, "hospital-a")
	req := httptest.NewRequest("POST", "/staff/login", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 415 {
		t.Fatalf("content type: %d", w.Code)
	}
	w = request(r, "POST", "/staff/create", `{"username":"`+strings.Repeat("x", 9000)+`"}`, "", key)
	if w.Code != 400 {
		t.Fatalf("body limit: %d", w.Code)
	}
	m.err = errors.New("database unavailable")
	w = request(r, "GET", "/patient/search", "", token, "")
	if w.Code != 500 {
		t.Fatalf("auth database failure: %d", w.Code)
	}
}
