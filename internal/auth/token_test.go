package auth

import (
	"testing"
	"time"

	"agnos-backend-assignment/internal/model"
	"github.com/golang-jwt/jwt/v5"
)

func TestRejectInvalidClaims(t *testing.T) {
	secret := "secret-at-least-thirty-two-bytes-long"
	tokens := New(secret, time.Hour)
	valid, err := tokens.Issue(model.Staff{ID: 1, HospitalID: 2, Username: "staff"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := tokens.Parse(valid)
	if err != nil || claims.HospitalID != 2 {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Claims)
		method jwt.SigningMethod
	}{
		{"no expiry", func(c *Claims) { c.ExpiresAt = nil }, jwt.SigningMethodHS256},
		{"issuer", func(c *Claims) { c.Issuer = "other" }, jwt.SigningMethodHS256},
		{"audience", func(c *Claims) { c.Audience = jwt.ClaimStrings{"other"} }, jwt.SigningMethodHS256},
		{"subject", func(c *Claims) { c.Subject = "2" }, jwt.SigningMethodHS256},
		{"hospital", func(c *Claims) { c.HospitalID = 0 }, jwt.SigningMethodHS256},
		{"algorithm", func(c *Claims) {}, jwt.SigningMethodHS384},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := *claims
			tc.change(&copy)
			raw, err := jwt.NewWithClaims(tc.method, copy).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tokens.Parse(raw); err == nil {
				t.Fatal("accepted invalid claims")
			}
		})
	}
}
