package config

import "testing"

func TestLoadRejectsWeakJWTSecret(t *testing.T) {
	t.Setenv("ENV_FILE", "/tmp/reform-nonexistent.env")
	t.Setenv("JWT_SECRET", "short-secret")

	_, err := Load()
	if err == nil {
		t.Fatal("expected Load to reject weak JWT secret")
	}
}

func TestLoadAcceptsStrongJWTSecret(t *testing.T) {
	t.Setenv("ENV_FILE", "/tmp/reform-nonexistent.env")
	t.Setenv("JWT_SECRET", "test-strong-jwt-secret-1234567890")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
	if cfg.JWTSecret != "test-strong-jwt-secret-1234567890" {
		t.Fatalf("unexpected JWT secret: %q", cfg.JWTSecret)
	}
}

func TestLoadParsesAuthenticatedUserQuotaSettings(t *testing.T) {
	t.Setenv("ENV_FILE", "/tmp/reform-nonexistent.env")
	t.Setenv("JWT_SECRET", "test-strong-jwt-secret-1234567890")
	t.Setenv("USER_UPLOADS_PER_MINUTE", "18")
	t.Setenv("USER_UPLOAD_BURST", "5")
	t.Setenv("USER_CONVERSIONS_PER_MINUTE", "9")
	t.Setenv("USER_CONVERSION_BURST", "4")
	t.Setenv("MAX_ACTIVE_JOBS_PER_USER", "2")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
	if cfg.UserUploadsPerMinute != 18 || cfg.UserUploadBurst != 5 {
		t.Fatalf("unexpected upload quota config: %d/%d", cfg.UserUploadsPerMinute, cfg.UserUploadBurst)
	}
	if cfg.UserConversionsPerMinute != 9 || cfg.UserConversionBurst != 4 {
		t.Fatalf("unexpected conversion quota config: %d/%d", cfg.UserConversionsPerMinute, cfg.UserConversionBurst)
	}
	if cfg.MaxActiveJobsPerUser != 2 {
		t.Fatalf("unexpected max active jobs: %d", cfg.MaxActiveJobsPerUser)
	}
}

func TestLoadRejectsProductionWithoutRedis(t *testing.T) {
	t.Setenv("ENV_FILE", "/tmp/reform-nonexistent.env")
	t.Setenv("JWT_SECRET", "test-strong-jwt-secret-1234567890")
	t.Setenv("APP_ENV", "production")
	t.Setenv("REDIS_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected Load to reject production without Redis")
	}
}

func TestLoadAcceptsProductionWithRedis(t *testing.T) {
	t.Setenv("ENV_FILE", "/tmp/reform-nonexistent.env")
	t.Setenv("JWT_SECRET", "test-strong-jwt-secret-1234567890")
	t.Setenv("APP_ENV", "production")
	t.Setenv("REDIS_URL", "redis://localhost:6379")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
	if cfg.AppEnv != "production" {
		t.Fatalf("unexpected app env: %q", cfg.AppEnv)
	}
}

func TestLoadAppURL_DefaultsToCORSOrigin(t *testing.T) {
	t.Setenv("ENV_FILE", "/tmp/reform-nonexistent.env")
	t.Setenv("JWT_SECRET", "test-strong-jwt-secret-1234567890")
	t.Setenv("CORS_ORIGIN", "https://myapp.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
	if cfg.AppURL != "https://myapp.example.com" {
		t.Fatalf("expected AppURL to default to CORS_ORIGIN, got %q", cfg.AppURL)
	}
}

func TestLoadAppURL_ExplicitOverride(t *testing.T) {
	t.Setenv("ENV_FILE", "/tmp/reform-nonexistent.env")
	t.Setenv("JWT_SECRET", "test-strong-jwt-secret-1234567890")
	t.Setenv("CORS_ORIGIN", "https://myapp.example.com")
	t.Setenv("APP_URL", "https://custom.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
	if cfg.AppURL != "https://custom.example.com" {
		t.Fatalf("expected AppURL to use APP_URL env, got %q", cfg.AppURL)
	}
}
