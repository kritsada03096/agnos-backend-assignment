package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"agnos-backend-assignment/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ Pool *pgxpool.Pool }

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrNotFound
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return model.ErrConflict
	}
	return err
}
func (r *Postgres) HospitalByCode(ctx context.Context, code string) (model.Hospital, error) {
	var h model.Hospital
	err := r.Pool.QueryRow(ctx, "SELECT id,code,name,api_url FROM hospitals WHERE code=$1", code).Scan(&h.ID, &h.Code, &h.Name, &h.APIURL)
	return h, mapError(err)
}
func (r *Postgres) HospitalByID(ctx context.Context, id int64) (model.Hospital, error) {
	var h model.Hospital
	err := r.Pool.QueryRow(ctx, "SELECT id,code,name,api_url FROM hospitals WHERE id=$1", id).Scan(&h.ID, &h.Code, &h.Name, &h.APIURL)
	return h, mapError(err)
}
func (r *Postgres) CreateStaff(ctx context.Context, s model.Staff) (model.Staff, error) {
	err := r.Pool.QueryRow(ctx, "INSERT INTO staff(hospital_id,username,password_hash) VALUES($1,$2,$3) RETURNING id", s.HospitalID, s.Username, s.PasswordHash).Scan(&s.ID)
	return s, mapError(err)
}
func (r *Postgres) StaffByUsername(ctx context.Context, hospitalID int64, username string) (model.Staff, error) {
	var s model.Staff
	err := r.Pool.QueryRow(ctx, "SELECT id,hospital_id,username,password_hash FROM staff WHERE hospital_id=$1 AND username=$2", hospitalID, username).Scan(&s.ID, &s.HospitalID, &s.Username, &s.PasswordHash)
	return s, mapError(err)
}
func (r *Postgres) StaffByID(ctx context.Context, id int64) (model.Staff, error) {
	var s model.Staff
	err := r.Pool.QueryRow(ctx, "SELECT id,hospital_id,username,password_hash FROM staff WHERE id=$1", id).Scan(&s.ID, &s.HospitalID, &s.Username, &s.PasswordHash)
	return s, mapError(err)
}

func searchQuery(hospitalID int64, f model.Filter) (string, []any) {
	query := `SELECT id,hospital_id,patient_hn,national_id,passport_id,first_name_th,middle_name_th,last_name_th,first_name_en,middle_name_en,last_name_en,to_char(date_of_birth,'YYYY-MM-DD'),phone_number,email,gender FROM patients WHERE hospital_id=$1`
	args := []any{hospitalID}
	for _, field := range []struct{ column, value string }{
		{"national_id", f.NationalID}, {"passport_id", f.PassportID}, {"date_of_birth", f.DateOfBirth}, {"phone_number", f.PhoneNumber}, {"email", f.Email},
	} {
		if field.value != "" {
			args = append(args, field.value)
			query += fmt.Sprintf(" AND %s=$%d", field.column, len(args))
		}
	}
	for _, field := range []struct{ column, value string }{{"first_name", f.FirstName}, {"middle_name", f.MiddleName}, {"last_name", f.LastName}} {
		if field.value != "" {
			// Treat user input as a literal substring, including SQL LIKE metacharacters.
			value := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(field.value)
			args = append(args, "%"+value+"%")
			query += fmt.Sprintf(" AND (%s_th ILIKE $%d OR %s_en ILIKE $%d)", field.column, len(args), field.column, len(args))
		}
	}
	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	query += fmt.Sprintf(" ORDER BY id LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	return query, args
}
func (r *Postgres) SearchPatients(ctx context.Context, hospitalID int64, f model.Filter) ([]model.Patient, error) {
	query, args := searchQuery(hospitalID, f)
	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	patients := []model.Patient{}
	for rows.Next() {
		var p model.Patient
		if err := rows.Scan(&p.ID, &p.HospitalID, &p.PatientHN, &p.NationalID, &p.PassportID, &p.FirstNameTH, &p.MiddleNameTH, &p.LastNameTH, &p.FirstNameEN, &p.MiddleNameEN, &p.LastNameEN, &p.DateOfBirth, &p.PhoneNumber, &p.Email, &p.Gender); err != nil {
			return nil, err
		}
		patients = append(patients, p)
	}
	return patients, rows.Err()
}
func (r *Postgres) UpsertPatient(ctx context.Context, p model.Patient) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO patients(hospital_id,patient_hn,national_id,passport_id,first_name_th,middle_name_th,last_name_th,first_name_en,middle_name_en,last_name_en,date_of_birth,phone_number,email,gender,last_synced_at)
	VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,now())
	ON CONFLICT(hospital_id,patient_hn) DO UPDATE SET
	national_id=EXCLUDED.national_id,passport_id=EXCLUDED.passport_id,first_name_th=EXCLUDED.first_name_th,middle_name_th=EXCLUDED.middle_name_th,last_name_th=EXCLUDED.last_name_th,
	first_name_en=EXCLUDED.first_name_en,middle_name_en=EXCLUDED.middle_name_en,last_name_en=EXCLUDED.last_name_en,date_of_birth=EXCLUDED.date_of_birth,phone_number=EXCLUDED.phone_number,email=EXCLUDED.email,gender=EXCLUDED.gender,updated_at=now(),last_synced_at=now()`,
		p.HospitalID, p.PatientHN, p.NationalID, p.PassportID, p.FirstNameTH, p.MiddleNameTH, p.LastNameTH, p.FirstNameEN, p.MiddleNameEN, p.LastNameEN, p.DateOfBirth, p.PhoneNumber, p.Email, p.Gender)
	return err
}
