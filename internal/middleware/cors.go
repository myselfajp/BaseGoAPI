package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS builds middleware that applies the configured CORS policy. `origins` is
// either "*" or a comma-separated allow-list, mirroring the FastAPI setup.
func CORS(origins string) gin.HandlerFunc {
	allowAll := strings.TrimSpace(origins) == "*"

	allowed := make(map[string]bool)
	if !allowAll {
		for _, o := range strings.Split(origins, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				allowed[trimmed] = true
			}
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		switch {
		case allowAll:
			c.Header("Access-Control-Allow-Origin", "*")
		case origin != "" && allowed[origin]:
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
