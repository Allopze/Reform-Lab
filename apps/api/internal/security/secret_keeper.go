package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const encryptedSecretPrefix = "enc:v1:"

var ErrSecretEncryptionUnavailable = errors.New("secret encryption is not configured")

type SecretKeeper struct {
	aead cipher.AEAD
}

func NewSecretKeeper(key string) (*SecretKeeper, error) {
	if strings.TrimSpace(key) == "" {
		return &SecretKeeper{}, nil
	}

	rawKey, err := parseSecretKeeperKey(key)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(rawKey)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	return &SecretKeeper{aead: aead}, nil
}

func (k *SecretKeeper) Enabled() bool {
	return k != nil && k.aead != nil
}

func (k *SecretKeeper) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if !k.Enabled() {
		return "", ErrSecretEncryptionUnavailable
	}
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := k.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return encryptedSecretPrefix + base64.StdEncoding.EncodeToString(payload), nil
}

func (k *SecretKeeper) Decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, encryptedSecretPrefix) {
		return value, nil
	}
	if !k.Enabled() {
		return "", ErrSecretEncryptionUnavailable
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encryptedSecretPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted secret: %w", err)
	}
	nonceSize := k.aead.NonceSize()
	if len(payload) < nonceSize {
		return "", fmt.Errorf("encrypted secret payload is too short")
	}
	nonce := payload[:nonceSize]
	ciphertext := payload[nonceSize:]
	plaintext, err := k.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}

func IsEncryptedSecret(value string) bool {
	return strings.HasPrefix(value, encryptedSecretPrefix)
}

func parseSecretKeeperKey(key string) ([]byte, error) {
	trimmed := strings.TrimSpace(key)
	for _, decoder := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		decoded, err := decoder.DecodeString(trimmed)
		if err == nil {
			if len(decoded) == 32 {
				return decoded, nil
			}
		}
	}
	if len(trimmed) == 32 {
		return []byte(trimmed), nil
	}
	return nil, fmt.Errorf("secret encryption key must be 32 raw bytes or base64 for 32 bytes")
}
