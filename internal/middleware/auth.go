// Package middleware contains the Gin middleware: authentication, the admin
// guard, per-IP rate limiting, CORS and error handling. It is the Go
// counterpart of the FastAPI dependencies in app/service/deps.py plus the
// cross-cutting concerns wired up in app/main.py.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/myselfajp/BaseGoAPI/internal/config"
	"github.com/myselfajp/BaseGoAPI/internal/core/jwtutil"
	"github.com/myselfajp/BaseGoAPI/internal/model"
	"github.com/myselfajp/BaseGoAPI/internal/repository"
)

// ContextUserKey is the Gin context key under which the authenticated user is
// stored.
const ContextUserKey = "currentUser"

// Auth builds middleware that validates the Bearer token and loads the user.
func Auth(cfg *config.Config, userRepo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := extractBearerToken(c)
		if !ok {
			abort(c, http.StatusUnauthorized, "Authorization header missing or invalid")
			return
		}

		email, err := jwtutil.ParseSubject(cfg, token)
		if err != nil {
			abort(c, http.StatusUnauthorized, "Could not validate credentials")
			return
		}

		user, err := userRepo.GetByEmail(email)
		if err != nil {
			abort(c, http.StatusInternalServerError, "Failed to load user")
			return
		}
		if user == nil {
			abort(c, http.StatusUnauthorized, "User not found")
			return
		}

		c.Set(ContextUserKey, user)
		c.Next()
	}
}

// AdminOnly builds middleware that requires the authenticated user to be an
// admin. It must run after Auth.
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := CurrentUser(c)
		if user == nil || !user.IsAdmin() {
			abort(c, http.StatusForbidden, "Access denied. Admin privileges required")
			return
		}
		c.Next()
	}
}

// CurrentUser returns the authenticated user stored in the context, or nil.
func CurrentUser(c *gin.Context) *model.User {
	value, exists := c.Get(ContextUserKey)
	if !exists {
		return nil
	}
	user, ok := value.(*model.User)
	if !ok {
		return nil
	}
	return user
}

func extractBearerToken(c *gin.Context) (string, bool) {
	header := c.GetHeader("Authorization")
	if header == "" {
		return "", false
	}
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func abort(c *gin.Context, status int, detail string) {
	c.Header("WWW-Authenticate", "Bearer")
	c.AbortWithStatusJSON(status, gin.H{"detail": detail})
}
