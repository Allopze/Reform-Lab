package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	AppEnv                                  string
	Port                                    int
	DatabasePath                            string
	MigrationsPath                          string
	RedisURL                                string // empty = use in-process queue (no Redis needed)
	StorageBasePath                         string
	CORSOrigin                              string
	LogLevel                                string
	JWTSecret                               string
	ExposeMetrics                           bool
	MetricsToken                            string // optional bearer token to protect /metrics
	TrustProxyHeaders                       bool
	InProcessConcurrency                    int
	WorkerConcurrency                       int
	UserUploadsPerMinute                    int
	UserUploadBurst                         int
	UserConversionsPerMinute                int
	UserConversionBurst                     int
	MaxActiveJobsPerUser                    int
	MaxActiveJobsPerGuestSession            int
	ArtifactTTLHours                        int // how many hours artifacts are retained before cleanup
	OriginalTTLHours                        int
	TempTTLHours                            int
	ArtifactTTLByFamily                     map[domain.FormatFamily]int
	GuestCumulativeQuotaBytes               int64 // max total bytes across all files for a guest session
	RegisteredCumulativeQuotaBytes          int64 // max total bytes across all files for a registered user
	DisabledCapabilities                    []string
	DisabledEngines                         []string
	BootstrapAdminEmails                    []string
	RequireVerifiedEmailForSensitiveActions bool

	AppURL string // public URL for email links; defaults to CORS_ORIGIN

	// SMTP configuration — empty SMTPHost disables email sending.
	SMTPHost            string
	SMTPPort            int
	SMTPUser            string
	SMTPPassword        string
	SMTPFrom            string
	SMTPUseTLS          bool
	SecretEncryptionKey string
}

