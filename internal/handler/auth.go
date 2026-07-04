package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/myselfajp/BaseGoAPI/internal/config"
	"github.com/myselfajp/BaseGoAPI/internal/core/email"
	"github.com/myselfajp/BaseGoAPI/internal/core/validation"
	"github.com/myselfajp/BaseGoAPI/internal/dto"
	"github.com/myselfajp/BaseGoAPI/internal/model"
	"github.com/myselfajp/BaseGoAPI/internal/service"
)

// AuthHandler exposes the authentication endpoints.
type AuthHandler struct {
	cfg         *config.Config
	userService *service.UserService
	authService *service.AuthService
	emailVerify *service.EmailVerificationService
	passwordRes *service.PasswordResetService
	emailSender email.Sender
}

// NewAuthHandler builds an AuthHandler.
func NewAuthHandler(
	cfg *config.Config,
	userService *service.UserService,
	authService *service.AuthService,
	emailVerify *service.EmailVerificationService,
	passwordRes *service.PasswordResetService,
	emailSender email.Sender,
) *AuthHandler {
	return &AuthHandler{
		cfg:         cfg,
		userService: userService,
		authService: authService,
		emailVerify: emailVerify,
		passwordRes: passwordRes,
		emailSender: emailSender,
	}
}

// Register handles POST /v1/auth/register.
func (h *AuthHandler) Register(c *gin.Context) {
	var input dto.RegisterInput
	if !bindJSON(c, &input) {
		return
	}

	// New self-service registrations always get the default "user" role.
	newUser, err := h.userService.CreateUser(
		input.Email, input.Password, input.FullName, input.PhoneNumber, model.RoleUser, true,
	)
	if err != nil {
		respondError(c, err)
		return
	}

	token, err := h.emailVerify.CreateVerificationToken(newUser.ID)
	if err != nil {
		respondError(c, err)
		return
	}

	// Email delivery is best-effort: registration still succeeds if it fails.
	content := service.BuildVerificationEmail(h.cfg, newUser.FullName, token)
	if err := h.emailSender.Send(newUser.Email, content.Subject, content.BodyText, content.BodyHTML); err != nil {
		log.Printf("failed to send verification email: %v", err)
	}

	c.JSON(http.StatusCreated, dto.RegisterResponse{
		Status:  "success",
		Message: "Registration successful. Please check your email to verify your account.",
		UserID:  newUser.ID,
		Email:   newUser.Email,
	})
}

// VerifyEmail handles POST /v1/auth/verify-email?token=...
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Email verification token is required"})
		return
	}

	if _, err := h.emailVerify.VerifyToken(token); err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.EmailVerificationResponse{
		Status:  "success",
		Message: "Email verified successfully. You can now log in.",
	})
}

// Login handles POST /v1/auth/login. It returns either a login response or an
// OTP challenge, both carrying a distinguishing "status" field.
func (h *AuthHandler) Login(c *gin.Context) {
	var input dto.LoginInput
	if !bindJSON(c, &input) {
		return
	}

	result, err := h.authService.Login(input.Email, input.Password, input.OTPCode, input.OTPChallengeID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ForgotPassword handles POST /v1/auth/forgot-password. The response is always
// identical to prevent email enumeration.
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var input dto.ForgotPasswordInput
	if !bindJSON(c, &input) {
		return
	}

	token, err := h.passwordRes.CreateResetToken(input.Email)
	if err != nil {
		respondError(c, err)
		return
	}

	// Only send an email when a token was actually created (user exists).
	if token != "" {
		if user, uErr := h.userService.GetUserByEmail(input.Email); uErr == nil && user != nil {
			content := service.BuildPasswordResetEmail(h.cfg, user.FullName, token)
			if sErr := h.emailSender.Send(user.Email, content.Subject, content.BodyText, content.BodyHTML); sErr != nil {
				log.Printf("failed to send password reset email: %v", sErr)
			}
		}
	}

	c.JSON(http.StatusOK, dto.ForgotPasswordResponse{
		Status:  "success",
		Message: "If an account with that email exists, a password reset link has been sent.",
	})
}

// ResetPassword handles POST /v1/auth/reset-password.
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var input dto.ResetPasswordInput
	if !bindJSON(c, &input) {
		return
	}

	if err := validation.ValidatePassword(input.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	if _, err := h.passwordRes.ResetPassword(input.Token, input.NewPassword); err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ResetPasswordResponse{
		Status:  "success",
		Message: "Password has been reset successfully. You can now log in with your new password.",
	})
}
