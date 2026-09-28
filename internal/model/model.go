package model

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("username already exists in this hospital")
	ErrUnauthorized = errors.New("invalid credentials")
	ErrUpstream     = errors.New("hospital service unavailable or invalid response")
	ErrTimeout      = errors.New("hospital service timed out")
)

type Hospital struct {
	ID     int64  `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	APIURL string `json:"-"`
}

type Staff struct {
	ID           int64  `json:"id"`
	HospitalID   int64  `json:"hospital_id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
}

type Patient struct {
	ID           int64   `json:"id"`
	HospitalID   int64   `json:"hospital_id"`
	PatientHN    string  `json:"patient_hn"`
	NationalID   *string `json:"national_id"`
	PassportID   *string `json:"passport_id"`
	FirstNameTH  *string `json:"first_name_th"`
	MiddleNameTH *string `json:"middle_name_th"`
	LastNameTH   *string `json:"last_name_th"`
	FirstNameEN  *string `json:"first_name_en"`
	MiddleNameEN *string `json:"middle_name_en"`
	LastNameEN   *string `json:"last_name_en"`
	DateOfBirth  *string `json:"date_of_birth"`
	PhoneNumber  *string `json:"phone_number"`
	Email        *string `json:"email"`
	Gender       *string `json:"gender"`
}

type Filter struct {
	NationalID  string
	PassportID  string
	FirstName   string
	MiddleName  string
	LastName    string
	DateOfBirth string
	PhoneNumber string
	Email       string
	Page        int
	Limit       int
}

type SearchResult struct {
	Patients []Patient `json:"data"`
	Page     int       `json:"page"`
	Limit    int       `json:"limit"`
}
