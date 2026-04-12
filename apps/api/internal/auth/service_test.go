package auth

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
)

func testMigrationsPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func newTestAuthService(t *testing.T) (*Service, repository.UserRepository) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "reform-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.Migrate(db, testMigrationsPath(t)); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	users := repository.NewUserRepository(db)
	return NewService(users, "test-secret"), users
}

func TestRegisterFirstUserBecomesAdmin(t *testing.T) {
	svc, users := newTestAuthService(t)
	ctx := context.Background()

	first, err := svc.Register(ctx, RegisterInput{
		Name:     "Admin One",
		Email:    "admin@example.com",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("register first user: %v", err)
	}
	if first.User.Role != domain.RoleAdmin {
		t.Fatalf("expected first user to be admin, got %q", first.User.Role)
	}

	storedFirst, err := users.GetByEmail(ctx, "admin@example.com")
	if err != nil {
		t.Fatalf("reload first user: %v", err)
	}
	if storedFirst.Role != domain.RoleAdmin {
		t.Fatalf("expected persisted first user to be admin, got %q", storedFirst.Role)
	}

	second, err := svc.Register(ctx, RegisterInput{
		Name:     "User Two",
		Email:    "user@example.com",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("register second user: %v", err)
	}
	if second.User.Role != domain.RoleUser {
		t.Fatalf("expected second user to be user, got %q", second.User.Role)
	}

	storedSecond, err := users.GetByEmail(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("reload second user: %v", err)
	}
	if storedSecond.Role != domain.RoleUser {
		t.Fatalf("expected persisted second user to be user, got %q", storedSecond.Role)
	}
	if first.SessionToken == "" || second.SessionToken == "" {
		t.Fatal("expected both registrations to issue tokens")
	}
}

func TestRegisterRequiresExplicitBootstrapWhenConfigured(t *testing.T) {
	svc, _ := newTestAuthService(t)
	svc = NewService(svc.users, "test-secret", WithExplicitBootstrapRequired(true))

	_, err := svc.Register(context.Background(), RegisterInput{
		Name:     "Blocked Admin",
		Email:    "blocked@example.com",
		Password: "secret123",
	})
	if !errors.Is(err, domain.ErrBootstrapAdminRequired) {
		t.Fatalf("expected ErrBootstrapAdminRequired, got %v", err)
	}
}

func TestRegisterAllowsBootstrapAdminEmailWhenConfigured(t *testing.T) {
	svc, users := newTestAuthService(t)
	svc = NewService(
		users,
		"test-secret",
		WithExplicitBootstrapRequired(true),
		WithBootstrapAdminEmails([]string{" owner@example.com "}),
	)

	result, err := svc.Register(context.Background(), RegisterInput{
		Name:     "Owner",
		Email:    "OWNER@example.com",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("register bootstrap admin: %v", err)
	}
	if result.User.Role != domain.RoleAdmin {
		t.Fatalf("expected bootstrap email to become admin, got %q", result.User.Role)
	}
}
