package service

import (
	"context"
	"errors"

	"agnos-backend-assignment/internal/auth"
	"agnos-backend-assignment/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type Store interface {
	HospitalByCode(context.Context, string) (model.Hospital, error)
	HospitalByID(context.Context, int64) (model.Hospital, error)
	CreateStaff(context.Context, model.Staff) (model.Staff, error)
	StaffByUsername(context.Context, int64, string) (model.Staff, error)
	StaffByID(context.Context, int64) (model.Staff, error)
	SearchPatients(context.Context, int64, model.Filter) ([]model.Patient, error)
	UpsertPatient(context.Context, model.Patient) error
}
type HospitalClient interface {
	Search(context.Context, string, string) (model.Patient, error)
}
type Service struct {
	Store     Store
	HIS       HospitalClient
	Tokens    *auth.Tokens
	dummyHash []byte
}

func New(store Store, his HospitalClient, tokens *auth.Tokens) *Service {
	hash, _ := bcrypt.GenerateFromPassword([]byte("dummy-password-for-login-timing"), bcrypt.DefaultCost)
	return &Service{store, his, tokens, hash}
}
func (s *Service) CreateStaff(ctx context.Context, username, password, hospital string) (model.Staff, error) {
	h, err := s.Store.HospitalByCode(ctx, hospital)
	if err != nil {
		return model.Staff{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.Staff{}, err
	}
	return s.Store.CreateStaff(ctx, model.Staff{HospitalID: h.ID, Username: username, PasswordHash: string(hash)})
}
func (s *Service) Login(ctx context.Context, username, password, hospital string) (string, error) {
	h, err := s.Store.HospitalByCode(ctx, hospital)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
			return "", model.ErrUnauthorized
		}
		return "", err
	}
	staff, err := s.Store.StaffByUsername(ctx, h.ID, username)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
			return "", model.ErrUnauthorized
		}
		return "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(staff.PasswordHash), []byte(password)) != nil {
		return "", model.ErrUnauthorized
	}
	return s.Tokens.Issue(staff)
}
func (s *Service) Authenticate(ctx context.Context, raw string) (*auth.Claims, error) {
	c, err := s.Tokens.Parse(raw)
	if err != nil {
		return nil, model.ErrUnauthorized
	}
	staff, err := s.Store.StaffByID(ctx, c.StaffID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, model.ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if staff.HospitalID != c.HospitalID || staff.Username != c.Username {
		return nil, model.ErrUnauthorized
	}
	return c, nil
}

func (s *Service) Search(ctx context.Context, hospitalID int64, f model.Filter) (model.SearchResult, error) {
	result := model.SearchResult{Patients: []model.Patient{}, Page: f.Page, Limit: f.Limit}
	patients, err := s.Store.SearchPatients(ctx, hospitalID, f)
	if err != nil {
		return result, err
	}
	result.Patients = patients
	if len(patients) > 0 {
		return result, nil
	}
	id := f.NationalID
	if id == "" {
		id = f.PassportID
	}
	if id == "" {
		return result, nil
	}
	// A miss on a later page or another filter must not refresh an existing identifier.
	identity := model.Filter{NationalID: f.NationalID, PassportID: f.PassportID, Page: 1, Limit: 1}
	existing, err := s.Store.SearchPatients(ctx, hospitalID, identity)
	if err != nil {
		return result, err
	}
	if len(existing) > 0 {
		return result, nil
	}
	h, err := s.Store.HospitalByID(ctx, hospitalID)
	if err != nil {
		return result, err
	}
	if h.APIURL == "" {
		return result, model.ErrUpstream
	}
	p, err := s.HIS.Search(ctx, h.APIURL, id)
	if errors.Is(err, model.ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	// Never trust an upstream hospital_id or identifier that differs from the lookup.
	if (f.NationalID != "" && (p.NationalID == nil || *p.NationalID != f.NationalID)) ||
		(f.PassportID != "" && (p.PassportID == nil || *p.PassportID != f.PassportID)) {
		return result, model.ErrUpstream
	}
	p.ID = 0
	p.HospitalID = hospitalID
	if err := s.Store.UpsertPatient(ctx, p); err != nil {
		return result, err
	}
	// Reapply ALL filters and pagination using the same database semantics.
	result.Patients, err = s.Store.SearchPatients(ctx, hospitalID, f)
	return result, err
}
