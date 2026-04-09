package api_test

import (
	"archive/zip"
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
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/api"
	"github.com/allopze/reform-lab/apps/api/internal/auth"
	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/allopze/reform-lab/apps/api/internal/workers"
	"github.com/allopze/reform-lab/apps/api/internal/workers/document"
	workerImage "github.com/allopze/reform-lab/apps/api/internal/workers/image"
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

type e2eLimits struct {
	userUploadsPerMinute     int
	userUploadBurst          int
	userConversionsPerMinute int
	userConversionBurst      int
	maxActiveJobsPerUser     int
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
	return setupE2EWithConfig(t, e2eLimits{
		userUploadsPerMinute:     12,
		userUploadBurst:          4,
		userConversionsPerMinute: 6,
		userConversionBurst:      3,
		maxActiveJobsPerUser:     3,
	}, false)
}

func setupE2EWithWorker(t *testing.T) *testEnv {
	t.Helper()
	return setupE2EWithConfig(t, e2eLimits{
		userUploadsPerMinute:     12,
		userUploadBurst:          4,
		userConversionsPerMinute: 6,
		userConversionBurst:      3,
		maxActiveJobsPerUser:     3,
	}, true)
}

func setupE2EWithLimits(t *testing.T, limits e2eLimits) *testEnv {
	t.Helper()
	return setupE2EWithConfig(t, limits, false)
}

func setupE2EWithConfig(t *testing.T, limits e2eLimits, withWorker bool) *testEnv {
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
	siteSettingRepo := repository.NewSiteSettingRepository(db)

	// Auth
	authSvc := auth.NewService(userRepo, "test-secret-key-for-e2e-tests")

	logger := observability.NewLogger("disabled")
	sharedMetricsOnce.Do(func() {
		sharedMetrics = observability.NewMetrics()
	})
	metrics := sharedMetrics

	// Queue — existing tests can keep the no-worker behavior, while fixture E2E
	// tests can opt into an embedded worker path.
	var workerHandler *workers.Handler
	if withWorker {
		registry := workers.NewRegistry()
		registry.Register("image-heic-to-png", &workerImage.HEIFConvertEngine{})
		registry.Register("presentation-to-jpg", &document.PresentationToImagesEngine{})
		registry.Register("spreadsheet-to-csv", &document.ToCSVEngine{})

		workerHandler = &workers.Handler{
			Registry:    registry,
			Store:       store,
			Artifacts:   artifactRepo,
			Audit:       auditRepo,
			Logger:      logger,
			Metrics:     metrics,
			ArtifactTTL: 24 * time.Hour,
			ArtifactTTLByFamily: map[domain.FormatFamily]time.Duration{
				domain.FamilyPDF:      48 * time.Hour,
				domain.FamilyImage:    12 * time.Hour,
				domain.FamilyDocument: 24 * time.Hour,
				domain.FamilyAudio:    72 * time.Hour,
				domain.FamilyVideo:    96 * time.Hour,
			},
		}
	}

	jobQueue := queue.NewInProcessQueueWithLimit(nil, 1)
	if workerHandler != nil {
		jobQueue = queue.NewInProcessQueueWithLimit(workerHandler.ProcessPayload, 1)
	}
	t.Cleanup(func() { jobQueue.Close() })

	// Orchestrator
	orch := orchestrator.NewService(jobRepo, auditRepo, jobQueue, orchestrator.WithMaxActiveJobsPerUser(limits.maxActiveJobsPerUser))
	if workerHandler != nil {
		workerHandler.Orch = orch
	}

	// Ensure engines are probed.
	capabilities.DefaultProber.Probe()

	router := api.NewRouter(api.Deps{
		Logger:                   logger,
		Metrics:                  metrics,
		Store:                    store,
		Files:                    fileRepo,
		Jobs:                     jobRepo,
		Artifacts:                artifactRepo,
		Audit:                    auditRepo,
		Users:                    userRepo,
		Dashboard:                dashboardRepo,
		SiteSettings:             siteSettingRepo,
		Orchestrator:             orch,
		AuthService:              authSvc,
		CORSOrigin:               "*",
		ExposeMetrics:            false,
		TrustProxyHeaders:        false,
		UserUploadsPerMinute:     limits.userUploadsPerMinute,
		UserUploadBurst:          limits.userUploadBurst,
		UserConversionsPerMinute: limits.userConversionsPerMinute,
		UserConversionBurst:      limits.userConversionBurst,
		ArtifactTTLHours:         24,
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

func registerUserClient(t *testing.T, env *testEnv, name, email string) *http.Client {
	t.Helper()
	client := newCookieClient(t)
	resp, data := doPostClient(client, env.server.URL+"/api/auth/register", map[string]string{
		"name":     name,
		"email":    email,
		"password": "password123",
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register %s: expected 201, got %d — %v", email, resp.StatusCode, data)
	}
	if _, ok := data["token"]; ok {
		t.Fatalf("register %s: token should not be exposed in auth response", email)
	}
	return client
}

// ── Helpers ──

func doPost(url string, body interface{}, token string) (*http.Response, map[string]interface{}) {
	return doPostClient(nil, url, body, token)
}

func doPostClient(client *http.Client, url string, body interface{}, token string) (*http.Response, map[string]interface{}) {
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
		req.AddCookie(&http.Cookie{Name: "reform_session", Value: token})
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, _ := client.Do(req)
	data := decode(resp)
	return resp, data
}

func doPut(url string, body interface{}, token string) (*http.Response, map[string]interface{}) {
	return doPutClient(nil, url, body, token)
}

func doPutClient(client *http.Client, url string, body interface{}, token string) (*http.Response, map[string]interface{}) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(http.MethodPut, url, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "reform_session", Value: token})
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, _ := client.Do(req)
	data := decode(resp)
	return resp, data
}

func doGet(url, token string) (*http.Response, map[string]interface{}) {
	return doGetClient(nil, url, token)
}

func doGetClient(client *http.Client, url, token string) (*http.Response, map[string]interface{}) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "reform_session", Value: token})
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, _ := client.Do(req)
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
	return uploadPNGClient(nil, baseURL, token)
}

