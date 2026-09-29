package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"agnos-backend-assignment/internal/middleware"
	"agnos-backend-assignment/internal/model"
	"agnos-backend-assignment/internal/service"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *service.Service }

func Router(s *service.Service, key string, ping func(context.Context) error) *gin.Engine {
	r := gin.New()
	// Gin's default recovery logger dumps request headers and URLs on panic.
	// Suppress that dump because it can contain provisioning keys or patient IDs.
	r.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		slog.Error("request panic", "route", c.FullPath())
		c.AbortWithStatusJSON(500, gin.H{"success": false, "message": "internal server error"})
	}))
	r.Use(func(c *gin.Context) {
		start := time.Now()
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
		// Only log route templates; query strings contain patient identifiers.
		slog.Info("request", "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(), "duration", time.Since(start))
	})
	_ = r.SetTrustedProxies(nil)
	h := Handler{s}
	r.GET("/health/live", func(c *gin.Context) { c.JSON(200, gin.H{"success": true}) })
	r.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := ping(ctx); err != nil {
			fail(c, 503, "database unavailable")
			return
		}
		c.JSON(200, gin.H{"success": true})
	})
	r.POST("/staff/create", middleware.RequireProvisioningKey(key), h.create)
	r.POST("/staff/login", h.login)
	r.GET("/patient/search", middleware.RequireAuth(s), h.search)
	r.NoRoute(func(c *gin.Context) { fail(c, 404, "route not found") })
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) { fail(c, 405, "method not allowed") })
	return r
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Hospital string `json:"hospital"`
}

var accountPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
var idPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

func readCredentials(c *gin.Context, create bool) (credentials, bool) {
	var v credentials
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		fail(c, 415, "Content-Type must be application/json")
		return v, false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF {
		fail(c, 400, "invalid JSON body")
		return v, false
	}
	min := 1
	if create {
		min = 8
	}
	if !accountPattern.MatchString(v.Username) || !accountPattern.MatchString(v.Hospital) || len(v.Password) < min || len(v.Password) > 72 {
		fail(c, 400, "invalid credentials: username/hospital must be 1-64 allowed characters; password must be 8-72 bytes on creation")
		return v, false
	}
	return v, true
}
func (h Handler) create(c *gin.Context) {
	v, ok := readCredentials(c, true)
	if !ok {
		return
	}
	s, err := h.service.CreateStaff(c.Request.Context(), v.Username, v.Password, v.Hospital)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(201, gin.H{"success": true, "data": s})
}
func (h Handler) login(c *gin.Context) {
	v, ok := readCredentials(c, false)
	if !ok {
		return
	}
	token, err := h.service.Login(c.Request.Context(), v.Username, v.Password, v.Hospital)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"access_token": token, "token_type": "Bearer"}})
}
func (h Handler) search(c *gin.Context) {
	f, err := parseFilter(c)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	result, err := h.service.Search(c.Request.Context(), c.GetInt64("hospital_id"), f)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, gin.H{"success": true, "data": result.Patients, "pagination": gin.H{"page": result.Page, "limit": result.Limit}})
}
func parseFilter(c *gin.Context) (model.Filter, error) {
	f := model.Filter{Page: 1, Limit: 20}
	fields := map[string]*string{"national_id": &f.NationalID, "passport_id": &f.PassportID, "first_name": &f.FirstName, "middle_name": &f.MiddleName, "last_name": &f.LastName, "date_of_birth": &f.DateOfBirth, "phone_number": &f.PhoneNumber, "email": &f.Email}
	// Reject malformed URL encoding rather than silently dropping a filter.
	q, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return f, errors.New("invalid query")
	}
	for k, values := range q {
		if len(values) != 1 {
			return f, errors.New("duplicate query parameter")
		}
		value := strings.TrimSpace(values[0])
		if ptr, ok := fields[k]; ok {
			if len(value) > 254 {
				return f, errors.New("filter is too long")
			}
			*ptr = value
		} else if k == "page" || k == "limit" {
			n, e := strconv.Atoi(value)
			if e != nil || n < 1 || (k == "limit" && n > 100) || (k == "page" && n > 1000000) {
				return f, errors.New("invalid pagination")
			}
			if k == "page" {
				f.Page = n
			} else {
				f.Limit = n
			}
		} else {
			return f, errors.New("unknown query parameter: " + k)
		}
	}
	for _, id := range []string{f.NationalID, f.PassportID} {
		if id != "" && !idPattern.MatchString(id) {
			return f, errors.New("invalid national_id or passport_id")
		}
	}
	if f.DateOfBirth != "" {
		if _, err := time.Parse("2006-01-02", f.DateOfBirth); err != nil {
			return f, errors.New("date_of_birth must be YYYY-MM-DD")
		}
	}
	if f.Email != "" {
		address, err := mail.ParseAddress(f.Email)
		if err != nil || address.Address != f.Email {
			return f, errors.New("invalid email")
		}
	}
	return f, nil
}
func fail(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"success": false, "message": message})
}
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		fail(c, 404, "hospital not found")
	case errors.Is(err, model.ErrConflict):
		fail(c, 409, "username already exists in this hospital")
	case errors.Is(err, model.ErrUnauthorized):
		fail(c, 401, "invalid credentials")
	case errors.Is(err, model.ErrTimeout):
		fail(c, 504, "hospital service timed out")
	case errors.Is(err, model.ErrUpstream):
		fail(c, 502, "hospital service unavailable or invalid response")
	default:
		fail(c, 500, "internal server error")
	}
}
