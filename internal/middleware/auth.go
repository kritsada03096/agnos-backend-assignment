package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"agnos-backend-assignment/internal/auth"
	"agnos-backend-assignment/internal/model"
	"github.com/gin-gonic/gin"
)

type Authenticator interface {
	Authenticate(context.Context, string) (*auth.Claims, error)
}

func RequireAuth(a Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			reject(c, 401, "authentication required")
			return
		}
		claims, err := a.Authenticate(c.Request.Context(), parts[1])
		if err != nil {
			if errors.Is(err, model.ErrUnauthorized) {
				reject(c, 401, "invalid or expired token")
			} else {
				reject(c, 500, "internal server error")
			}
			return
		}
		c.Set("hospital_id", claims.HospitalID)
		c.Set("staff_id", claims.StaffID)
		c.Next()
	}
}
func RequireProvisioningKey(key string) gin.HandlerFunc {
	expected := sha256.Sum256([]byte(key))
	return func(c *gin.Context) {
		actual := sha256.Sum256([]byte(c.GetHeader("X-Provisioning-Key")))
		if key == "" || subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
			reject(c, http.StatusForbidden, "staff provisioning key required")
			return
		}
		c.Next()
	}
}
func reject(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"success": false, "message": message})
}