func uploadPNGClient(client *http.Client, baseURL, token string) (*http.Response, map[string]interface{}) {
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
		req.AddCookie(&http.Cookie{Name: "reform_session", Value: token})
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, _ := client.Do(req)
	data := decode(resp)
	return resp, data
}

func uploadFixtureClient(client *http.Client, baseURL, token, fixturePath string) (*http.Response, map[string]interface{}) {
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		panic(err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", filepath.Base(fixturePath))
	_, _ = part.Write(data)
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "reform_session", Value: token})
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, _ := client.Do(req)
	decoded := decode(resp)
	return resp, decoded
}

func fixturePath(parts ...string) string {
	path := []string{"..", "..", "tests", "fixtures"}
	path = append(path, parts...)
	return filepath.Join(path...)
}

func requireCapabilityID(t *testing.T, capsData map[string]interface{}, capabilityID string) string {
	t.Helper()
	items, ok := capsData["capabilities"].([]interface{})
	if !ok {
		t.Fatalf("capabilities payload missing array: %v", capsData)
	}
	for _, item := range items {
		capability, ok := item.(map[string]interface{})
		if ok && capability["id"] == capabilityID {
			return capabilityID
		}
	}
	t.Fatalf("capability %s not available: %v", capabilityID, items)
	return ""
}

