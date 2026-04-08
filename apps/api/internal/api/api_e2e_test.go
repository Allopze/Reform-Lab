package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/api"
	"github.com/allopze/reform-lab/apps/api/internal/auth"
	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/google/uuid"
)

// Shared metrics instance to avoid Prometheus double-registration across tests.
var (
	sharedMetrics     *observability.Metrics
	sharedMetricsOnce sync.Once
)

// testEnv holds all the components needed for an E2E test.
type testEnv struct {
	server *httptest.Server
	tmpDir string
	orch   *orchestrator.Service
}

func withFeatureFlags(t *testing.T, disabledCapabilities, disabledEngines []string) func() {
	t.Helper()
	old := capabilities.DefaultFlags
	capabilities.DefaultFlags = capabilities.NewFeatureFlags(disabledCapabilities, disabledEngines)
	return func() { capabilities.DefaultFlags = old }
}

func (e *testEnv) close() {
	e.server.Close()
	os.RemoveAll(e.tmpDir)
}

func setupE2E(t *testing.T) *testEnv {
	t.Helper()

	tmpDir := t.TempDir()

	// SQLite in temp directory
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Run migrations
	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := database.Migrate(db, migrationsDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Storage
	storagePath := filepath.Join(tmpDir, "storage")
	store, err := storage.NewFilesystem(storagePath)
	if err != nil {
		t.Fatalf("init storage: %v", err)
	}

	// Repositories
	fileRepo := repository.NewFileRepository(db)
	jobRepo := repository.NewJobRepository(db)
	artifactRepo := repository.NewArtifactRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	userRepo := repository.NewUserRepository(db)
	dashboardRepo := repository.NewDashboardRepository(db)

	// Auth
	authSvc := auth.NewService(userRepo, "test-secret-key-for-e2e-tests")

	// Queue — nil handler drops tasks silently (workers don't run in E2E).
	jobQueue := queue.NewInProcessQueue(nil)
	t.Cleanup(func() { jobQueue.Close() })

	// Orchestrator
	orch := orchestrator.NewService(jobRepo, auditRepo, jobQueue)

	// Ensure engines are probed.
	capabilities.DefaultProber.Probe()

	logger := observability.NewLogger("disabled")
	sharedMetricsOnce.Do(func() {
		sharedMetrics = observability.NewMetrics()
	})
	metrics := sharedMetrics

	router := api.NewRouter(api.Deps{
		Logger:           logger,
		Metrics:          metrics,
		Store:            store,
		Files:            fileRepo,
		Jobs:             jobRepo,
		Artifacts:        artifactRepo,
		Audit:            auditRepo,
		Users:            userRepo,
		Dashboard:        dashboardRepo,
		Orchestrator:     orch,
		AuthService:      authSvc,
		CORSOrigin:       "*",
		ArtifactTTLHours: 24,
		ArtifactTTLByFamily: map[string]int{
			"pdf":      48,
			"image":    12,
			"document": 24,
			"audio":    72,
			"video":    96,
		},
	})

	return &testEnv{
		server: httptest.NewServer(router),
		tmpDir: tmpDir,
		orch:   orch,
	}
}

// ── Helpers ──

func doPost(url string, body interface{}, token string) (*http.Response, map[string]interface{}) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(http.MethodPost, url, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, _ := http.DefaultClient.Do(req)
	data := decode(resp)
	return resp, data
}

func doGet(url, token string) (*http.Response, map[string]interface{}) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, _ := http.DefaultClient.Do(req)
	data := decode(resp)
	return resp, data
}

func decode(resp *http.Response) map[string]interface{} {
	defer resp.Body.Close()
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	return m
}

func uploadPNG(baseURL, token string) (*http.Response, map[string]interface{}) {
	// Create a minimal valid PNG image in memory.
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, img)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.png")
	part.Write(pngBuf.Bytes())
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, _ := http.DefaultClient.Do(req)
	data := decode(resp)
	return resp, data
}

// ── Tests ──

