package security
package security

import (
	"errors"
	"testing"
)

const testSecretKeeperKey = "0123456789abcdef0123456789abcdef"

func TestSecretKeeperEncryptDecrypt(t *testing.T) {
	t.Parallel()

	keeper, err := NewSecretKeeper(testSecretKeeperKey)
	if err != nil {
		t.Fatalf("NewSecretKeeper: %v", err)
	}

	encrypted, err := keeper.Encrypt("top-secret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if encrypted == "top-secret" || !IsEncryptedSecret(encrypted) {
		t.Fatalf("expected encrypted secret with prefix, got %q", encrypted)
	}

	decrypted, err := keeper.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decrypted != "top-secret" {
		t.Fatalf("expected decrypted plaintext, got %q", decrypted)
	}
}

func TestSecretKeeperDecryptLegacyPlaintext(t *testing.T) {
	t.Parallel()

	keeper, err := NewSecretKeeper("")
	if err != nil {
		t.Fatalf("NewSecretKeeper: %v", err)
	}

	decrypted, err := keeper.Decrypt("legacy-plaintext")
	if err != nil {
		t.Fatalf("Decrypt legacy plaintext: %v", err)
	}
	if decrypted != "legacy-plaintext" {
		t.Fatalf("expected legacy plaintext to pass through, got %q", decrypted)
	}
}

func TestSecretKeeperEncryptFailsWithoutKey(t *testing.T) {
	t.Parallel()

	keeper, err := NewSecretKeeper("")
	if err != nil {
		t.Fatalf("NewSecretKeeper: %v", err)
	}

	_, err = keeper.Encrypt("top-secret")
	if !errors.Is(err, ErrSecretEncryptionUnavailable) {
		t.Fatalf("expected ErrSecretEncryptionUnavailable, got %v", err)
	}
}