func waitForTerminalJob(t *testing.T, client *http.Client, baseURL, jobID string) map[string]interface{} {
	t.Helper()
	for range 60 {
		resp, jobData := doGetClient(client, fmt.Sprintf("%s/api/jobs/%s", baseURL, jobID), "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("job status: expected 200, got %d — %v", resp.StatusCode, jobData)
		}
		status, _ := jobData["status"].(string)
		switch status {
		case "succeeded", "failed", "cancelled", "expired":
			return jobData
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach terminal state in time", jobID)
	return nil
}

func downloadArtifactClient(client *http.Client, baseURL, artifactID string) (*http.Response, []byte) {
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/artifacts/%s/download", baseURL, artifactID), nil)
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func newCookieClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	return &http.Client{Jar: jar}
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
	if _, ok := data["retention"]; ok {
		t.Fatal("public health should not expose retention details")
	}
	if _, ok := data["featureFlags"]; ok {
		t.Fatal("public health should not expose feature flags")
	}
}

func TestE2E_AdminDetailedHealth(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	resp, _ := doGet(env.server.URL+"/api/admin/health", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
	}

	client := registerUserClient(t, env, "HealthAdmin", "health@test.com")

	resp, data := doGetClient(client, env.server.URL+"/api/admin/health", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for admin health, got %d — %v", resp.StatusCode, data)
	}
	retention := data["retention"].(map[string]interface{})
	if retention["artifactTTLHours"] != float64(24) {
		t.Fatalf("expected artifactTTLHours=24, got %v", retention["artifactTTLHours"])
	}
	featureFlags := data["featureFlags"].(map[string]interface{})
	if disabledCaps, ok := featureFlags["disabledCapabilities"].([]interface{}); !ok || len(disabledCaps) != 0 {
		t.Fatalf("expected no disabled capabilities by default, got %v", featureFlags["disabledCapabilities"])
	}
}

func TestE2E_AdminCanUpdateFooterMessage(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	resp, data := doGet(env.server.URL+"/api/footer-message", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public footer message: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["message"] != handlers.DefaultFooterMessage {
		t.Fatalf("unexpected default footer message: %v", data["message"])
	}

	resp, _ = doPut(env.server.URL+"/api/admin/footer-message", map[string]string{"message": "Nuevo footer"}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous footer update, got %d", resp.StatusCode)
	}

	adminClient := registerUserClient(t, env, "FooterAdmin", "footer-admin@test.com")
	userClient := registerUserClient(t, env, "FooterUser", "footer-user@test.com")

	resp, _ = doPutClient(userClient, env.server.URL+"/api/admin/footer-message", map[string]string{"message": "Nuevo footer"}, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin footer update, got %d", resp.StatusCode)
	}

	nextMessage := "Operado por Reform Lab · Conversion segura y trazable"
	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/footer-message", map[string]string{"message": nextMessage}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin footer update: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["message"] != nextMessage {
		t.Fatalf("expected updated footer message, got %v", data["message"])
	}

	resp, data = doGet(env.server.URL+"/api/footer-message", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public footer message after update: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["message"] != nextMessage {
		t.Fatalf("expected public footer message %q, got %v", nextMessage, data["message"])
	}
}

func TestE2E_RegisterAndLogin(t *testing.T) {
	env := setupE2E(t)
	defer env.close()
	client := newCookieClient(t)

	// Register first user (should become admin).
	resp, data := doPostClient(client, env.server.URL+"/api/auth/register", map[string]string{
		"name":     "Alice",
		"email":    "alice@test.com",
		"password": "password123",
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d — %v", resp.StatusCode, data)
	}
	if _, ok := data["token"]; ok {
		t.Fatal("register: token should not be exposed in response")
	}
	if len(resp.Cookies()) == 0 || resp.Cookies()[0].Name != "reform_session" {
		t.Fatalf("register: expected reform_session cookie, got %v", resp.Cookies())
	}

	// /auth/me should return admin role.
	resp, me := doGetClient(client, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me with cookie: expected 200, got %d — %v", resp.StatusCode, me)
	}
	if me["role"] != "admin" {
		t.Fatalf("first user should be admin, got role=%v", me["role"])
	}

	// Login with same credentials.
	resp, loginData := doPostClient(client, env.server.URL+"/api/auth/login", map[string]string{
		"email":    "alice@test.com",
		"password": "password123",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d — %v", resp.StatusCode, loginData)
	}
	if _, ok := loginData["token"]; ok {
		t.Fatal("login: token should not be exposed in response")
	}

	resp, logoutData := doPostClient(client, env.server.URL+"/api/auth/logout", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d — %v", resp.StatusCode, logoutData)
	}

	resp, _ = doGetClient(client, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp.StatusCode)
	}
}

func TestE2E_UploadAndCapabilities(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "Bob", "bob@test.com")

	// Upload a PNG file.
	resp, fileData := uploadPNGClient(client, env.server.URL, "")
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
	resp, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
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

	client := registerUserClient(t, env, "Carol", "carol@test.com")

	// Upload.
	_, fileData := uploadPNGClient(client, env.server.URL, "")
	fileID := fileData["id"].(string)

	// Get capabilities and pick one.
	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	caps := capsData["capabilities"].([]interface{})
	if len(caps) == 0 {
		t.Fatal("no capabilities to test conversion")
	}
	firstCap := caps[0].(map[string]interface{})
	capID := firstCap["id"].(string)

	// Create conversion.
	resp, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
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
	resp, jobStatus := doGetClient(client, fmt.Sprintf("%s/api/jobs/%s", env.server.URL, jobID), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("job status: expected 200, got %d — %v", resp.StatusCode, jobStatus)
	}
	// Job should still be queued (no worker is processing).
	if jobStatus["status"] != "queued" {
		t.Fatalf("expected queued, got %v", jobStatus["status"])
	}
}

func TestE2E_AnonymousPublicFlowAllowed(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	resp, fileData := uploadPNG(env.server.URL, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("anonymous upload: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID := fileData["id"].(string)

	resp, capsData := doGet(fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("anonymous capabilities: expected 200, got %d — %v", resp.StatusCode, capsData)
	}
	caps := capsData["capabilities"].([]interface{})
	if len(caps) == 0 {
		t.Fatal("anonymous capabilities: expected at least one capability")
	}
	capID := caps[0].(map[string]interface{})["id"].(string)

	resp, jobData := doPost(env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("anonymous conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}

	resp, jobStatus := doGet(fmt.Sprintf("%s/api/jobs/%s", env.server.URL, jobData["id"].(string)), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("anonymous job status: expected 200, got %d — %v", resp.StatusCode, jobStatus)
	}
	if jobStatus["status"] != "queued" {
		t.Fatalf("expected queued anonymous job, got %v", jobStatus["status"])
	}
}

func TestE2E_DashboardStillRequiresAuth(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	// Unauthenticated dashboard should be rejected.
	resp, _ := doGet(env.server.URL+"/api/dashboard/me", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated dashboard, got %d", resp.StatusCode)
	}

	// Invalid token should be rejected on protected routes.
	resp, _ = doGet(env.server.URL+"/api/dashboard/me", "invalid-token-xyz")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", resp.StatusCode)
	}
}

func TestE2E_OwnershipIsolation(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	client1 := registerUserClient(t, env, "User1", "user1@test.com")
	client2 := registerUserClient(t, env, "User2", "user2@test.com")

	// User1 uploads a file.
	_, fileData := uploadPNGClient(client1, env.server.URL, "")
	fileID := fileData["id"].(string)

	// User2 should NOT be able to see User1's file capabilities.
	resp, _ := doGetClient(client2, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-user access, got %d", resp.StatusCode)
	}
}

func TestE2E_DashboardReturnsData(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "Admin", "admin@test.com")

	// User dashboard.
	resp, dashData := doGetClient(client, env.server.URL+"/api/dashboard/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("user dashboard: expected 200, got %d", resp.StatusCode)
	}
	if dashData["totalFiles"] == nil {
		t.Fatal("user dashboard: expected totalFiles field")
	}

	// Create observable activity for admin overview.
	_, fileData := uploadPNGClient(client, env.server.URL, "")
	fileID := fileData["id"].(string)
	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	caps := capsData["capabilities"].([]interface{})
	capID := caps[0].(map[string]interface{})["id"].(string)
	_, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	jobID := jobData["id"].(string)
	resp, _ = doPostClient(client, fmt.Sprintf("%s/api/jobs/%s/cancel", env.server.URL, jobID), nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel job in dashboard test: expected 200, got %d", resp.StatusCode)
	}

	// Admin overview (first user is admin).
	resp, adminData := doGetClient(client, env.server.URL+"/api/admin/overview", "")
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
	resp, engData := doGetClient(client, env.server.URL+"/api/admin/engines", "")
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
	registerUserClient(t, env, "Admin", "admin@test.com")

	// Second user = regular user.
	userClient := registerUserClient(t, env, "Regular", "regular@test.com")

	resp, _ := doGetClient(userClient, env.server.URL+"/api/admin/overview", "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", resp.StatusCode)
	}
}

func TestE2E_CancelJob(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "CancelUser", "cancel@test.com")

	_, fileData := uploadPNGClient(client, env.server.URL, "")
	fileID := fileData["id"].(string)

	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	caps := capsData["capabilities"].([]interface{})
	capID := caps[0].(map[string]interface{})["id"].(string)

	_, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	jobID := jobData["id"].(string)

	// Cancel the queued job.
	resp, _ := doPostClient(client, fmt.Sprintf("%s/api/jobs/%s/cancel", env.server.URL, jobID), nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel: expected 200, got %d", resp.StatusCode)
	}

	// Verify the job is cancelled.
	resp, jobStatus := doGetClient(client, fmt.Sprintf("%s/api/jobs/%s", env.server.URL, jobID), "")
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

	client := registerUserClient(t, env, "RetryUser", "retry@test.com")

	_, fileData := uploadPNGClient(client, env.server.URL, "")
	fileID := fileData["id"].(string)

	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	caps := capsData["capabilities"].([]interface{})
	capID := caps[0].(map[string]interface{})["id"].(string)

	_, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
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

	resp, retryData := doPostClient(client, fmt.Sprintf("%s/api/jobs/%s/retry", env.server.URL, jobID), nil, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("retry: expected 201, got %d — %v", resp.StatusCode, retryData)
	}
	if retryData["status"] != "queued" {
		t.Fatalf("expected retried job queued, got %v", retryData["status"])
	}
	if retryData["id"] == jobID {
		t.Fatal("expected a new job id on retry")
	}

	resp, originalStatus := doGetClient(client, fmt.Sprintf("%s/api/jobs/%s", env.server.URL, jobID), "")
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

func TestE2E_RealFixtureHEIFConversion(t *testing.T) {
	if !capabilities.DefaultProber.IsAvailable("libheif") {
		t.Skip("libheif runtime not available")
	}

	env := setupE2EWithWorker(t)
	defer env.close()

	client := registerUserClient(t, env, "HeifUser", "heif@test.com")
	resp, fileData := uploadFixtureClient(client, env.server.URL, "", fixturePath("heif", "valid-basic.heif"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload heif fixture: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID := fileData["id"].(string)

	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	capID := requireCapabilityID(t, capsData, "image-heic-to-png")

	resp, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create heif conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}

	terminal := waitForTerminalJob(t, client, env.server.URL, jobData["id"].(string))
	if terminal["status"] != "succeeded" {
		t.Fatalf("expected succeeded job, got %v", terminal)
	}
	if terminal["artifactFileName"] != "converted.png" {
		t.Fatalf("expected artifact filename converted.png, got %v", terminal["artifactFileName"])
	}
	if terminal["artifactMimeType"] != "image/png" {
		t.Fatalf("expected artifact mime image/png, got %v", terminal["artifactMimeType"])
	}

	artifactID := terminal["artifactId"].(string)
	downloadResp, body := downloadArtifactClient(client, env.server.URL, artifactID)
	if downloadResp.StatusCode != http.StatusOK {
		t.Fatalf("download heif artifact: expected 200, got %d", downloadResp.StatusCode)
	}
	if !strings.Contains(downloadResp.Header.Get("Content-Disposition"), `filename="converted.png"`) {
		t.Fatalf("expected png download filename, got %q", downloadResp.Header.Get("Content-Disposition"))
	}
	if len(body) < 8 || !bytes.Equal(body[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("unexpected png artifact header: %q", body[:min(len(body), 12)])
	}
}

func TestE2E_RealFixtureComplexHEIFConversion(t *testing.T) {
	if !capabilities.DefaultProber.IsAvailable("libheif") {
		t.Skip("libheif runtime not available")
	}

	env := setupE2EWithWorker(t)
	defer env.close()

	client := registerUserClient(t, env, "HeifComplexUser", "heif-complex@test.com")
	resp, fileData := uploadFixtureClient(client, env.server.URL, "", fixturePath("heif", "valid-complex.heif"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload complex heif fixture: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID := fileData["id"].(string)

	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	capID := requireCapabilityID(t, capsData, "image-heic-to-png")

	resp, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create complex heif conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}

	terminal := waitForTerminalJob(t, client, env.server.URL, jobData["id"].(string))
	if terminal["status"] != "succeeded" {
		t.Fatalf("expected succeeded complex heif job, got %v", terminal)
	}

	artifactID := terminal["artifactId"].(string)
	downloadResp, body := downloadArtifactClient(client, env.server.URL, artifactID)
	if downloadResp.StatusCode != http.StatusOK {
		t.Fatalf("download complex heif artifact: expected 200, got %d", downloadResp.StatusCode)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("decode complex png artifact: %v", err)
	}
	if config.Width != 960 || config.Height != 540 {
		t.Fatalf("expected complex png dimensions 960x540, got %dx%d", config.Width, config.Height)
	}
}

func TestE2E_RealFixturePresentationCreatesZIP(t *testing.T) {
	if !capabilities.DefaultProber.IsAvailable("libreoffice-poppler") {
		t.Skip("presentation runtime not available")
	}

	env := setupE2EWithWorker(t)
	defer env.close()

	client := registerUserClient(t, env, "SlidesUser", "slides@test.com")
	resp, fileData := uploadFixtureClient(client, env.server.URL, "", fixturePath("presentation", "valid-two-slides.pptx"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload presentation fixture: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID := fileData["id"].(string)

	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	capID := requireCapabilityID(t, capsData, "presentation-to-jpg")

	resp, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create presentation conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}

	terminal := waitForTerminalJob(t, client, env.server.URL, jobData["id"].(string))
	if terminal["status"] != "succeeded" {
		t.Fatalf("expected succeeded presentation job, got %v", terminal)
	}
	if terminal["artifactFileName"] != "slides.zip" {
		t.Fatalf("expected slides.zip artifact, got %v", terminal["artifactFileName"])
	}
	if terminal["artifactMimeType"] != "application/zip" {
		t.Fatalf("expected application/zip artifact, got %v", terminal["artifactMimeType"])
	}

	artifactID := terminal["artifactId"].(string)
	downloadResp, body := downloadArtifactClient(client, env.server.URL, artifactID)
	if downloadResp.StatusCode != http.StatusOK {
		t.Fatalf("download presentation artifact: expected 200, got %d", downloadResp.StatusCode)
	}
	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("open slide zip: %v", err)
	}
	if len(zipReader.File) != 2 {
		t.Fatalf("expected two slide images in zip, got %d", len(zipReader.File))
	}
	for _, file := range zipReader.File {
		if !strings.HasSuffix(strings.ToLower(file.Name), ".jpg") {
			t.Fatalf("expected jpg slide image, got %s", file.Name)
		}
	}
}

func TestE2E_RealFixturePresentationComplexCreatesZIP(t *testing.T) {
	if !capabilities.DefaultProber.IsAvailable("libreoffice-poppler") {
		t.Skip("presentation runtime not available")
	}

	env := setupE2EWithWorker(t)
	defer env.close()

	client := registerUserClient(t, env, "SlidesComplexUser", "slides-complex@test.com")
	resp, fileData := uploadFixtureClient(client, env.server.URL, "", fixturePath("presentation", "valid-three-slides.pptx"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload complex presentation fixture: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID := fileData["id"].(string)

	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	capID := requireCapabilityID(t, capsData, "presentation-to-jpg")

	resp, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create complex presentation conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}

	terminal := waitForTerminalJob(t, client, env.server.URL, jobData["id"].(string))
	if terminal["status"] != "succeeded" {
		t.Fatalf("expected succeeded complex presentation job, got %v", terminal)
	}

	artifactID := terminal["artifactId"].(string)
	_, body := downloadArtifactClient(client, env.server.URL, artifactID)
	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("open complex slide zip: %v", err)
	}
	if len(zipReader.File) != 3 {
		t.Fatalf("expected three slide images in zip, got %d", len(zipReader.File))
	}
}

func TestE2E_RealFixtureSpreadsheetConvertsToCSV(t *testing.T) {
	if !capabilities.DefaultProber.IsAvailable("libreoffice") {
		t.Skip("spreadsheet runtime not available")
	}

	env := setupE2EWithWorker(t)
	defer env.close()

	client := registerUserClient(t, env, "SheetUser", "sheet@test.com")
	resp, fileData := uploadFixtureClient(client, env.server.URL, "", fixturePath("spreadsheet", "valid-basic.xlsx"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload spreadsheet fixture: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID := fileData["id"].(string)

	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	capID := requireCapabilityID(t, capsData, "spreadsheet-to-csv")

	resp, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create spreadsheet conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}

	terminal := waitForTerminalJob(t, client, env.server.URL, jobData["id"].(string))
	if terminal["status"] != "succeeded" {
		t.Fatalf("expected succeeded spreadsheet job, got %v", terminal)
	}
	if terminal["artifactMimeType"] != "text/csv" {
		t.Fatalf("expected text/csv artifact, got %v", terminal["artifactMimeType"])
	}

	artifactID := terminal["artifactId"].(string)
	downloadResp, body := downloadArtifactClient(client, env.server.URL, artifactID)
	if downloadResp.StatusCode != http.StatusOK {
		t.Fatalf("download spreadsheet artifact: expected 200, got %d", downloadResp.StatusCode)
	}
	text := string(body)
	for _, expected := range []string{"capability,status,count", "presentation-to-jpg", "image-heic-to-png"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in csv artifact, got %q", expected, text)
		}
	}
}

func TestE2E_RealFixtureSpreadsheetMultiSheetConvertsToCSV(t *testing.T) {
	if !capabilities.DefaultProber.IsAvailable("libreoffice") {
		t.Skip("spreadsheet runtime not available")
	}

	env := setupE2EWithWorker(t)
	defer env.close()

	client := registerUserClient(t, env, "SheetComplexUser", "sheet-complex@test.com")
	resp, fileData := uploadFixtureClient(client, env.server.URL, "", fixturePath("spreadsheet", "valid-multi-sheet.xlsx"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload multi-sheet spreadsheet fixture: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID := fileData["id"].(string)

	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	capID := requireCapabilityID(t, capsData, "spreadsheet-to-csv")

	resp, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create multi-sheet spreadsheet conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}

	terminal := waitForTerminalJob(t, client, env.server.URL, jobData["id"].(string))
	if terminal["status"] != "succeeded" {
		t.Fatalf("expected succeeded multi-sheet spreadsheet job, got %v", terminal)
	}

	artifactID := terminal["artifactId"].(string)
	_, body := downloadArtifactClient(client, env.server.URL, artifactID)
	text := string(body)
	for _, expected := range []string{"capability,status,count", "presentation-to-jpg", "image-heic-to-png"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in multi-sheet csv artifact, got %q", expected, text)
		}
	}
}

func TestE2E_DisabledCapabilityIsHiddenAndRejected(t *testing.T) {
	defer withFeatureFlags(t, []string{"image-to-jpg"}, nil)()
	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "FlagUser", "flag@test.com")

	_, fileData := uploadPNGClient(client, env.server.URL, "")
	fileID := fileData["id"].(string)

	resp, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
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

	resp, data := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": "image-to-jpg",
	}, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("conversion with disabled capability: expected 400, got %d — %v", resp.StatusCode, data)
	}

	resp, health := doGetClient(client, env.server.URL+"/api/admin/health", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health with feature flags: expected 200, got %d", resp.StatusCode)
	}
	featureFlags := health["featureFlags"].(map[string]interface{})
	disabledCaps := featureFlags["disabledCapabilities"].([]interface{})
	if len(disabledCaps) != 1 || disabledCaps[0] != "image-to-jpg" {
		t.Fatalf("unexpected disabled capabilities snapshot: %v", disabledCaps)
	}
}

func TestE2E_AuthenticatedUserUploadQuota(t *testing.T) {
	env := setupE2EWithLimits(t, e2eLimits{
		userUploadsPerMinute:     1,
		userUploadBurst:          1,
		userConversionsPerMinute: 60,
		userConversionBurst:      10,
		maxActiveJobsPerUser:     10,
	})
	defer env.close()

	client := registerUserClient(t, env, "QuotaUser", "quota@test.com")

	resp, _ := uploadPNGClient(client, env.server.URL, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first upload: expected 201, got %d", resp.StatusCode)
	}

	resp, data := uploadPNGClient(client, env.server.URL, "")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second upload should hit user quota, got %d — %v", resp.StatusCode, data)
	}
	if data["error"] != "user quota exceeded" {
		t.Fatalf("unexpected quota error: %v", data["error"])
	}
}

func TestE2E_MaxActiveJobsPerUser(t *testing.T) {
	env := setupE2EWithLimits(t, e2eLimits{
		userUploadsPerMinute:     60,
		userUploadBurst:          10,
		userConversionsPerMinute: 60,
		userConversionBurst:      10,
		maxActiveJobsPerUser:     1,
	})
	defer env.close()

	client := registerUserClient(t, env, "LimitedUser", "limit@test.com")
	_, fileData := uploadPNGClient(client, env.server.URL, "")
	fileID := fileData["id"].(string)
	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	capID := capsData["capabilities"].([]interface{})[0].(map[string]interface{})["id"].(string)

	resp, _ := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first job: expected 201, got %d", resp.StatusCode)
	}

	resp, data := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second active job should be blocked, got %d — %v", resp.StatusCode, data)
	}
	if data["error"] != "too many active jobs for this user" {
		t.Fatalf("unexpected active job error: %v", data["error"])
	}
}
