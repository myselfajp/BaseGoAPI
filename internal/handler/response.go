// Package handler holds the Gin HTTP handlers. It is the Go equivalent of
// app/router/*.py.
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/myselfajp/BaseGoAPI/internal/apperror"
)

// respondError writes an error as JSON. Handled AppErrors use their status code
// and message (mirroring FastAPI's HTTPException -> {"detail": ...}); any other
// error becomes a 500 with a generic body.
func respondError(c *gin.Context, err error) {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.Status, gin.H{"detail": appErr.Message})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"status": "error",
		"detail": err.Error(),
	})
}

// bindJSON binds and validates the request body, writing a 400 on failure.
func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return false
	}
	return true
}
