package auth

import (
	"fmt"
	"strconv"
	"time"

	"agnos-backend-assignment/internal/model"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	StaffID    int64  `json:"staff_id"`
	HospitalID int64  `json:"hospital_id"`
	Username   string `json:"username"`
	jwt.RegisteredClaims
}

type Tokens struct {
	secret []byte
	ttl    time.Duration
}

func New(secret string, ttl time.Duration) *Tokens { return &Tokens{[]byte(secret), ttl} }
func (t *Tokens) Issue(s model.Staff) (string, error) {
	now := time.Now()
	c := Claims{s.ID, s.HospitalID, s.Username, jwt.RegisteredClaims{
		Issuer: "agnos", Audience: jwt.ClaimStrings{"agnos-api"}, Subject: strconv.FormatInt(s.ID, 10),
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.secret)
}
func (t *Tokens) Parse(raw string) (*Claims, error) {
	c := new(Claims)
	token, err := jwt.ParseWithClaims(raw, c, func(_ *jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("agnos"), jwt.WithAudience("agnos-api"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid || c.StaffID <= 0 || c.HospitalID <= 0 || c.Subject != strconv.FormatInt(c.StaffID, 10) {
		return nil, fmt.Errorf("invalid access token")
	}
	return c, nil
}
