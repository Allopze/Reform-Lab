package api_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	emailpkg "github.com/allopze/reform-lab/apps/api/internal/email"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
)

func TestE2E_PasswordResetRequest_CreatesToken(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	ctx := context.Background()
	settings := repository.NewSiteSettingRepository(env.db)
	if err := settings.UpsertValue(ctx, emailpkg.SettingSMTPHost, "smtp.test", time.Now().UTC()); err != nil {
		t.Fatalf("set smtp host: %v", err)
	}

	client := registerUserClient(t, env, "Alice", "alice-reset@test.com")

	resp, data := doPostClient(client, env.server.URL+"/api/auth/password-reset/request", map[string]string{
		"email": "alice-reset@test.com",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("password reset request: expected 200, got %d — %v", resp.StatusCode, data)
	}

	var count int
	if err := env.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM password_reset_tokens prt
		 JOIN users u ON u.id = prt.user_id
		 WHERE u.email = ?`,
		"alice-reset@test.com",
	).Scan(&count); err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 reset token, got %d", count)
	}
}

func TestE2E_PasswordResetConfirm_UpdatesPasswordAndRevokesSessions(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	ctx := context.Background()
	client := registerUserClient(t, env, "Bob", "bob-reset@test.com")

	resp, me := doGetClient(client, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me: expected 200, got %d — %v", resp.StatusCode, me)
	}
	userID, _ := me["id"].(string)
	if userID == "" {
		t.Fatalf("me: expected id, got %v", me)
	}

	// Create a reset token directly (request endpoint doesn't reveal token).
	rawToken := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	expiresAt := now.Add(1 * time.Hour)
	_, err := env.db.ExecContext(ctx,
		`INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, used_at, created_at)
		 VALUES (?, ?, ?, ?, NULL, ?)`,
		"token-1",
		userID,
		tokenHash,
		expiresAt.Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("insert reset token: %v", err)
	}

	resp, data := doPostClient(client, env.server.URL+"/api/auth/password-reset/confirm", map[string]string{
		"token":    rawToken,
		"password": "newpassword123",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("password reset confirm: expected 200, got %d — %v", resp.StatusCode, data)
	}

	// Session cookie should be cleared/revoked.
	resp, me = doGetClient(client, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me after reset: expected 401, got %d — %v", resp.StatusCode, me)
	}

	// Old password should fail.
	resp, data = doPostClient(newCookieClient(t), env.server.URL+"/api/auth/login", map[string]string{
		"email":    "bob-reset@test.com",
		"password": "password123",
	}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login old password: expected 401, got %d — %v", resp.StatusCode, data)
	}

	// New password should work.
	resp, data = doPostClient(newCookieClient(t), env.server.URL+"/api/auth/login", map[string]string{
		"email":    "bob-reset@test.com",
		"password": "newpassword123",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login new password: expected 200, got %d — %v", resp.StatusCode, data)
	}
}
