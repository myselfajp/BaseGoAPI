// Package router registers all HTTP routes and their middleware. It is the Go
// equivalent of the router wiring in app/main.py.
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/myselfajp/BaseGoAPI/internal/config"
	"github.com/myselfajp/BaseGoAPI/internal/handler"
	"github.com/myselfajp/BaseGoAPI/internal/middleware"
	"github.com/myselfajp/BaseGoAPI/internal/repository"
)

// Setup builds the Gin engine with all middleware and routes registered.
func Setup(
	cfg *config.Config,
	userRepo *repository.UserRepository,
	authHandler *handler.AuthHandler,
	userHandler *handler.UserHandler,
) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Logger(), middleware.Recovery(), middleware.CORS(cfg.CORSOrigins))

	engine.GET("/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// --- Authentication (public) ---
	rateLimit := cfg.AuthRateLimitPerMinute
	auth := engine.Group("/v1/auth")
	{
		auth.POST("/register", middleware.RateLimit(rateLimit), authHandler.Register)
		auth.POST("/verify-email", authHandler.VerifyEmail)
		auth.POST("/login", middleware.RateLimit(rateLimit), authHandler.Login)
		auth.POST("/forgot-password", middleware.RateLimit(rateLimit), authHandler.ForgotPassword)
		auth.POST("/reset-password", authHandler.ResetPassword)
	}

	// --- Admin (protected: requires a valid token + admin role) ---
	admin := engine.Group("/v1/admin")
	admin.Use(middleware.Auth(cfg, userRepo), middleware.AdminOnly())
	{
		admin.GET("/users", userHandler.ListUsers)
		admin.GET("/users/:id", userHandler.GetUser)
		admin.POST("/users", userHandler.CreateUser)
		admin.PUT("/users/:id", userHandler.UpdateUser)
		admin.DELETE("/users/:id", userHandler.DeleteUser)
	}

	return engine
}