// Load reads configuration from environment variables with sensible defaults.
// It attempts to load a .env file from ENV_FILE (if set) or from the repo root.
func Load() (*Config, error) {
	loadEnvFile()

	appEnv := normalizeAppEnv(os.Getenv("APP_ENV"))

	port := 8080
	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT: %w", err)
		}
		port = p
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/reform.db"
	}

	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "./migrations"
	}

	// Redis is optional. Empty string means in-process queue.
	redisURL := os.Getenv("REDIS_URL")

	storagePath := os.Getenv("STORAGE_BASE_PATH")
	if storagePath == "" {
		storagePath = "./data"
	}

	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:3000"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if err := validateJWTSecret(jwtSecret); err != nil {
		return nil, err
	}

	exposeMetrics := lookupBoolEnv("EXPOSE_METRICS", false)
	metricsToken := os.Getenv("METRICS_TOKEN")
	if appEnv == "production" && exposeMetrics && strings.TrimSpace(metricsToken) == "" {
		return nil, fmt.Errorf("METRICS_TOKEN is required when EXPOSE_METRICS=true in production")
	}
	trustProxyHeaders := lookupBoolEnv("TRUST_PROXY_HEADERS", false)
	inProcessConcurrency := lookupPositiveIntEnv("IN_PROCESS_WORKER_CONCURRENCY", 2)
	workerConcurrency := lookupPositiveIntEnv("WORKER_CONCURRENCY", 2)
	userUploadsPerMinute := lookupPositiveIntEnv("USER_UPLOADS_PER_MINUTE", 6)
	userUploadBurst := lookupPositiveIntEnv("USER_UPLOAD_BURST", 2)
	userConversionsPerMinute := lookupPositiveIntEnv("USER_CONVERSIONS_PER_MINUTE", 4)
	userConversionBurst := lookupPositiveIntEnv("USER_CONVERSION_BURST", 2)
	maxActiveJobsPerUser := lookupPositiveIntEnv("MAX_ACTIVE_JOBS_PER_USER", 2)
	maxActiveJobsPerGuestSession := lookupPositiveIntEnv("MAX_ACTIVE_JOBS_PER_GUEST_SESSION", 1)
	guestCumulativeQuota := lookupPositiveInt64Env("GUEST_CUMULATIVE_QUOTA_BYTES", 25*1024*1024)            // 25 MB
	registeredCumulativeQuota := lookupPositiveInt64Env("REGISTERED_CUMULATIVE_QUOTA_BYTES", 500*1024*1024) // 500 MB

	artifactTTL := 24
	if v := os.Getenv("ARTIFACT_TTL_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			artifactTTL = n
		}
	}
	originalTTL := lookupPositiveIntEnv("ORIGINAL_RETENTION_HOURS", 24)
	tempTTL := lookupPositiveIntEnv("TEMP_RETENTION_HOURS", 6)

	artifactTTLByFamily := map[domain.FormatFamily]int{
		domain.FamilyPDF:      lookupPositiveIntEnv("ARTIFACT_TTL_HOURS_PDF", artifactTTL),
		domain.FamilyImage:    lookupPositiveIntEnv("ARTIFACT_TTL_HOURS_IMAGE", artifactTTL),
		domain.FamilyDocument: lookupPositiveIntEnv("ARTIFACT_TTL_HOURS_DOCUMENT", artifactTTL),
		domain.FamilyAudio:    lookupPositiveIntEnv("ARTIFACT_TTL_HOURS_AUDIO", artifactTTL),
		domain.FamilyVideo:    lookupPositiveIntEnv("ARTIFACT_TTL_HOURS_VIDEO", artifactTTL),
	}

	disabledCapabilities := parseCSVEnv("FEATURE_DISABLE_CAPABILITIES")
	disabledEngines := parseCSVEnv("FEATURE_DISABLE_ENGINES")
	bootstrapAdminEmails := parseCSVEnv("BOOTSTRAP_ADMIN_EMAILS")
	requireVerifiedEmailForSensitiveActions := lookupBoolEnv("REQUIRE_VERIFIED_EMAIL_FOR_SENSITIVE_ACTIONS", false)

	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = corsOrigin
	}

	// SMTP (optional)
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := lookupPositiveIntEnv("SMTP_PORT", 587)
	smtpUser := os.Getenv("SMTP_USER")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	smtpFrom := os.Getenv("SMTP_FROM")
	if smtpFrom == "" {
		smtpFrom = "noreply@example.com"
	}
	smtpUseTLS := lookupBoolEnv("SMTP_USE_TLS", true)
	secretEncryptionKey := os.Getenv("SECRET_ENCRYPTION_KEY")

	if appEnv == "production" && redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is required when APP_ENV=production")
	}

	return &Config{
		AppEnv:                                  appEnv,
		Port:                                    port,
		DatabasePath:                            dbPath,
		MigrationsPath:                          migrationsPath,
		RedisURL:                                redisURL,
		StorageBasePath:                         storagePath,
		CORSOrigin:                              corsOrigin,
		LogLevel:                                logLevel,
		JWTSecret:                               jwtSecret,
		ExposeMetrics:                           exposeMetrics,
		MetricsToken:                            metricsToken,
		TrustProxyHeaders:                       trustProxyHeaders,
		InProcessConcurrency:                    inProcessConcurrency,
		WorkerConcurrency:                       workerConcurrency,
		UserUploadsPerMinute:                    userUploadsPerMinute,
		UserUploadBurst:                         userUploadBurst,
		UserConversionsPerMinute:                userConversionsPerMinute,
		UserConversionBurst:                     userConversionBurst,
		MaxActiveJobsPerUser:                    maxActiveJobsPerUser,
		MaxActiveJobsPerGuestSession:            maxActiveJobsPerGuestSession,
		GuestCumulativeQuotaBytes:               guestCumulativeQuota,
		RegisteredCumulativeQuotaBytes:          registeredCumulativeQuota,
		ArtifactTTLHours:                        artifactTTL,
		OriginalTTLHours:                        originalTTL,
		TempTTLHours:                            tempTTL,
		ArtifactTTLByFamily:                     artifactTTLByFamily,
		DisabledCapabilities:                    disabledCapabilities,
		DisabledEngines:                         disabledEngines,
		BootstrapAdminEmails:                    bootstrapAdminEmails,
		RequireVerifiedEmailForSensitiveActions: requireVerifiedEmailForSensitiveActions,
		AppURL:                                  appURL,
		SMTPHost:                                smtpHost,
		SMTPPort:                                smtpPort,
		SMTPUser:                                smtpUser,
		SMTPPassword:                            smtpPassword,
		SMTPFrom:                                smtpFrom,
		SMTPUseTLS:                              smtpUseTLS,
		SecretEncryptionKey:                     secretEncryptionKey,
	}, nil
}

func normalizeAppEnv(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "dev", "development":
		return "development"
	case "test":
		return "test"
	case "prod", "production":
		return "production"
	default:
		return "development"
	}
}

// loadEnvFile loads variables from a .env file. It checks, in order:
//  1. ENV_FILE environment variable (explicit path)
//  2. .env in the current working directory
//  3. .env in the repo root (../../ relative to apps/api/)
//
// Missing files are silently ignored — env vars may come from the OS or Docker.
func loadEnvFile() {
	if f := os.Getenv("ENV_FILE"); f != "" {
		_ = godotenv.Load(f)
		return
	}
	// Try cwd first, then repo root relative to apps/api/.
	candidates := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".env"))
	}
	candidates = append(candidates, "../../.env")
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			_ = godotenv.Load(c)
			return
		}
	}
}

func lookupPositiveIntEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func lookupPositiveInt64Env(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func lookupBoolEnv(key string, fallback bool) bool {
	if v := strings.TrimSpace(strings.ToLower(os.Getenv(key))); v != "" {
		switch v {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}

func validateJWTSecret(secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if secret == "dev-secret-change-me" || secret == "change-me-in-production" {
		return fmt.Errorf("JWT_SECRET must not use the development placeholder")
	}
	if len(secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	return nil
}

func parseCSVEnv(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}

	seen := make(map[string]struct{})
	values := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
