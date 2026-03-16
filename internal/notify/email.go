package notify

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/msjurset/runbook/internal/runbook"
)

func sendEmail(cfg *runbook.EmailConfig, subject, body string) error {
	password := cfg.Password
	if password != "" {
		resolved, err := resolveOpRef(password, "email_password")
		if err != nil {
			return fmt.Errorf("resolving email password: %w", err)
		}
		password = resolved
	}

	host := cfg.Host
	// Extract hostname without port for auth
	hostname := host
	if idx := strings.Index(host, ":"); idx >= 0 {
		hostname = host[:idx]
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		cfg.From, cfg.To, subject, body)

	var auth smtp.Auth
	if cfg.Username != "" && password != "" {
		auth = smtp.PlainAuth("", cfg.Username, password, hostname)
	}

	if err := smtp.SendMail(host, auth, cfg.From, []string{cfg.To}, []byte(msg)); err != nil {
		return fmt.Errorf("sending email: %w", err)
	}
	return nil
}
