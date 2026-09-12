package auth

import "context"

type Mailer interface {
	SendPasswordReset(ctx context.Context, toEmail, resetURL string) error
}

type LogMailer struct {
	Log func(msg string, args ...any)
}

func (m *LogMailer) SendPasswordReset(ctx context.Context, toEmail, resetURL string) error {
	if m.Log != nil {
		m.Log("password reset requested", "to", toEmail, "reset_url", resetURL)
	}
	return nil
}