func TestE2E_HealthEndpoint(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	resp, data := doGet(env.server.URL+"/api/health", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if data["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", data["status"])
	}
	retention, ok := data["retention"].(map[string]interface{})
	if !ok {
		t.Fatal("expected retention object")
	}
	if retention["artifactTTLHours"] != float64(24) {
		t.Fatalf("expected artifactTTLHours=24, got %v", retention["artifactTTLHours"])
	}
	byFamily, ok := retention["artifactTTLHoursByFamily"].(map[string]interface{})
	if !ok {
		t.Fatal("expected artifactTTLHoursByFamily object")
	}
	if byFamily["image"] != float64(12) || byFamily["pdf"] != float64(48) {
		t.Fatalf("unexpected per-family retention: %v", byFamily)
	}
	featureFlags, ok := data["featureFlags"].(map[string]interface{})
	if !ok {
		t.Fatal("expected featureFlags object")
	}
	if disabledCaps, ok := featureFlags["disabledCapabilities"].([]interface{}); !ok || len(disabledCaps) != 0 {
		t.Fatalf("expected no disabled capabilities by default, got %v", featureFlags["disabledCapabilities"])
	}
	if disabledEngines, ok := featureFlags["disabledEngines"].([]interface{}); !ok || len(disabledEngines) != 0 {
		t.Fatalf("expected no disabled engines by default, got %v", featureFlags["disabledEngines"])
	}
}

func TestE2E_RegisterAndLogin(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	// Register first user (should become admin).
	resp, data := doPost(env.server.URL+"/api/auth/register", map[string]string{
		"name":     "Alice",
		"email":    "alice@test.com",
		"password": "password123",
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d — %v", resp.StatusCode, data)
	}
	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatal("register: expected non-empty token")
	}

	// /auth/me should return admin role.
	resp, me := doGet(env.server.URL+"/api/auth/me", token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me: expected 200, got %d — %v", resp.StatusCode, me)
	}
	if me["role"] != "admin" {
		t.Fatalf("first user should be admin, got role=%v", me["role"])
	}

	// Login with same credentials.
	resp, loginData := doPost(env.server.URL+"/api/auth/login", map[string]string{
		"email":    "alice@test.com",
		"password": "password123",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d — %v", resp.StatusCode, loginData)
	}
	if _, ok := loginData["token"].(string); !ok {
		t.Fatal("login: expected non-empty token")
	}
}

func TestE2E_UploadAndCapabilities(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	// Register and get token.
	_, regData := doPost(env.server.URL+"/api/auth/register", map[string]string{
		"name":     "Bob",
		"email":    "bob@test.com",
		"password": "password123",
	}, "")
	token := regData["token"].(string)

	// Upload a PNG file.
	resp, fileData := uploadPNG(env.server.URL, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID, ok := fileData["id"].(string)
	if !ok || fileID == "" {
		t.Fatal("upload: expected non-empty file ID")
	}

	// Detected format should be image/png.
	format, _ := fileData["detectedFormat"].(map[string]interface{})
	if format["family"] != "image" {
		t.Fatalf("expected family=image, got %v", format["family"])
	}

	// Get capabilities for this file.
	resp, capsData := doGet(fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("capabilities: expected 200, got %d — %v", resp.StatusCode, capsData)
	}
	caps, ok := capsData["capabilities"].([]interface{})
	if !ok {
		t.Fatal("capabilities: expected array")
	}
	// PNG images should have at least image-to-jpg (go-image engine is always available).
	if len(caps) == 0 {
		t.Fatal("capabilities: expected at least one capability for PNG image")
	}
}

func TestE2E_ConversionCreatesJob(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	// Register.
	_, regData := doPost(env.server.URL+"/api/auth/register", map[string]string{
		"name":     "Carol",
		"email":    "carol@test.com",
		"password": "password123",
	}, "")
	token := regData["token"].(string)

	// Upload.
	_, fileData := uploadPNG(env.server.URL, token)
	fileID := fileData["id"].(string)

	// Get capabilities and pick one.
	_, capsData := doGet(fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), token)
	caps := capsData["capabilities"].([]interface{})
	if len(caps) == 0 {
		t.Fatal("no capabilities to test conversion")
	}
	firstCap := caps[0].(map[string]interface{})
	capID := firstCap["id"].(string)

	// Create conversion.
	resp, jobData := doPost(env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}
	jobID, ok := jobData["id"].(string)
	if !ok || jobID == "" {
		t.Fatal("conversion: expected non-empty job ID")
	}
	if jobData["status"] != "queued" {
		t.Fatalf("expected job status=queued, got %v", jobData["status"])
	}

	// Get job status.
	resp, jobStatus := doGet(fmt.Sprintf("%s/api/jobs/%s", env.server.URL, jobID), token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("job status: expected 200, got %d — %v", resp.StatusCode, jobStatus)
	}
	// Job should still be queued (no worker is processing).
	if jobStatus["status"] != "queued" {
		t.Fatalf("expected queued, got %v", jobStatus["status"])
	}
}

func TestE2E_UnauthorizedAccessBlocked(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	// Unauthenticated upload should be rejected.
	resp, _ := uploadPNG(env.server.URL, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated upload, got %d", resp.StatusCode)
	}

	// Unauthenticated dashboard should be rejected.
	resp, _ = doGet(env.server.URL+"/api/dashboard/me", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated dashboard, got %d", resp.StatusCode)
	}

	// Invalid token should be rejected.
	resp, _ = doGet(env.server.URL+"/api/dashboard/me", "invalid-token-xyz")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", resp.StatusCode)
	}
}

