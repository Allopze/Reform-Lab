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
	Port                 int
	DatabasePath         string
	MigrationsPath       string
	RedisURL             string // empty = use in-process queue (no Redis needed)
	StorageBasePath      string
	CORSOrigin           string
	LogLevel             string
	JWTSecret            string
	ArtifactTTLHours     int // how many hours artifacts are retained before cleanup
	ArtifactTTLByFamily  map[domain.FormatFamily]int
	DisabledCapabilities []string
	DisabledEngines      []string
}

// Load reads configuration from environment variables with sensible defaults.
// It attempts to load a .env file from ENV_FILE (if set) or from the repo root.
func Load() (*Config, error) {
	loadEnvFile()

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
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	artifactTTL := 24
	if v := os.Getenv("ARTIFACT_TTL_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			artifactTTL = n
		}
	}

	artifactTTLByFamily := map[domain.FormatFamily]int{
		domain.FamilyPDF:      lookupPositiveIntEnv("ARTIFACT_TTL_HOURS_PDF", artifactTTL),
		domain.FamilyImage:    lookupPositiveIntEnv("ARTIFACT_TTL_HOURS_IMAGE", artifactTTL),
		domain.FamilyDocument: lookupPositiveIntEnv("ARTIFACT_TTL_HOURS_DOCUMENT", artifactTTL),
		domain.FamilyAudio:    lookupPositiveIntEnv("ARTIFACT_TTL_HOURS_AUDIO", artifactTTL),
		domain.FamilyVideo:    lookupPositiveIntEnv("ARTIFACT_TTL_HOURS_VIDEO", artifactTTL),
	}

	disabledCapabilities := parseCSVEnv("FEATURE_DISABLE_CAPABILITIES")
	disabledEngines := parseCSVEnv("FEATURE_DISABLE_ENGINES")

	return &Config{
		Port:                 port,
		DatabasePath:         dbPath,
		MigrationsPath:       migrationsPath,
		RedisURL:             redisURL,
		StorageBasePath:      storagePath,
		CORSOrigin:           corsOrigin,
		LogLevel:             logLevel,
		JWTSecret:            jwtSecret,
		ArtifactTTLHours:     artifactTTL,
		ArtifactTTLByFamily:  artifactTTLByFamily,
		DisabledCapabilities: disabledCapabilities,
		DisabledEngines:      disabledEngines,
	}, nil
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
