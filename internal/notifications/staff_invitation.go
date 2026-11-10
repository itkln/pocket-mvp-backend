package notifications

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

func (s *PasswordResetSender) SendStaffInvitation(ctx context.Context, recipient, inviteURL string) error {
	address, parseErr := mail.ParseAddress(recipient)
	if parseErr != nil || address.Address != recipient || strings.ContainsAny(recipient, "\r\n") {
		return fmt.Errorf("invalid invitation recipient")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.address == "" {
		s.logger.Warn("staff invitation captured by development sender", "invite_url", inviteURL)
		return nil
	}
	host, _, err := net.SplitHostPort(s.address)
	if err != nil {
		return err
	}
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, host)
	}
	body := strings.Join([]string{
		"From: Pocket <" + s.from + ">", "To: " + recipient, "Subject: Pocket Vendor team invitation",
		"MIME-Version: 1.0", "Content-Type: text/plain; charset=UTF-8", "", "Вас пригласили в команду заведения Pocket Vendor.",
		"Войдите или зарегистрируйтесь с e-mail, на который пришло приглашение, и подтвердите его.",
		"Ссылка действует 72 часа:", inviteURL, "", "You have been invited to a Pocket Vendor team.",
		"Sign in with this email address and accept the invitation. The link expires in 72 hours.",
	}, "\r\n")
	if err = sendInvitationSMTP(ctx, s.address, host, auth, s.from, recipient, []byte(body)); err != nil {
		return fmt.Errorf("send staff invitation: %w", err)
	}
	return nil
}

// Bound network operations so a stalled SMTP server cannot leave the owner waiting indefinitely.
func sendInvitationSMTP(ctx context.Context, address, host string, auth smtp.Auth, from, to string, body []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}
	if err = client.Mail(from); err != nil {
		return err
	}
	if err = client.Rcpt(to); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = writer.Write(body); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
