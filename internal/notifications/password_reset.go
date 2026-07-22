package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
)

type PasswordResetSender struct {
	logger   *slog.Logger
	address  string
	username string
	password string
	from     string
}

func NewPasswordResetSender(logger *slog.Logger, address, username, password, from string) *PasswordResetSender {
	return &PasswordResetSender{
		logger: logger, address: strings.TrimSpace(address), username: username,
		password: password, from: strings.TrimSpace(from),
	}
}

func (s *PasswordResetSender) SendPasswordReset(ctx context.Context, recipient, resetURL, locale string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.address == "" {
		s.logger.Warn("password reset e-mail captured by development sender", "reset_url", resetURL)
		return nil
	}

	host, _, err := net.SplitHostPort(s.address)
	if err != nil {
		return fmt.Errorf("invalid SMTP address: %w", err)
	}
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, host)
	}
	message := []byte(strings.Join([]string{
		"From: Pocket <" + s.from + ">",
		"To: " + recipient,
		"Subject: Reset your Pocket password",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		passwordResetBody(locale, resetURL),
	}, "\r\n"))
	if err := smtp.SendMail(s.address, auth, s.from, []string{recipient}, message); err != nil {
		return fmt.Errorf("send SMTP message: %w", err)
	}
	return nil
}

func passwordResetBody(locale, resetURL string) string {
	switch locale {
	case "ru":
		return "Вы запросили смену пароля Pocket. Ссылка действует ограниченное время:\r\n\r\n" + resetURL + "\r\n\r\nЕсли это были не вы, проигнорируйте письмо."
	case "ua", "uk":
		return "Ви запросили зміну пароля Pocket. Посилання діє обмежений час:\r\n\r\n" + resetURL + "\r\n\r\nЯкщо це були не ви, проігноруйте лист."
	case "sk":
		return "Požiadali ste o zmenu hesla Pocket. Odkaz je platný obmedzený čas:\r\n\r\n" + resetURL + "\r\n\r\nAk ste o zmenu nežiadali, tento e-mail ignorujte."
	default:
		return "You requested a Pocket password change. This link is valid for a limited time:\r\n\r\n" + resetURL + "\r\n\r\nIf this was not you, ignore this e-mail."
	}
}
