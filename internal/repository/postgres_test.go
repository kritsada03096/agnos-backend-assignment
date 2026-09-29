package repository

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"agnos-backend-assignment/internal/database"
	"agnos-backend-assignment/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSearchQueryUsesBoundValuesAndHospitalScope(t *testing.T) {
	attack := `' OR 1=1 --%_`
	query, args := searchQuery(42, model.Filter{FirstName: attack, NationalID: "123", Page: 2, Limit: 20})
	if strings.Contains(query, attack) || !strings.Contains(query, "WHERE hospital_id=$1 AND national_id=$2 AND (first_name_th ILIKE $3 OR first_name_en ILIKE $3)") {
		t.Fatal(query)
	}
	if len(args) != 5 || args[0] != int64(42) || args[1] != "123" || args[2] != `%' OR 1=1 --\%\_%` || args[4] != 20 {
		t.Fatal(args)
	}
}

// Integration tests use a dedicated schema and never truncate existing tables.
func TestPostgresIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("agnos_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for i := 0; i < 2; i++ {
		if err = database.Migrate(ctx, pool); err != nil {
			t.Fatal(err)
		}
	}
	if err = database.Seed(ctx, pool, "http://a.test", "http://b.test"); err != nil {
		t.Fatal(err)
	}
	r := &Postgres{Pool: pool}
	a, _ := r.HospitalByCode(ctx, "hospital-a")
	b, _ := r.HospitalByCode(ctx, "hospital-b")
	if a.ID == 0 || a.ID == b.ID {
		t.Fatal("hospital seed failed")
	}
	staff := model.Staff{HospitalID: a.ID, Username: "staff", PasswordHash: "test-only-hash"}
	created, err := r.CreateStaff(ctx, staff)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.CreateStaff(ctx, staff); err != model.ErrConflict {
		t.Fatalf("duplicate: %v", err)
	}
	staff.HospitalID = b.ID
	if _, err = r.CreateStaff(ctx, staff); err != nil {
		t.Fatal(err)
	}
	if found, err := r.StaffByUsername(ctx, a.ID, "staff"); err != nil || found.ID != created.ID {
		t.Fatal(found, err)
	}
	if found, err := r.StaffByID(ctx, created.ID); err != nil || found.HospitalID != a.ID {
		t.Fatal(found, err)
	}
	str := func(v string) *string { return &v }
	p := model.Patient{HospitalID: a.ID, PatientHN: "same-hn", NationalID: str("111"), PassportID: str("PASS"), FirstNameEN: str("Alice"), FirstNameTH: str("อลิส"), DateOfBirth: str("1990-01-01"), Gender: str("F"), Email: str("a@example.test")}
	p.MiddleNameTH = str("กลาง")
	p.LastNameEN = str("Example")
	p.PhoneNumber = str("0800000000")
	if err = r.UpsertPatient(ctx, p); err != nil {
		t.Fatal(err)
	}
	p.HospitalID = b.ID
	p.FirstNameEN = str("Bob")
	if err = r.UpsertPatient(ctx, p); err != nil {
		t.Fatal(err)
	}
	for _, filter := range []model.Filter{
		{Page: 1, Limit: 20},
		{NationalID: "111", PassportID: "PASS", DateOfBirth: "1990-01-01", Email: "a@example.test", Page: 1, Limit: 20},
		{FirstName: "ali", Page: 1, Limit: 20},
		{FirstName: "อลิ", Page: 1, Limit: 20},
		{NationalID: "111", PassportID: "PASS", FirstName: "ali", MiddleName: "กลาง", LastName: "ample", DateOfBirth: "1990-01-01", PhoneNumber: "0800000000", Email: "a@example.test", Page: 1, Limit: 20},
	} {
		found, err := r.SearchPatients(ctx, a.ID, filter)
		if err != nil || len(found) != 1 || found[0].HospitalID != a.ID || found[0].DateOfBirth == nil || *found[0].DateOfBirth != "1990-01-01" || found[0].MiddleNameEN != nil {
			t.Fatalf("query: %+v %v", found, err)
		}
	}
	for _, filter := range []model.Filter{{FirstName: "Bob", Page: 1, Limit: 20}, {FirstName: "%", Page: 1, Limit: 20}, {NationalID: "' OR 1=1 --", Page: 1, Limit: 20}, {PhoneNumber: "wrong", Page: 1, Limit: 20}, {MiddleName: "wrong", Page: 1, Limit: 20}, {LastName: "wrong", Page: 1, Limit: 20}, {Page: 2, Limit: 1}} {
		found, err := r.SearchPatients(ctx, a.ID, filter)
		if err != nil || len(found) != 0 {
			t.Fatalf("isolation/filter failure: %+v %v", found, err)
		}
	}
	p.HospitalID = a.ID
	p.FirstNameEN = str("Updated")
	if err = r.UpsertPatient(ctx, p); err != nil {
		t.Fatal(err)
	}
	updated, err := r.SearchPatients(ctx, a.ID, model.Filter{Page: 1, Limit: 20})
	if err != nil || len(updated) != 1 || *updated[0].FirstNameEN != "Updated" {
		t.Fatal(updated, err)
	}
	unchanged, err := r.SearchPatients(ctx, b.ID, model.Filter{Page: 1, Limit: 20})
	if err != nil || len(unchanged) != 1 || *unchanged[0].FirstNameEN != "Bob" {
		t.Fatal("upsert changed another hospital", err)
	}
}