func TestE2E_OwnershipIsolation(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	// Register two users.
	_, u1Data := doPost(env.server.URL+"/api/auth/register", map[string]string{
		"name":     "User1",
		"email":    "user1@test.com",
		"password": "password123",
	}, "")
	token1 := u1Data["token"].(string)

	_, u2Data := doPost(env.server.URL+"/api/auth/register", map[string]string{
		"name":     "User2",
		"email":    "user2@test.com",
		"password": "password123",
	}, "")
	token2 := u2Data["token"].(string)

	// User1 uploads a file.
	_, fileData := uploadPNG(env.server.URL, token1)
	fileID := fileData["id"].(string)

	// User2 should NOT be able to see User1's file capabilities.
	resp, _ := doGet(fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), token2)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-user access, got %d", resp.StatusCode)
	}
}

func TestE2E_DashboardReturnsData(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	// Register (admin).
	_, regData := doPost(env.server.URL+"/api/auth/register", map[string]string{
		"name":     "Admin",
		"email":    "admin@test.com",
		"password": "password123",
	}, "")
	token := regData["token"].(string)

	// User dashboard.
	resp, dashData := doGet(env.server.URL+"/api/dashboard/me", token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("user dashboard: expected 200, got %d", resp.StatusCode)
	}
	if dashData["totalFiles"] == nil {
		t.Fatal("user dashboard: expected totalFiles field")
	}

	// Create observable activity for admin overview.
	_, fileData := uploadPNG(env.server.URL, token)
	fileID := fileData["id"].(string)
	_, capsData := doGet(fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), token)
	caps := capsData["capabilities"].([]interface{})
	capID := caps[0].(map[string]interface{})["id"].(string)
	_, jobData := doPost(env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, token)
	jobID := jobData["id"].(string)
	resp, _ = doPost(fmt.Sprintf("%s/api/jobs/%s/cancel", env.server.URL, jobID), nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel job in dashboard test: expected 200, got %d", resp.StatusCode)
	}

	// Admin overview (first user is admin).
	resp, adminData := doGet(env.server.URL+"/api/admin/overview", token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin overview: expected 200, got %d", resp.StatusCode)
	}
	if adminData["totalUsers"] == nil {
		t.Fatal("admin overview: expected totalUsers field")
	}
	if adminData["successRatePct"] == nil {
		t.Fatal("admin overview: expected successRatePct field")
	}
	if adminData["averageDurationSec"] == nil {
		t.Fatal("admin overview: expected averageDurationSec field")
	}
	if adminData["availableEngines"] == nil || adminData["totalEngines"] == nil {
		t.Fatal("admin overview: expected engine availability fields")
	}
	if adminData["engineUsage"] == nil {
		t.Fatal("admin overview: expected engineUsage field")
	}
	auditEvents, ok := adminData["recentAudit"].([]interface{})
	if !ok || len(auditEvents) == 0 {
		t.Fatal("admin overview: expected recentAudit events")
	}

	// Admin engines.
	resp, engData := doGet(env.server.URL+"/api/admin/engines", token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin engines: expected 200, got %d", resp.StatusCode)
	}
	if engData["engines"] == nil {
		t.Fatal("admin engines: expected engines field")
	}
}

func TestE2E_NonAdminCannotAccessAdminEndpoints(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	// First user = admin.
	doPost(env.server.URL+"/api/auth/register", map[string]string{
		"name":     "Admin",
		"email":    "admin@test.com",
		"password": "password123",
	}, "")

	// Second user = regular user.
	_, regData := doPost(env.server.URL+"/api/auth/register", map[string]string{
		"name":     "Regular",
		"email":    "regular@test.com",
		"password": "password123",
	}, "")
	userToken := regData["token"].(string)

	resp, _ := doGet(env.server.URL+"/api/admin/overview", userToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", resp.StatusCode)
	}
}

