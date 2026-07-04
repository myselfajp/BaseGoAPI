package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Recovery builds middleware that converts panics into a JSON 500 response,
// mirroring the global exception handler in the FastAPI app.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"status": "error",
					"detail": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}
