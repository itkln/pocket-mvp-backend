package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

type DataProtector struct {
	aead      cipher.AEAD
	lookupKey []byte
}

func NewDataProtector(encryptionKey, lookupKey []byte) (*DataProtector, error) {
	if len(encryptionKey) != 32 {
		return nil, errors.New("data encryption key must be exactly 32 bytes")
	}
	if len(lookupKey) < 32 {
		return nil, errors.New("data lookup key must be at least 32 bytes")
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	return &DataProtector{aead: aead, lookupKey: append([]byte(nil), lookupKey...)}, nil
}

func DecodeKey(value string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, errors.New("key must be standard base64")
	}
	return key, nil
}

func (p *DataProtector) Encrypt(value, field string) (string, error) {
	sealed, err := p.EncryptBytes([]byte(value), field)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (p *DataProtector) EncryptBytes(value []byte, field string) ([]byte, error) {
	nonce := make([]byte, p.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate encryption nonce: %w", err)
	}
	ciphertext := p.aead.Seal(nil, nonce, value, []byte(field))
	return append(nonce, ciphertext...), nil
}

func (p *DataProtector) Decrypt(value, field string) (string, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", errors.New("invalid encrypted value")
	}
	plaintext, err := p.DecryptBytes(sealed, field)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (p *DataProtector) DecryptBytes(sealed []byte, field string) ([]byte, error) {
	if len(sealed) < p.aead.NonceSize()+p.aead.Overhead() {
		return nil, errors.New("invalid encrypted value")
	}
	nonce := sealed[:p.aead.NonceSize()]
	plaintext, err := p.aead.Open(nil, nonce, sealed[p.aead.NonceSize():], []byte(field))
	if err != nil {
		return nil, errors.New("decrypt value: authentication failed")
	}
	return plaintext, nil
}

func (p *DataProtector) Lookup(normalizedValue string) []byte {
	mac := hmac.New(sha256.New, p.lookupKey)
	_, _ = mac.Write([]byte(normalizedValue))
	return mac.Sum(nil)
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