func TestE2E_CancelJob(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	// Register + upload + create conversion.
	_, regData := doPost(env.server.URL+"/api/auth/register", map[string]string{
		"name":     "CancelUser",
		"email":    "cancel@test.com",
		"password": "password123",
	}, "")
	token := regData["token"].(string)

	_, fileData := uploadPNG(env.server.URL, token)
	fileID := fileData["id"].(string)

	_, capsData := doGet(fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), token)
	caps := capsData["capabilities"].([]interface{})
	capID := caps[0].(map[string]interface{})["id"].(string)

	_, jobData := doPost(env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, token)
	jobID := jobData["id"].(string)

	// Cancel the queued job.
	resp, _ := doPost(fmt.Sprintf("%s/api/jobs/%s/cancel", env.server.URL, jobID), nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel: expected 200, got %d", resp.StatusCode)
	}

	// Verify the job is cancelled.
	resp, jobStatus := doGet(fmt.Sprintf("%s/api/jobs/%s", env.server.URL, jobID), token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("job status after cancel: expected 200, got %d", resp.StatusCode)
	}
	if jobStatus["status"] != "cancelled" {
		t.Fatalf("expected cancelled, got %v", jobStatus["status"])
	}
}

func TestE2E_RetryFailedJob(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	_, regData := doPost(env.server.URL+"/api/auth/register", map[string]string{
		"name":     "RetryUser",
		"email":    "retry@test.com",
		"password": "password123",
	}, "")
	token := regData["token"].(string)

	_, fileData := uploadPNG(env.server.URL, token)
	fileID := fileData["id"].(string)

	_, capsData := doGet(fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), token)
	caps := capsData["capabilities"].([]interface{})
	capID := caps[0].(map[string]interface{})["id"].(string)

	_, jobData := doPost(env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, token)
	jobID := jobData["id"].(string)
	parsedJobID, err := uuid.Parse(jobID)
	if err != nil {
		t.Fatalf("parse job id: %v", err)
	}

	if err := env.orch.MarkRunning(context.Background(), parsedJobID); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	if err := env.orch.MarkFailed(context.Background(), parsedJobID, "simulated worker failure"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	resp, retryData := doPost(fmt.Sprintf("%s/api/jobs/%s/retry", env.server.URL, jobID), nil, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("retry: expected 201, got %d — %v", resp.StatusCode, retryData)
	}
	if retryData["status"] != "queued" {
		t.Fatalf("expected retried job queued, got %v", retryData["status"])
	}
	if retryData["id"] == jobID {
		t.Fatal("expected a new job id on retry")
	}

	resp, originalStatus := doGet(fmt.Sprintf("%s/api/jobs/%s", env.server.URL, jobID), token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("original job fetch: expected 200, got %d", resp.StatusCode)
	}
	if originalStatus["status"] != "failed" {
		t.Fatalf("expected original job to remain failed, got %v", originalStatus["status"])
	}
	if originalStatus["error"] != "simulated worker failure" {
		t.Fatalf("expected preserved failure error, got %v", originalStatus["error"])
	}
}

func TestE2E_DisabledCapabilityIsHiddenAndRejected(t *testing.T) {
	defer withFeatureFlags(t, []string{"image-to-jpg"}, nil)()
	env := setupE2E(t)
	defer env.close()

	_, regData := doPost(env.server.URL+"/api/auth/register", map[string]string{
		"name":     "FlagUser",
		"email":    "flag@test.com",
		"password": "password123",
	}, "")
	token := regData["token"].(string)

	_, fileData := uploadPNG(env.server.URL, token)
	fileID := fileData["id"].(string)

	resp, capsData := doGet(fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("capabilities: expected 200, got %d", resp.StatusCode)
	}
	caps := capsData["capabilities"].([]interface{})
	for _, item := range caps {
		capability := item.(map[string]interface{})
		if capability["id"] == "image-to-jpg" {
			t.Fatal("expected image-to-jpg to be hidden by feature flag")
		}
	}

	resp, data := doPost(env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": "image-to-jpg",
	}, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("conversion with disabled capability: expected 400, got %d — %v", resp.StatusCode, data)
	}

	resp, health := doGet(env.server.URL+"/api/health", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health with feature flags: expected 200, got %d", resp.StatusCode)
	}
	featureFlags := health["featureFlags"].(map[string]interface{})
	disabledCaps := featureFlags["disabledCapabilities"].([]interface{})
	if len(disabledCaps) != 1 || disabledCaps[0] != "image-to-jpg" {
		t.Fatalf("unexpected disabled capabilities snapshot: %v", disabledCaps)
	}
}
