package service

import (
	"fmt"

	"github.com/myselfajp/BaseGoAPI/internal/config"
)

// EmailContent bundles the parts of a transactional email.
type EmailContent struct {
	Subject  string
	BodyText string
	BodyHTML string
}

func projectName(cfg *config.Config) string {
	if cfg.ProjectName != "" {
		return cfg.ProjectName
	}
	return "Application"
}

func greeting(fullName string) string {
	if fullName != "" {
		return fmt.Sprintf("Hello %s,", fullName)
	}
	return "Hello,"
}

// BuildVerificationEmail renders the email-verification message.
func BuildVerificationEmail(cfg *config.Config, fullName, token string) EmailContent {
	project := projectName(cfg)
	verificationURL := fmt.Sprintf("%s/v1/auth/verify-email?token=%s", cfg.BaseURL, token)
	g := greeting(fullName)

	subject := "Verify Your Email Address"
	bodyText := fmt.Sprintf(
		"%s\n\n"+
			"Thank you for registering with %s.\n\n"+
			"Please open the following link to verify your email address:\n%s\n\n"+
			"This link will expire in %d minutes.\n\n"+
			"If you did not create an account, please ignore this email.",
		g, project, verificationURL, cfg.EmailVerificationTokenExpirationMinutes,
	)

	bodyHTML := fmt.Sprintf(baseEmailTemplate, project+" - Verify Your Email", fmt.Sprintf(`
      <p>%s</p>
      <p>Thank you for registering with %s.</p>
      <p>Please click the button below to verify your email address:</p>
      <a href="%s" class="button">Verify Email</a>
      <p>Or copy and paste this link into your browser:</p>
      <p style="word-break: break-all; color: #0b7285;">%s</p>
      <p>This link will expire in %d minutes.</p>
      <p>If you did not create an account, please ignore this email.</p>`,
		g, project, verificationURL, verificationURL, cfg.EmailVerificationTokenExpirationMinutes,
	))

	return EmailContent{Subject: subject, BodyText: bodyText, BodyHTML: bodyHTML}
}

// BuildPasswordResetEmail renders the password-reset message.
func BuildPasswordResetEmail(cfg *config.Config, fullName, token string) EmailContent {
	project := projectName(cfg)
	resetURL := fmt.Sprintf("%s/v1/auth/reset-password?token=%s", cfg.BaseURL, token)
	g := greeting(fullName)
	minutes := cfg.PasswordResetTokenExpirationMinutes

	subject := "Reset Your Password"
	bodyText := fmt.Sprintf(
		"%s\n\n"+
			"You requested a password reset for your %s account.\n\n"+
			"Please open the following link to reset your password:\n%s\n\n"+
			"This link will expire in %d minutes.\n\n"+
			"If you did not request a password reset, please ignore this email and your password will remain unchanged.",
		g, project, resetURL, minutes,
	)

	bodyHTML := fmt.Sprintf(baseEmailTemplate, project+" - Reset Your Password", fmt.Sprintf(`
      <p>%s</p>
      <p>You requested a password reset for your %s account.</p>
      <p>Please click the button below to reset your password:</p>
      <a href="%s" class="button">Reset Password</a>
      <p>Or copy and paste this link into your browser:</p>
      <p style="word-break: break-all; color: #0b7285;">%s</p>
      <p>This link will expire in %d minutes.</p>
      <p><strong>Security notice:</strong> If you did not request a password reset, please ignore this email and your password will remain unchanged.</p>`,
		g, project, resetURL, resetURL, minutes,
	))

	return EmailContent{Subject: subject, BodyText: bodyText, BodyHTML: bodyHTML}
}

// BuildOTPEmail renders the two-factor OTP message.
func BuildOTPEmail(cfg *config.Config, fullName, otpCode string, expiresMinutes int) EmailContent {
	project := projectName(cfg)
	g := greeting(fullName)

	subject := "Your One-Time Login Code"
	bodyText := fmt.Sprintf(
		"%s\n\n"+
			"Your one-time login code is: %s\n"+
			"This code is valid for %d minutes.\n\n"+
			"If you did not request this code, please contact support immediately.",
		g, otpCode, expiresMinutes,
	)

	bodyHTML := fmt.Sprintf(baseEmailTemplate, project+" - Your One-Time Login Code", fmt.Sprintf(`
      <p>%s</p>
      <p>Use the one-time code below to complete your login within %d minutes.</p>
      <div class="otp">%s</div>
      <p>If you did not request this code, please contact our support team immediately.</p>`,
		g, expiresMinutes, otpCode,
	))

	return EmailContent{Subject: subject, BodyText: bodyText, BodyHTML: bodyHTML}
}

// baseEmailTemplate is the shared HTML shell. The first %s is the heading, the
// second %s is the body content.
const baseEmailTemplate = `<html>
  <head>
    <meta charset="utf-8" />
    <style>
      body { background-color: #f7f9fc; font-family: Arial, sans-serif; color: #1f2933; padding: 24px; }
      .container { max-width: 600px; margin: 0 auto; background: #ffffff; border-radius: 12px; border: 1px solid #d9e2ec; box-shadow: 0 12px 24px rgba(15, 23, 42, 0.05); padding: 32px 28px; }
      h1 { font-size: 24px; margin-bottom: 16px; color: #0b7285; }
      p { font-size: 15px; line-height: 1.6; margin: 12px 0; }
      .button { display: inline-block; background-color: #0b7285; color: #ffffff; padding: 12px 24px; text-decoration: none; border-radius: 6px; margin: 16px 0; }
      .otp { display: inline-block; font-size: 32px; letter-spacing: 6px; font-weight: bold; color: #142d4c; background: #e0f7fa; border-radius: 8px; padding: 12px 20px; margin: 16px 0; }
      .footer { margin-top: 24px; font-size: 12px; color: #617d98; }
    </style>
  </head>
  <body>
    <div class="container">
      <h1>%s</h1>
      %s
      <div class="footer">This email was sent automatically. Please do not reply to this message.</div>
    </div>
  </body>
</html>`
