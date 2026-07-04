// Package email delivers transactional email through a configurable provider
// (AWS SES or SMTP). It is the Go equivalent of app/core/email.py.
package email

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	gomail "gopkg.in/gomail.v2"

	"github.com/myselfajp/BaseGoAPI/internal/config"
)

// Sender is the abstraction the rest of the application depends on. It makes
// the email layer easy to stub out in tests.
type Sender interface {
	Send(recipient, subject, bodyText, bodyHTML string) error
}

// Service is the concrete Sender backed by the configured provider.
type Service struct {
	cfg *config.Config
}

// NewService builds an email Service from configuration.
func NewService(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

// Send dispatches an email through the configured provider.
func (s *Service) Send(recipient, subject, bodyText, bodyHTML string) error {
	switch s.cfg.EmailProvider {
	case "aws_ses":
		return s.sendViaSES(recipient, subject, bodyText, bodyHTML)
	case "smtp":
		return s.sendViaSMTP(recipient, subject, bodyText, bodyHTML)
	default:
		return fmt.Errorf("invalid email provider: %q (must be 'aws_ses' or 'smtp')", s.cfg.EmailProvider)
	}
}

func (s *Service) sendViaSES(recipient, subject, bodyText, bodyHTML string) error {
	if s.cfg.AWSAccessKeyID == "" || s.cfg.AWSSecretAccessKey == "" {
		return fmt.Errorf("AWS SES credentials are not configured")
	}
	if s.cfg.SenderEmail == "" {
		return fmt.Errorf("SENDER_EMAIL is not configured")
	}

	awsCfg := aws.Config{
		Region: s.cfg.AWSSESRegion,
		Credentials: credentials.NewStaticCredentialsProvider(
			s.cfg.AWSAccessKeyID, s.cfg.AWSSecretAccessKey, "",
		),
	}
	client := ses.NewFromConfig(awsCfg)

	body := &sestypes.Body{
		Text: &sestypes.Content{Data: aws.String(bodyText)},
	}
	if bodyHTML != "" {
		body.Html = &sestypes.Content{Data: aws.String(bodyHTML)}
	}

	input := &ses.SendEmailInput{
		Source:      aws.String(s.cfg.SenderEmail),
		Destination: &sestypes.Destination{ToAddresses: []string{recipient}},
		Message: &sestypes.Message{
			Subject: &sestypes.Content{Data: aws.String(subject)},
			Body:    body,
		},
	}

	if _, err := client.SendEmail(context.Background(), input); err != nil {
		return fmt.Errorf("failed to send email via AWS SES: %w", err)
	}
	return nil
}

func (s *Service) sendViaSMTP(recipient, subject, bodyText, bodyHTML string) error {
	if s.cfg.SMTPHost == "" {
		return fmt.Errorf("SMTP_HOST is not configured")
	}
	if s.cfg.SenderEmail == "" {
		return fmt.Errorf("SENDER_EMAIL is not configured")
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", s.cfg.SenderEmail)
	msg.SetHeader("To", recipient)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/plain", bodyText)
	if bodyHTML != "" {
		msg.AddAlternative("text/html", bodyHTML)
	}

	dialer := gomail.NewDialer(s.cfg.SMTPHost, s.cfg.SMTPPort, s.cfg.SMTPUser, s.cfg.SMTPPassword)
	// SSL means an implicit TLS connection; otherwise gomail upgrades via
	// STARTTLS automatically when the server advertises it.
	dialer.SSL = s.cfg.SMTPUseSSL

	if err := dialer.DialAndSend(msg); err != nil {
		return fmt.Errorf("failed to send email via SMTP: %w", err)
	}
	return nil
}
