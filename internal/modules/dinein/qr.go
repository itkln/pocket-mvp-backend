package dinein

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"pocket-mvp-backend/internal/appfault"
	"strings"
)

func (s *Service) OwnerQRCode(ctx context.Context, ownerID, venueID, identifier string, rotate bool) (QRCode, error) {
	if err := s.authorizer.RequireOwner(ctx, ownerID, venueID); err != nil {
		return QRCode{}, err
	}
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || len(identifier) > 48 {
		return QRCode{}, appfault.ErrInvalidInput
	}
	qr, err := s.repository.IssueQR(ctx, venueID, identifier, rotate, func() (string, string, error) {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return "", "", err
		}
		token := base64.RawURLEncoding.EncodeToString(raw)
		encrypted, err := s.protector.Encrypt(token, "table_qr")
		return encrypted, tokenHash(token), err
	})
	if err != nil {
		return QRCode{}, err
	}
	token, err := s.protector.Decrypt(qr.Encrypted, "table_qr")
	if err != nil {
		return QRCode{}, err
	}
	qr.URL = s.baseURL + "/ru/qr/" + token
	return qr, nil
}
func (s *Service) Resolve(ctx context.Context, token string) (Table, error) {
	if !validToken(token) {
		return Table{}, ErrQRInvalid
	}
	return s.repository.Resolve(ctx, tokenHash(token))
}
