package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime"
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

	"github.com/allopze/reform-lab/apps/api/config"
	"github.com/allopze/reform-lab/apps/api/internal/api"
	"github.com/allopze/reform-lab/apps/api/internal/api/handlers"
	"github.com/allopze/reform-lab/apps/api/internal/auth"
	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	emailpkg "github.com/allopze/reform-lab/apps/api/internal/email"
	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/security"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/allopze/reform-lab/apps/api/internal/workers"
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
	db     *sql.DB
}

type e2eLimits struct {
	userUploadsPerMinute     int
	userUploadBurst          int
	userConversionsPerMinute int
	userConversionBurst      int
	maxActiveJobsPerUser     int
	maxActiveJobsPerGuest    int
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
		maxActiveJobsPerGuest:    1,
	}, false, false)
}

func setupE2EWithWorker(t *testing.T) *testEnv {
	t.Helper()
	return setupE2EWithConfig(t, e2eLimits{
		userUploadsPerMinute:     12,
		userUploadBurst:          4,
		userConversionsPerMinute: 6,
		userConversionBurst:      3,
		maxActiveJobsPerUser:     3,
		maxActiveJobsPerGuest:    1,
	}, true, false)
}

func setupE2EWithLimits(t *testing.T, limits e2eLimits) *testEnv {
	t.Helper()
	return setupE2EWithConfig(t, limits, false, false)
}

func setupE2ERequiringVerifiedEmail(t *testing.T) *testEnv {
	t.Helper()
	return setupE2EWithConfig(t, e2eLimits{
		userUploadsPerMinute:     12,
		userUploadBurst:          4,
		userConversionsPerMinute: 6,
		userConversionBurst:      3,
		maxActiveJobsPerUser:     3,
		maxActiveJobsPerGuest:    1,
	}, false, true)
}

func setupE2EWithConfig(t *testing.T, limits e2eLimits, withWorker bool, requireVerifiedEmailForSensitiveActions bool) *testEnv {
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
	workerStatusRepo := repository.NewWorkerStatusRepository(db)
	runtimeControlRepo := repository.NewRuntimeControlRepository(db)
	siteSettingRepo := repository.NewSiteSettingRepository(db)
	secretKeeper, err := security.NewSecretKeeper("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("init secret keeper: %v", err)
	}
	webhookRepo := repository.NewWebhookRepository(db, repository.WithSecretKeeper(secretKeeper))

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
		registry := workers.BuildDefaultRegistry()

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
	orch := orchestrator.NewService(
		jobRepo,
		auditRepo,
		jobQueue,
		orchestrator.WithMaxActiveJobsPerUser(limits.maxActiveJobsPerUser),
		orchestrator.WithMaxActiveJobsPerGuestSession(limits.maxActiveJobsPerGuest),
		orchestrator.WithRuntimeControls(runtimeControlRepo),
	)
	if workerHandler != nil {
		workerHandler.Orch = orch
	}

	// Ensure engines are probed.
	capabilities.DefaultProber.Probe()

	// Email
	emailTemplateRepo := repository.NewEmailTemplateRepository(db)
	emailSvc := emailpkg.NewService(&config.Config{SecretEncryptionKey: "0123456789abcdef0123456789abcdef"}, siteSettingRepo, emailTemplateRepo, logger, emailpkg.WithSecretKeeper(secretKeeper))
	passwordResetRepo := repository.NewPasswordResetRepository(db)
	emailVerificationRepo := repository.NewEmailVerificationRepository(db)

	router := api.NewRouter(api.Deps{
		Logger:                                  logger,
		Metrics:                                 metrics,
		Database:                                db,
		StorageBasePath:                         storagePath,
		Store:                                   store,
		Files:                                   fileRepo,
		Jobs:                                    jobRepo,
		Artifacts:                               artifactRepo,
		Audit:                                   auditRepo,
		Users:                                   userRepo,
		PasswordResets:                          passwordResetRepo,
		EmailVerifications:                      emailVerificationRepo,
		Dashboard:                               dashboardRepo,
		Workers:                                 workerStatusRepo,
		RuntimeControls:                         runtimeControlRepo,
		SiteSettings:                            siteSettingRepo,
		EmailTemplates:                          emailTemplateRepo,
		Webhooks:                                webhookRepo,
		EmailService:                            emailSvc,
		SecretKeeper:                            secretKeeper,
		Queue:                                   jobQueue,
		Orchestrator:                            orch,
		AuthService:                             authSvc,
		AppURL:                                  "http://localhost",
		CORSOrigin:                              "*",
		ExposeMetrics:                           false,
		TrustProxyHeaders:                       false,
		UserUploadsPerMinute:                    limits.userUploadsPerMinute,
		UserUploadBurst:                         limits.userUploadBurst,
		UserConversionsPerMinute:                limits.userConversionsPerMinute,
		UserConversionBurst:                     limits.userConversionBurst,
		GuestCumulativeQuotaBytes:               500 * 1024 * 1024, // 500 MB for test
		RegisteredCumulativeQuotaBytes:          500 * 1024 * 1024, // 500 MB for test
		RequireVerifiedEmailForSensitiveActions: requireVerifiedEmailForSensitiveActions,
		ArtifactTTLHours:                        24,
		ArtifactTTLByFamily: map[string]int{
			"pdf":      48,
			"image":    12,
			"document": 24,
			"audio":    72,
			"video":    96,
		},
		QueueMode:         "in-process",
		WorkerConcurrency: 1,
		RedisURL:          "",
	})

	return &testEnv{
		server: httptest.NewServer(router),
		tmpDir: tmpDir,
		orch:   orch,
		db:     db,
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

func e2eTokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
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

func doPatchClient(client *http.Client, url string, body interface{}) (*http.Response, map[string]interface{}) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(http.MethodPatch, url, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
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

func doGetRawClient(client *http.Client, url, token string) (*http.Response, []byte) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "reform_session", Value: token})
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func doDeleteClient(client *http.Client, url, token string) (*http.Response, map[string]interface{}) {
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
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

func doGetArrayClient(client *http.Client, url, token string) (*http.Response, []interface{}) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "reform_session", Value: token})
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
	}
	defer resp.Body.Close()
	var arr []interface{}
	json.NewDecoder(resp.Body).Decode(&arr)
	return resp, arr
}

func decode(resp *http.Response) map[string]interface{} {
	defer resp.Body.Close()
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	return m
}

func uploadRawFileClient(
	client *http.Client,
	baseURL, token, fileName string,
	data []byte,
) (*http.Response, map[string]interface{}) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", fileName)
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
	dataMap := decode(resp)
	return resp, dataMap
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

func uploadNoisyPNGClient(client *http.Client, baseURL, token string, side int) (*http.Response, map[string]interface{}) {
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	state := uint32(0x12345678)
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			state ^= state << 13
			state ^= state >> 17
			state ^= state << 5
			img.Set(x, y, color.RGBA{
				R: uint8(state >> 16),
				G: uint8(state >> 8),
				B: uint8(state),
				A: 255,
			})
		}
	}

	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, img)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "large-test.png")
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

func TestE2E_PublicCatalogEndpoint(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	resp, data := doGet(env.server.URL+"/api/catalog", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("catalog: expected 200, got %d — %v", resp.StatusCode, data)
	}

	families, _ := data["families"].([]interface{})
	if len(families) == 0 {
		t.Fatal("expected catalog families")
	}
	for _, rawFamily := range families {
		family, _ := rawFamily.(map[string]interface{})
		if family["family"] != "document" {
			continue
		}
		caps, _ := family["capabilities"].([]interface{})
		for _, rawCap := range caps {
			capability, _ := rawCap.(map[string]interface{})
			if capability["id"] != "doc-to-docx" {
				continue
			}
			sources, _ := capability["sourceFormats"].([]interface{})
			for _, source := range sources {
				if source == "application/msword" {
					return
				}
			}
			t.Fatal("expected doc-to-docx sourceFormats to include application/msword")
		}
		t.Fatal("expected document catalog to include doc-to-docx")
	}
	t.Fatal("expected document family in catalog")
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

	runtime := data["runtime"].(map[string]interface{})
	queueInfo := runtime["queue"].(map[string]interface{})
	if queueInfo["mode"] != "in-process" {
		t.Fatalf("expected queue mode in-process, got %v", queueInfo["mode"])
	}
	if queueInfo["stalledJobs"] == nil || queueInfo["stalledQueuedJobs"] == nil || queueInfo["stalledRunningJobs"] == nil {
		t.Fatalf("expected stalled queue fields in health snapshot, got %v", queueInfo)
	}
	deps := data["dependencies"].(map[string]interface{})
	database := deps["database"].(map[string]interface{})
	if database["status"] != "up" {
		t.Fatalf("expected database dependency up, got %v", database["status"])
	}
	if _, ok := data["alerts"].([]interface{}); !ok {
		t.Fatalf("expected alerts to be an array, got %T", data["alerts"])
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

func TestE2E_VerifiedEmailPolicyGatesSensitiveAdminMutations(t *testing.T) {
	env := setupE2ERequiringVerifiedEmail(t)
	defer env.close()

	adminClient := registerUserClient(t, env, "VerifiedPolicyAdmin", "verified-policy-admin@test.com")

	resp, data := doGetClient(adminClient, env.server.URL+"/api/admin/overview", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin overview should remain readable: expected 200, got %d — %v", resp.StatusCode, data)
	}

	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/footer-message", map[string]string{"message": "bloqueado"}, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unverified admin mutation: expected 403, got %d — %v", resp.StatusCode, data)
	}
	if data["error"] != "email verification required" {
		t.Fatalf("unexpected unverified admin error: %v", data["error"])
	}

	resp, me := doGetClient(adminClient, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth/me: expected 200, got %d — %v", resp.StatusCode, me)
	}
	userID, ok := me["id"].(string)
	if !ok || userID == "" {
		t.Fatalf("expected user id in /auth/me response, got %v", me["id"])
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := env.db.Exec(`UPDATE users SET email_verified_at = ? WHERE id = ?`, now, userID); err != nil {
		t.Fatalf("mark email verified: %v", err)
	}

	nextMessage := "Politica sensible con email verificado"
	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/footer-message", map[string]string{"message": nextMessage}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verified admin mutation: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["message"] != nextMessage {
		t.Fatalf("expected updated footer message, got %v", data["message"])
	}
}

func TestE2E_AdminCanUpdateUploadPolicy(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	resp, data := doGet(env.server.URL+"/api/upload-policy", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public upload policy: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["viewerType"] != "guest" {
		t.Fatalf("expected guest viewer type, got %v", data["viewerType"])
	}
	if data["guestMaxBytes"] != float64(10*1024*1024) {
		t.Fatalf("unexpected default guest limit: %v", data["guestMaxBytes"])
	}
	if data["registeredMaxBytes"] != float64(100*1024*1024) {
		t.Fatalf("unexpected default registered limit: %v", data["registeredMaxBytes"])
	}

	resp, _ = doPut(env.server.URL+"/api/admin/upload-policy", map[string]int64{
		"guestMaxBytes":      1 * 1024 * 1024,
		"registeredMaxBytes": 10 * 1024 * 1024,
	}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous upload policy update, got %d", resp.StatusCode)
	}

	adminClient := registerUserClient(t, env, "PolicyAdmin", "policy-admin@test.com")
	userClient := registerUserClient(t, env, "PolicyUser", "policy-user@test.com")

	resp, _ = doPutClient(userClient, env.server.URL+"/api/admin/upload-policy", map[string]int64{
		"guestMaxBytes":      1 * 1024 * 1024,
		"registeredMaxBytes": 10 * 1024 * 1024,
	}, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin upload policy update, got %d", resp.StatusCode)
	}

	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/upload-policy", map[string]int64{
		"guestMaxBytes":      2 * 1024 * 1024,
		"registeredMaxBytes": 12 * 1024 * 1024,
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin upload policy update: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["guestMaxBytes"] != float64(2*1024*1024) {
		t.Fatalf("expected updated guest limit, got %v", data["guestMaxBytes"])
	}
	if data["registeredMaxBytes"] != float64(12*1024*1024) {
		t.Fatalf("expected updated registered limit, got %v", data["registeredMaxBytes"])
	}
	if data["viewerType"] != "registered" {
		t.Fatalf("expected registered viewer type for admin, got %v", data["viewerType"])
	}

	resp, data = doGet(env.server.URL+"/api/upload-policy", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public upload policy after update: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["effectiveMaxBytes"] != float64(2*1024*1024) {
		t.Fatalf("expected guest effective limit, got %v", data["effectiveMaxBytes"])
	}

	resp, data = doGetClient(adminClient, env.server.URL+"/api/upload-policy", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("registered upload policy after update: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["effectiveMaxBytes"] != float64(12*1024*1024) {
		t.Fatalf("expected registered effective limit, got %v", data["effectiveMaxBytes"])
	}

	// Cumulative quota fields should appear in the response.
	if _, ok := data["cumulativeQuotaBytes"]; !ok {
		t.Fatal("expected cumulativeQuotaBytes in upload policy response")
	}
	if data["cumulativeQuotaBytes"] != float64(500*1024*1024) {
		t.Fatalf("expected registered cumulative quota 500 MB, got %v", data["cumulativeQuotaBytes"])
	}
	if data["cumulativeUsedBytes"] != float64(0) {
		t.Fatalf("expected 0 cumulative used bytes, got %v", data["cumulativeUsedBytes"])
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

	// Failed login should be audited as access failure.
	resp, failedLogin := doPostClient(client, env.server.URL+"/api/auth/login", map[string]string{
		"email":    "alice@test.com",
		"password": "wrong-password",
	}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login invalid credentials: expected 401, got %d — %v", resp.StatusCode, failedLogin)
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

	// Login again and verify access audit events are present.
	resp, loginData = doPostClient(client, env.server.URL+"/api/auth/login", map[string]string{
		"email":    "alice@test.com",
		"password": "password123",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second login: expected 200, got %d — %v", resp.StatusCode, loginData)
	}

	resp, auditData := doGetClient(client, env.server.URL+"/api/admin/audit?limit=100", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin audit list: expected 200, got %d — %v", resp.StatusCode, auditData)
	}
	events, _ := auditData["events"].([]interface{})
	seenLogin := false
	seenLoginFailed := false
	seenLogout := false
	for _, row := range events {
		event, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		eventType, _ := event["eventType"].(string)
		switch eventType {
		case string(domain.AuditSessionLogin):
			seenLogin = true
		case string(domain.AuditSessionLoginFailed):
			seenLoginFailed = true
		case string(domain.AuditSessionLogout):
			seenLogout = true
		}
	}
	if !seenLogin || !seenLoginFailed || !seenLogout {
		t.Fatalf("expected session audit events (login, login_failed, logout), got %v", events)
	}
}

func TestE2E_EmailVerificationConfirmMarksUserVerified(t *testing.T) {
	env := setupE2E(t)
	defer env.close()
	client := registerUserClient(t, env, "Verifier", "verify@test.com")

	resp, me := doGetClient(client, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth/me: expected 200, got %d — %v", resp.StatusCode, me)
	}
	if _, ok := me["emailVerifiedAt"]; ok {
		t.Fatalf("email should start unverified, got %v", me["emailVerifiedAt"])
	}
	userID, ok := me["id"].(string)
	if !ok || userID == "" {
		t.Fatalf("expected user id in /auth/me response, got %v", me["id"])
	}

	rawToken := "verify-token-for-e2e"
	now := time.Now().UTC()
	_, err := env.db.Exec(
		`INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at, used_at, created_at)
		 VALUES (?, ?, ?, ?, NULL, ?)`,
		uuid.New().String(),
		userID,
		e2eTokenHash(rawToken),
		now.Add(time.Hour).Format(time.RFC3339),
		now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert verification token: %v", err)
	}

	resp, data := doPost(env.server.URL+"/api/auth/email-verification/confirm", map[string]string{"token": rawToken}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("confirm email verification: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["status"] != "email_verified" {
		t.Fatalf("unexpected verification status: %v", data["status"])
	}

	resp, me = doGetClient(client, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth/me after verification: expected 200, got %d — %v", resp.StatusCode, me)
	}
	if me["emailVerifiedAt"] == nil || me["emailVerifiedAt"] == "" {
		t.Fatalf("expected emailVerifiedAt after verification, got %v", me["emailVerifiedAt"])
	}

	resp, data = doPost(env.server.URL+"/api/auth/email-verification/confirm", map[string]string{"token": rawToken}, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("reusing verification token: expected 400, got %d — %v", resp.StatusCode, data)
	}
}

func TestE2E_ArtifactDownloadIsAudited(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "AuditDownloadUser", "audit-download@test.com")
	resp, me := doGetClient(client, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth me: expected 200, got %d — %v", resp.StatusCode, me)
	}
	userID, _ := me["id"].(string)
	if userID == "" {
		t.Fatal("expected user id in /auth/me response")
	}

	now := time.Now().UTC()
	fileID := uuid.New()
	jobID := uuid.New()
	artifactID := uuid.New()
	artifactDir := filepath.Join(env.tmpDir, "storage", "artifacts", artifactID.String())
	if err := os.MkdirAll(artifactDir, 0o750); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	artifactPath := filepath.Join(artifactDir, "audit.txt")
	if err := os.WriteFile(artifactPath, []byte("audit-download"), 0o600); err != nil {
		t.Fatalf("write artifact file: %v", err)
	}

	if _, err := env.db.Exec(
		`INSERT INTO files (id, user_id, guest_session_id, internal_name, original_name, size, mime_type, format_family, detected_extension, metadata, uploaded_at)
		 VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fileID.String(), userID, "fixture", "fixture.txt", 14, "text/plain", "document", "txt", `{}`, now.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert file row: %v", err)
	}
	if _, err := env.db.Exec(
		`INSERT INTO jobs (id, user_id, file_id, capability_id, output_format, status, progress, created_at, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		jobID.String(), userID, fileID.String(), "doc-to-txt", "txt", string(domain.JobSucceeded), 100, now.Add(-2*time.Minute).Format(time.RFC3339Nano), now.Add(-90*time.Second).Format(time.RFC3339Nano), now.Add(-time.Minute).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert job row: %v", err)
	}
	if _, err := env.db.Exec(
		`INSERT INTO artifacts (id, user_id, job_id, file_id, file_name, mime_type, size, storage_path, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifactID.String(), userID, jobID.String(), fileID.String(), "audit.txt", "text/plain", 14, artifactPath, now.Format(time.RFC3339Nano), now.Add(2*time.Hour).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert artifact row: %v", err)
	}

	downloadResp, body := downloadArtifactClient(client, env.server.URL, artifactID.String())
	if downloadResp.StatusCode != http.StatusOK {
		t.Fatalf("download artifact: expected 200, got %d", downloadResp.StatusCode)
	}
	if string(body) != "audit-download" {
		t.Fatalf("unexpected downloaded body: %q", string(body))
	}
	if got := downloadResp.Header.Get("Content-Length"); got != "14" {
		t.Fatalf("expected Content-Length 14, got %q", got)
	}

	resp, auditData := doGetClient(client, env.server.URL+"/api/admin/audit?eventType=artifact_downloaded&limit=20", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin audit artifact_downloaded: expected 200, got %d — %v", resp.StatusCode, auditData)
	}
	total, _ := auditData["total"].(float64)
	if total < 1 {
		t.Fatalf("expected at least one artifact_downloaded event, got %v", total)
	}
}

func TestE2E_ArtifactDownloadRejectsTraversalFileName(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "TraversalUser", "artifact-traversal@test.com")
	resp, me := doGetClient(client, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth me: expected 200, got %d — %v", resp.StatusCode, me)
	}
	userID, _ := me["id"].(string)
	if userID == "" {
		t.Fatal("expected user id in /auth/me response")
	}

	now := time.Now().UTC()
	fileID := uuid.New()
	jobID := uuid.New()
	artifactID := uuid.New()
	artifactDir := filepath.Join(env.tmpDir, "storage", "artifacts", artifactID.String())
	if err := os.MkdirAll(artifactDir, 0o750); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "safe.txt"), []byte("safe"), 0o600); err != nil {
		t.Fatalf("write safe artifact file: %v", err)
	}
	outsidePath := filepath.Join(env.tmpDir, "storage", "artifacts", "outside.txt")
	if err := os.WriteFile(outsidePath, []byte("outside-secret"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	if _, err := env.db.Exec(
		`INSERT INTO files (id, user_id, guest_session_id, internal_name, original_name, size, mime_type, format_family, detected_extension, metadata, uploaded_at)
		 VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fileID.String(), userID, "fixture", "fixture.txt", 4, "text/plain", "document", "txt", `{}`, now.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert file row: %v", err)
	}
	if _, err := env.db.Exec(
		`INSERT INTO jobs (id, user_id, file_id, capability_id, output_format, status, progress, created_at, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		jobID.String(), userID, fileID.String(), "doc-to-txt", "txt", string(domain.JobSucceeded), 100, now.Add(-2*time.Minute).Format(time.RFC3339Nano), now.Add(-90*time.Second).Format(time.RFC3339Nano), now.Add(-time.Minute).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert job row: %v", err)
	}
	if _, err := env.db.Exec(
		`INSERT INTO artifacts (id, user_id, job_id, file_id, file_name, mime_type, size, storage_path, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifactID.String(), userID, jobID.String(), fileID.String(), "../outside.txt", "text/plain", 4, outsidePath, now.Format(time.RFC3339Nano), now.Add(2*time.Hour).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert artifact row: %v", err)
	}

	downloadResp, body := downloadArtifactClient(client, env.server.URL, artifactID.String())
	if downloadResp.StatusCode != http.StatusGone {
		t.Fatalf("download traversal artifact: expected 410, got %d — body: %s", downloadResp.StatusCode, string(body))
	}
	if strings.Contains(string(body), "outside-secret") {
		t.Fatal("path traversal response leaked outside file content")
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

	lastOrder := -1
	for _, item := range caps {
		capability, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("capabilities: expected object, got %T", item)
		}
		order, ok := capability["presentationOrder"].(float64)
		if !ok {
			t.Fatalf("capabilities: expected numeric presentationOrder, got %v", capability["presentationOrder"])
		}
		if int(order) < lastOrder {
			t.Fatalf("capabilities: expected ascending presentationOrder, got %d after %d", int(order), lastOrder)
		}
		lastOrder = int(order)
	}
}

func TestE2E_LegacyDocFixtureResolvesCapabilities(t *testing.T) {
	if !capabilities.DefaultProber.IsAvailable("libreoffice") {
		t.Skip("libreoffice not available")
	}

	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "LegacyDocUser", "legacy-doc@test.com")
	resp, fileData := uploadFixtureClient(client, env.server.URL, "", fixturePath("doc", "valid-basic.doc"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload legacy doc fixture: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	format, _ := fileData["detectedFormat"].(map[string]interface{})
	if format["mimeType"] != "application/msword" {
		t.Fatalf("expected application/msword detection, got %v", format)
	}

	fileID := fileData["id"].(string)
	resp, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("legacy doc capabilities: expected 200, got %d — %v", resp.StatusCode, capsData)
	}
	requireCapabilityID(t, capsData, "doc-to-pdf")
	requireCapabilityID(t, capsData, "doc-to-docx")
	requireCapabilityID(t, capsData, "doc-to-txt")
}

func TestE2E_ControlledZipBombFixtureRejectedOnUpload(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "ZipBombUser", "zip-bomb@test.com")
	resp, fileData := uploadFixtureClient(client, env.server.URL, "", fixturePath("security", "zip-bomb-controlled.docx"))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("controlled zip bomb upload: expected 422, got %d — %v", resp.StatusCode, fileData)
	}
}

func TestE2E_ProtectedODFFixtureRejectedOnUpload(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "ProtectedODFUser", "protected-odf@test.com")
	resp, fileData := uploadFixtureClient(client, env.server.URL, "", fixturePath("protected", "odf-encrypted-manifest.odt"))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("protected odf upload: expected 422, got %d — %v", resp.StatusCode, fileData)
	}
	if fileData["error"] != "protected or encrypted files not supported" {
		t.Fatalf("expected protected-file error, got %v", fileData)
	}
}

func TestE2E_PNGCapabilitiesKeepDistinctSameTargetVariants(t *testing.T) {
	if !capabilities.DefaultProber.IsAvailable("ffmpeg") || !capabilities.DefaultProber.IsAvailable("tesseract") {
		t.Skip("ffmpeg and tesseract runtimes are required for PNG capability breadth")
	}

	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "OptionsPNG", "png-options@test.com")

	_, fileData := uploadPNGClient(client, env.server.URL, "")
	fileID := fileData["id"].(string)

	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")

	for _, capabilityID := range []string{
		"image-to-jpg",
		"image-compress-png",
		"image-thumbnail-png",
		"image-web-jpg-1600",
		"image-ocr-to-txt",
	} {
		requireCapabilityID(t, capsData, capabilityID)
	}
}

func TestE2E_PDFCapabilitiesKeepTextAndOCRDistinct(t *testing.T) {
	if !capabilities.DefaultProber.IsAvailable("poppler") || !capabilities.DefaultProber.IsAvailable("ocr-pdf") {
		t.Skip("poppler and ocr-pdf runtimes are required for PDF OCR capability coverage")
	}

	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "OptionsPDF", "pdf-options@test.com")
	pdfData := []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF")

	resp, fileData := uploadRawFileClient(client, env.server.URL, "", "sample.pdf", pdfData)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("pdf upload: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID := fileData["id"].(string)

	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")

	requireCapabilityID(t, capsData, "pdf-to-txt")
	requireCapabilityID(t, capsData, "pdf-ocr-to-txt")
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

	client := newCookieClient(t)

	resp, fileData := uploadPNGClient(client, env.server.URL, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("anonymous upload: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID := fileData["id"].(string)

	resp, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("anonymous capabilities: expected 200, got %d — %v", resp.StatusCode, capsData)
	}
	caps := capsData["capabilities"].([]interface{})
	if len(caps) == 0 {
		t.Fatal("anonymous capabilities: expected at least one capability")
	}
	capID := caps[0].(map[string]interface{})["id"].(string)

	resp, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("anonymous conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}

	resp, jobStatus := doGetClient(client, fmt.Sprintf("%s/api/jobs/%s", env.server.URL, jobData["id"].(string)), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("anonymous job status: expected 200, got %d — %v", resp.StatusCode, jobStatus)
	}
	if jobStatus["status"] != "queued" {
		t.Fatalf("expected queued anonymous job, got %v", jobStatus["status"])
	}
}

func TestE2E_AnonymousResourcesAreIsolatedPerGuestSession(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	guest1 := newCookieClient(t)
	guest2 := newCookieClient(t)

	resp, fileData := uploadPNGClient(guest1, env.server.URL, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("guest upload: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID := fileData["id"].(string)

	resp, _ = doGetClient(guest2, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-guest file access, got %d", resp.StatusCode)
	}

	resp, capsData := doGetClient(guest1, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("guest capabilities: expected 200, got %d — %v", resp.StatusCode, capsData)
	}
	capID := capsData["capabilities"].([]interface{})[0].(map[string]interface{})["id"].(string)

	resp, jobData := doPostClient(guest1, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("guest conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}

	resp, _ = doGetClient(guest2, fmt.Sprintf("%s/api/jobs/%s", env.server.URL, jobData["id"].(string)), "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-guest job access, got %d", resp.StatusCode)
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
	if engData["capabilities"] == nil {
		t.Fatal("admin engines: expected capabilities field")
	}
	if engData["availableCapabilities"] == nil || engData["totalCapabilities"] == nil {
		t.Fatal("admin engines: expected capabilities summary fields")
	}

	capabilitiesList, ok := engData["capabilities"].([]interface{})
	if !ok || len(capabilitiesList) == 0 {
		t.Fatalf("admin engines: expected non-empty capabilities list, got %T", engData["capabilities"])
	}
	firstCapability, ok := capabilitiesList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("admin engines: expected capability object, got %T", capabilitiesList[0])
	}
	for _, field := range []string{"id", "displayName", "engine", "family", "available", "reason"} {
		if _, found := firstCapability[field]; !found {
			t.Fatalf("admin engines: expected capability field %q", field)
		}
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
	disposition, params, err := mime.ParseMediaType(downloadResp.Header.Get("Content-Disposition"))
	if err != nil {
		t.Fatalf("parse content disposition: %v", err)
	}
	if disposition != "attachment" {
		t.Fatalf("expected attachment disposition, got %q", disposition)
	}
	if params["filename"] != "converted.png" {
		t.Fatalf("expected png download filename, got %q", downloadResp.Header.Get("Content-Disposition"))
	}
	if len(body) < 8 || !bytes.Equal(body[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("unexpected png artifact header: %q", body[:min(len(body), 12)])
	}
}

func TestE2E_RealUploadConvertDownloadPNGThumbnail(t *testing.T) {
	env := setupE2EWithWorker(t)
	defer env.close()

	client := registerUserClient(t, env, "ImageUser", "image-real@test.com")
	resp, fileData := uploadNoisyPNGClient(client, env.server.URL, "", 640)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload png fixture: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID := fileData["id"].(string)

	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	capID := requireCapabilityID(t, capsData, "image-thumbnail-png")

	resp, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create png thumbnail conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}

	terminal := waitForTerminalJob(t, client, env.server.URL, jobData["id"].(string))
	if terminal["status"] != "succeeded" {
		t.Fatalf("expected succeeded thumbnail job, got %v", terminal)
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
		t.Fatalf("download png thumbnail artifact: expected 200, got %d", downloadResp.StatusCode)
	}
	decoded, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("decode thumbnail png artifact: %v", err)
	}
	if decoded.Width > 320 || decoded.Height > 320 {
		t.Fatalf("expected thumbnail <= 320px, got %dx%d", decoded.Width, decoded.Height)
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

func TestE2E_UploadPolicyAppliesDifferentLimitsForGuestsAndRegisteredUsers(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	adminClient := registerUserClient(t, env, "UploadAdmin", "upload-admin@test.com")
	resp, data := doPutClient(adminClient, env.server.URL+"/api/admin/upload-policy", map[string]int64{
		"guestMaxBytes":      1 * 1024 * 1024,
		"registeredMaxBytes": 10 * 1024 * 1024,
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set upload policy: expected 200, got %d — %v", resp.StatusCode, data)
	}

	resp, data = uploadNoisyPNGClient(nil, env.server.URL, "", 1024)
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("guest upload should hit upload policy, got %d — %v", resp.StatusCode, data)
	}
	if data["error"] != "file exceeds size limit" {
		t.Fatalf("unexpected guest upload error: %v", data["error"])
	}

	resp, data = uploadNoisyPNGClient(adminClient, env.server.URL, "", 1024)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("registered upload should respect higher limit, got %d — %v", resp.StatusCode, data)
	}
}

func TestE2E_MaxActiveJobsPerUser(t *testing.T) {
	env := setupE2EWithLimits(t, e2eLimits{
		userUploadsPerMinute:     60,
		userUploadBurst:          10,
		userConversionsPerMinute: 60,
		userConversionBurst:      10,
		maxActiveJobsPerUser:     1,
		maxActiveJobsPerGuest:    1,
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

func TestE2E_MaxActiveJobsPerGuestSession(t *testing.T) {
	env := setupE2EWithLimits(t, e2eLimits{
		userUploadsPerMinute:     60,
		userUploadBurst:          10,
		userConversionsPerMinute: 60,
		userConversionBurst:      10,
		maxActiveJobsPerUser:     10,
		maxActiveJobsPerGuest:    1,
	})
	defer env.close()

	client := newCookieClient(t)

	_, fileData1 := uploadPNGClient(client, env.server.URL, "")
	fileID1 := fileData1["id"].(string)
	_, capsData1 := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID1), "")
	capID1 := capsData1["capabilities"].([]interface{})[0].(map[string]interface{})["id"].(string)

	resp, _ := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID1,
		"capabilityId": capID1,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first guest job: expected 201, got %d", resp.StatusCode)
	}

	_, fileData2 := uploadPNGClient(client, env.server.URL, "")
	fileID2 := fileData2["id"].(string)
	_, capsData2 := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID2), "")
	capID2 := capsData2["capabilities"].([]interface{})[0].(map[string]interface{})["id"].(string)

	resp, data := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID2,
		"capabilityId": capID2,
	}, "")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second guest job should be blocked, got %d — %v", resp.StatusCode, data)
	}
	if data["error"] != "too many active jobs for this guest session" {
		t.Fatalf("unexpected guest active job error: %v", data["error"])
	}
}

func TestE2E_RetryFailedGuestJobRespectsActiveJobLimit(t *testing.T) {
	env := setupE2EWithLimits(t, e2eLimits{
		userUploadsPerMinute:     60,
		userUploadBurst:          10,
		userConversionsPerMinute: 60,
		userConversionBurst:      10,
		maxActiveJobsPerUser:     10,
		maxActiveJobsPerGuest:    1,
	})
	defer env.close()

	client := newCookieClient(t)
	_, fileData := uploadPNGClient(client, env.server.URL, "")
	fileID := fileData["id"].(string)
	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	capID := capsData["capabilities"].([]interface{})[0].(map[string]interface{})["id"].(string)

	resp, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("guest job: expected 201, got %d — %v", resp.StatusCode, jobData)
	}
	jobID := jobData["id"].(string)
	parsedJobID := uuid.MustParse(jobID)
	if err := env.orch.MarkRunning(context.Background(), parsedJobID); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	if err := env.orch.MarkFailed(context.Background(), parsedJobID, "simulated worker failure"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	resp, retryData := doPostClient(client, fmt.Sprintf("%s/api/jobs/%s/retry", env.server.URL, jobID), nil, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first retry: expected 201, got %d — %v", resp.StatusCode, retryData)
	}
	resp, retryData = doPostClient(client, fmt.Sprintf("%s/api/jobs/%s/retry", env.server.URL, jobID), nil, "")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second retry should be blocked by guest active limit, got %d — %v", resp.StatusCode, retryData)
	}
}

func TestE2E_RetryFailedJobReturnsGoneWhenOriginalMissing(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	client := registerUserClient(t, env, "ExpiredRetryUser", "expired-retry@test.com")
	_, fileData := uploadPNGClient(client, env.server.URL, "")
	fileID := fileData["id"].(string)
	_, capsData := doGetClient(client, fmt.Sprintf("%s/api/files/%s/capabilities", env.server.URL, fileID), "")
	capID := capsData["capabilities"].([]interface{})[0].(map[string]interface{})["id"].(string)

	resp, jobData := doPostClient(client, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("job: expected 201, got %d — %v", resp.StatusCode, jobData)
	}
	jobID := jobData["id"].(string)
	parsedJobID := uuid.MustParse(jobID)
	if err := env.orch.MarkRunning(context.Background(), parsedJobID); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	if err := env.orch.MarkFailed(context.Background(), parsedJobID, "simulated worker failure"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	originalPath := filepath.Join(env.tmpDir, "storage", "originals", fileID, "data")
	if err := os.Remove(originalPath); err != nil {
		t.Fatalf("remove original: %v", err)
	}

	resp, retryData := doPostClient(client, fmt.Sprintf("%s/api/jobs/%s/retry", env.server.URL, jobID), nil, "")
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("retry should return 410 when original is missing, got %d — %v", resp.StatusCode, retryData)
	}
}

// ── Email / SMTP E2E Tests ──

func doGetClientList(client *http.Client, url, token string) (*http.Response, []map[string]interface{}) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "reform_session", Value: token})
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()
	var list []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&list)
	return resp, list
}

func TestE2E_AdminSMTPSettings(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	adminClient := registerUserClient(t, env, "SMTPAdmin", "smtp-admin@test.com")
	userClient := registerUserClient(t, env, "SMTPUser", "smtp-user@test.com")

	// Anonymous cannot access SMTP settings.
	resp, _ := doGet(env.server.URL+"/api/admin/smtp-settings", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous SMTP get, got %d", resp.StatusCode)
	}

	// Non-admin cannot access SMTP settings.
	resp, _ = doGetClient(userClient, env.server.URL+"/api/admin/smtp-settings", "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin SMTP get, got %d", resp.StatusCode)
	}

	// Admin can get SMTP settings (initially empty / source=none).
	resp, data := doGetClient(adminClient, env.server.URL+"/api/admin/smtp-settings", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin SMTP get: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["source"] != "none" {
		t.Fatalf("expected source=none, got %v", data["source"])
	}

	// Admin can update SMTP settings.
	useTLS := true
	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/smtp-settings", map[string]interface{}{
		"host":     "mail.example.com",
		"port":     587,
		"user":     "testuser",
		"password": "secret",
		"from":     "noreply@example.com",
		"use_tls":  useTLS,
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin SMTP update: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["status"] != "saved" {
		t.Fatalf("expected status=saved, got %v", data["status"])
	}

	// Verify settings were persisted (source=admin, password masked).
	resp, data = doGetClient(adminClient, env.server.URL+"/api/admin/smtp-settings", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin SMTP get after update: expected 200, got %d", resp.StatusCode)
	}
	if data["source"] != "admin" {
		t.Fatalf("expected source=admin, got %v", data["source"])
	}
	if data["host"] != "mail.example.com" {
		t.Fatalf("expected host=mail.example.com, got %v", data["host"])
	}
	if data["password"] != "****" {
		t.Fatalf("expected masked password, got %v", data["password"])
	}

	// Non-admin cannot update SMTP settings.
	resp, _ = doPutClient(userClient, env.server.URL+"/api/admin/smtp-settings", map[string]interface{}{
		"host": "evil.com",
		"port": 25,
	}, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin SMTP update, got %d", resp.StatusCode)
	}

	// Invalid port rejected.
	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/smtp-settings", map[string]interface{}{
		"host": "mail.example.com",
		"port": 99999,
	}, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid port, got %d — %v", resp.StatusCode, data)
	}

	// Invalid from email rejected.
	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/smtp-settings", map[string]interface{}{
		"host": "mail.example.com",
		"port": 587,
		"from": "not-an-email",
	}, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid from, got %d — %v", resp.StatusCode, data)
	}
}

func TestE2E_AdminEmailTemplates(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	adminClient := registerUserClient(t, env, "TmplAdmin", "tmpl-admin@test.com")
	userClient := registerUserClient(t, env, "TmplUser", "tmpl-user@test.com")

	// Anonymous cannot list templates.
	resp, _ := doGet(env.server.URL+"/api/admin/email-templates", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous template list, got %d", resp.StatusCode)
	}

	// Non-admin cannot list templates.
	resp, _ = doGetClient(userClient, env.server.URL+"/api/admin/email-templates", "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin template list, got %d", resp.StatusCode)
	}

	// Admin can list templates (seeded by migration 006).
	resp, list := doGetClientList(adminClient, env.server.URL+"/api/admin/email-templates", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin template list: expected 200, got %d", resp.StatusCode)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one seeded template")
	}

	templateKey := list[0]["key"].(string)

	// Admin can get a single template.
	resp, data := doGetClient(adminClient, env.server.URL+"/api/admin/email-templates/"+templateKey, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin template get: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["key"] != templateKey {
		t.Fatalf("expected key=%s, got %v", templateKey, data["key"])
	}

	// 404 for non-existent template.
	resp, _ = doGetClient(adminClient, env.server.URL+"/api/admin/email-templates/nonexistent", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent template, got %d", resp.StatusCode)
	}

	// Admin can update a template.
	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/email-templates/"+templateKey, map[string]string{
		"subject":   "Updated: {{.Name}}",
		"body_html": "<h1>Hello {{.Name}}</h1><p>Welcome!</p>",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin template update: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if data["subject"] != "Updated: {{.Name}}" {
		t.Fatalf("expected updated subject, got %v", data["subject"])
	}

	// Non-admin cannot update template.
	resp, _ = doPutClient(userClient, env.server.URL+"/api/admin/email-templates/"+templateKey, map[string]string{
		"subject":   "Hacked",
		"body_html": "<p>hacked</p>",
	}, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin template update, got %d", resp.StatusCode)
	}

	// Validation: empty subject rejected.
	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/email-templates/"+templateKey, map[string]string{
		"subject":   "",
		"body_html": "<p>test</p>",
	}, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty subject, got %d — %v", resp.StatusCode, data)
	}

	// Validation: empty body rejected.
	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/email-templates/"+templateKey, map[string]string{
		"subject":   "Test",
		"body_html": "",
	}, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d — %v", resp.StatusCode, data)
	}

	// Validation: invalid Go template syntax in subject rejected.
	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/email-templates/"+templateKey, map[string]string{
		"subject":   "Bad {{.Unclosed",
		"body_html": "<p>ok</p>",
	}, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid template syntax, got %d — %v", resp.StatusCode, data)
	}

	// Admin can preview a template.
	resp, data = doPostClient(adminClient, env.server.URL+"/api/admin/email-templates/"+templateKey+"/preview", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin template preview: expected 200, got %d — %v", resp.StatusCode, data)
	}
	htmlContent, _ := data["html"].(string)
	if htmlContent == "" {
		t.Fatal("expected non-empty rendered HTML in preview")
	}
	subjectContent, _ := data["subject"].(string)
	if subjectContent == "" {
		t.Fatal("expected non-empty rendered subject in preview")
	}
	// Preview should have the example variables rendered.
	if !strings.Contains(subjectContent, "Usuario de Ejemplo") {
		t.Fatalf("expected preview subject to contain rendered name, got %q", subjectContent)
	}
}

func TestE2E_AdminWebhookCRUD(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	adminClient := registerUserClient(t, env, "WebhookAdmin", "webhook-admin@test.com")
	userClient := registerUserClient(t, env, "RegularUser", "regular@test.com")

	// Non-admin cannot list webhooks.
	resp, _ := doGetClient(userClient, env.server.URL+"/api/admin/webhooks", "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin webhook list, got %d", resp.StatusCode)
	}

	// Admin lists webhooks — initially empty.
	resp, webhooks := doGetArrayClient(adminClient, env.server.URL+"/api/admin/webhooks", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin webhook list: expected 200, got %d", resp.StatusCode)
	}

	// Admin creates a webhook.
	resp, data := doPostClient(adminClient, env.server.URL+"/api/admin/webhooks", map[string]interface{}{
		"url":        "https://example.com/hook",
		"secret":     "s3cret",
		"eventTypes": []string{"job.completed", "job.failed"},
		"enabled":    true,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("admin webhook create: expected 201, got %d — %v", resp.StatusCode, data)
	}
	webhookID, _ := data["id"].(string)
	if webhookID == "" {
		t.Fatal("expected webhook id in create response")
	}

	// Non-admin cannot create a webhook.
	resp, _ = doPostClient(userClient, env.server.URL+"/api/admin/webhooks", map[string]interface{}{
		"url":        "https://evil.com/hook",
		"eventTypes": []string{"job.completed"},
		"enabled":    true,
	}, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin webhook create, got %d", resp.StatusCode)
	}

	// Admin lists again — should have one webhook.
	resp, webhooks = doGetArrayClient(adminClient, env.server.URL+"/api/admin/webhooks", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin webhook list: expected 200, got %d", resp.StatusCode)
	}
	if len(webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(webhooks))
	}

	// Admin updates the webhook.
	resp, data = doPutClient(adminClient, env.server.URL+"/api/admin/webhooks/"+webhookID, map[string]interface{}{
		"url":        "https://example.com/hook-v2",
		"eventTypes": []string{"job.completed"},
		"enabled":    false,
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin webhook update: expected 200, got %d — %v", resp.StatusCode, data)
	}

	// Non-admin cannot update.
	resp, _ = doPutClient(userClient, env.server.URL+"/api/admin/webhooks/"+webhookID, map[string]interface{}{
		"url":        "https://evil.com/hook",
		"eventTypes": []string{"job.completed"},
		"enabled":    true,
	}, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin webhook update, got %d", resp.StatusCode)
	}

	// Non-admin cannot delete.
	resp, _ = doDeleteClient(userClient, env.server.URL+"/api/admin/webhooks/"+webhookID, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin webhook delete, got %d", resp.StatusCode)
	}

	// Admin deletes the webhook.
	resp, _ = doDeleteClient(adminClient, env.server.URL+"/api/admin/webhooks/"+webhookID, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin webhook delete: expected 200, got %d", resp.StatusCode)
	}

	// List again — should be empty.
	resp, webhooks = doGetArrayClient(adminClient, env.server.URL+"/api/admin/webhooks", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin webhook list after delete: expected 200, got %d", resp.StatusCode)
	}
	if len(webhooks) != 0 {
		t.Fatalf("expected 0 webhooks after delete, got %d", len(webhooks))
	}
}

func TestE2E_AdminJobsList(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	adminClient := registerUserClient(t, env, "JobsAdmin", "jobs-admin@test.com")
	userClient := registerUserClient(t, env, "RegularUser", "regular-jobs@test.com")

	// Non-admin cannot list jobs.
	resp, _ := doGetClient(userClient, env.server.URL+"/api/admin/jobs", "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin jobs list, got %d", resp.StatusCode)
	}

	// Admin can list jobs — initially empty.
	resp, data := doGetClient(adminClient, env.server.URL+"/api/admin/jobs", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin jobs list: expected 200, got %d — %v", resp.StatusCode, data)
	}
	total, _ := data["total"].(float64)
	if total != 0 {
		t.Fatalf("expected 0 total jobs, got %v", total)
	}

	// Upload a file so we can create a job.
	resp, fileData := uploadPNGClient(adminClient, env.server.URL, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID, _ := fileData["id"].(string)
	caps, _ := fileData["capabilities"].([]interface{})
	if len(caps) == 0 {
		t.Skip("no capabilities for PNG, skipping job creation")
	}
	firstCap := caps[0].(map[string]interface{})
	capID, _ := firstCap["id"].(string)
	outputFormat, _ := firstCap["outputFormat"].(string)

	// Create a conversion job.
	resp, createdJob := doPostClient(adminClient, env.server.URL+"/api/files/"+fileID+"/convert", map[string]interface{}{
		"capabilityId": capID,
		"outputFormat": outputFormat,
	}, "")
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		t.Fatalf("convert: expected 201 or 202, got %d", resp.StatusCode)
	}
	jobID, _ := createdJob["id"].(string)
	if jobID == "" {
		t.Fatal("convert: expected response id")
	}

	oldCreatedAt := time.Now().Add(-45 * time.Minute).UTC().Format(time.RFC3339Nano)
	if _, err := env.db.Exec(
		`UPDATE jobs
		 SET created_at = ?, started_at = NULL, completed_at = NULL
		 WHERE id = ?`,
		oldCreatedAt,
		jobID,
	); err != nil {
		t.Fatalf("age queued job for stalled filter: %v", err)
	}

	// Admin can list jobs — should now have one.
	resp, data = doGetClient(adminClient, env.server.URL+"/api/admin/jobs", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin jobs list: expected 200, got %d — %v", resp.StatusCode, data)
	}
	total, _ = data["total"].(float64)
	if total < 1 {
		t.Fatalf("expected at least 1 total job, got %v", total)
	}
	if data["stalledJobs"] == nil || data["stalledQueuedJobs"] == nil || data["stalledRunningJobs"] == nil {
		t.Fatal("admin jobs list: expected stalled summary fields")
	}
	rows, ok := data["jobs"].([]interface{})
	if !ok || len(rows) == 0 {
		t.Fatalf("admin jobs list: expected non-empty jobs array, got %T", data["jobs"])
	}
	firstJob, ok := rows[0].(map[string]interface{})
	if !ok {
		t.Fatalf("admin jobs list: expected job object, got %T", rows[0])
	}
	for _, field := range []string{"stalled", "backlogAgeSec"} {
		if _, found := firstJob[field]; !found {
			t.Fatalf("admin jobs list: expected job field %q", field)
		}
	}

	// Test status filter.
	resp, data = doGetClient(adminClient, env.server.URL+"/api/admin/jobs?status=queued", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin jobs list filtered: expected 200, got %d", resp.StatusCode)
	}

	// Test stalled-only filter.
	resp, data = doGetClient(adminClient, env.server.URL+"/api/admin/jobs?stalled=true", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin jobs list stalled-only: expected 200, got %d", resp.StatusCode)
	}
	rows, _ = data["jobs"].([]interface{})
	if len(rows) < 1 {
		t.Fatal("expected at least 1 stalled job")
	}
	firstJob, _ = rows[0].(map[string]interface{})
	if firstJob["stalled"] != true {
		t.Fatalf("expected stalled=true in stalled-only filter, got %v", firstJob["stalled"])
	}

	// Test search filter.
	resp, data = doGetClient(adminClient, env.server.URL+"/api/admin/jobs?q=JobsAdmin", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin jobs list search: expected 200, got %d", resp.StatusCode)
	}
	total, _ = data["total"].(float64)
	if total < 1 {
		t.Fatalf("expected at least 1 job matching search, got %v", total)
	}
}

func TestE2E_AdminJobsBatchActions(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	adminClient := registerUserClient(t, env, "BatchAdmin", "batch-admin@test.com")

	resp, fileData := uploadPNGClient(adminClient, env.server.URL, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d", resp.StatusCode)
	}
	fileID, _ := fileData["id"].(string)
	caps, _ := fileData["capabilities"].([]interface{})
	if len(caps) == 0 {
		t.Skip("no capabilities for PNG, skipping batch admin actions")
	}
	firstCap := caps[0].(map[string]interface{})
	capID, _ := firstCap["id"].(string)
	outputFormat, _ := firstCap["outputFormat"].(string)

	createJob := func() string {
		resp, createdJob := doPostClient(adminClient, env.server.URL+"/api/files/"+fileID+"/convert", map[string]interface{}{
			"capabilityId": capID,
			"outputFormat": outputFormat,
		}, "")
		if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
			t.Fatalf("convert: expected 201 or 202, got %d", resp.StatusCode)
		}
		jobID, _ := createdJob["id"].(string)
		if jobID == "" {
			t.Fatal("expected job id")
		}
		return jobID
	}

	queuedJobID := createJob()
	failedJobID := createJob()
	if _, err := env.db.Exec(
		`UPDATE jobs SET status = ?, progress = 0, error = ?, completed_at = ? WHERE id = ?`,
		string(domain.JobFailed), "forced failure for retry", time.Now().UTC().Format(time.RFC3339Nano), failedJobID,
	); err != nil {
		t.Fatalf("mark job failed: %v", err)
	}

	resp, data := doPostClient(adminClient, env.server.URL+"/api/admin/jobs/batch/cancel", map[string]interface{}{
		"jobIds": []string{queuedJobID},
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin batch cancel: expected 200, got %d — %v", resp.StatusCode, data)
	}
	cancelledIDs, _ := data["cancelledJobIds"].([]interface{})
	if len(cancelledIDs) != 1 {
		t.Fatalf("expected 1 cancelled job, got %d", len(cancelledIDs))
	}

	resp, data = doPostClient(adminClient, env.server.URL+"/api/admin/jobs/batch/retry", map[string]interface{}{
		"filter": map[string]interface{}{"status": "failed"},
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("admin batch retry by filter: expected 201, got %d — %v", resp.StatusCode, data)
	}
	jobs, _ := data["jobs"].([]interface{})
	if len(jobs) < 1 {
		t.Fatal("expected at least one retried job")
	}
	var cancelledStatus string
	if err := env.db.QueryRow(`SELECT status FROM jobs WHERE id = ?`, queuedJobID).Scan(&cancelledStatus); err != nil {
		t.Fatalf("load cancelled job: %v", err)
	}
	if cancelledStatus != string(domain.JobCancelled) {
		t.Fatalf("expected cancelled status, got %q", cancelledStatus)
	}
}

func TestE2E_AdminUsersList(t *testing.T) {
	env := setupE2E(t)
	defer env.server.Close()

	adminClient := registerUserClient(t, env, "adminUser", "admin-users@test.com")
	normalClient := registerUserClient(t, env, "normalUser", "normal-users@test.com")

	// Non-admin cannot list users.
	resp, _ := doGetClient(normalClient, env.server.URL+"/api/admin/users", "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin list users: expected 403, got %d", resp.StatusCode)
	}

	// Admin can list users.
	resp2, data := doGetClient(adminClient, env.server.URL+"/api/admin/users", "")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("admin list users: expected 200, got %d", resp2.StatusCode)
	}
	usersArr, _ := data["users"].([]interface{})
	if len(usersArr) != 2 {
		t.Fatalf("expected 2 users, got %d", len(usersArr))
	}
	total, _ := data["total"].(float64)
	if int(total) != 2 {
		t.Fatalf("expected total=2 users, got %v", total)
	}

	// Admin can filter users by search.
	resp2, data = doGetClient(adminClient, env.server.URL+"/api/admin/users?q=normal-users", "")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("admin list users search: expected 200, got %d", resp2.StatusCode)
	}
	usersArr, _ = data["users"].([]interface{})
	if len(usersArr) != 1 {
		t.Fatalf("expected 1 user from search filter, got %d", len(usersArr))
	}

	// Admin can paginate users.
	resp2, data = doGetClient(adminClient, env.server.URL+"/api/admin/users?limit=1&offset=0", "")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("admin list users pagination: expected 200, got %d", resp2.StatusCode)
	}
	usersArr, _ = data["users"].([]interface{})
	if len(usersArr) != 1 {
		t.Fatalf("expected 1 user on paginated page, got %d", len(usersArr))
	}

	// Find the normal user's ID.
	var normalUserID string
	var adminUserID string
	for _, u := range usersArr {
		m := u.(map[string]interface{})
		if m["email"] == "normal-users@test.com" {
			normalUserID = m["id"].(string)
		}
		if m["email"] == "admin-users@test.com" {
			adminUserID = m["id"].(string)
		}
	}
	if normalUserID == "" {
		t.Fatal("could not find normal user in list")
	}

	// Non-admin cannot change roles.
	resp, _ = doPatchClient(normalClient, env.server.URL+"/api/admin/users/"+normalUserID+"/role",
		map[string]string{"role": "admin"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin update role: expected 403, got %d", resp.StatusCode)
	}

	// Admin can promote user.
	resp, _ = doPatchClient(adminClient, env.server.URL+"/api/admin/users/"+normalUserID+"/role",
		map[string]string{"role": "admin"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin promote user: expected 200, got %d", resp.StatusCode)
	}

	// Role filter should now include both admins.
	resp2, data = doGetClient(adminClient, env.server.URL+"/api/admin/users?role=admin", "")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("admin list users role=admin: expected 200, got %d", resp2.StatusCode)
	}
	usersArr, _ = data["users"].([]interface{})
	if len(usersArr) != 2 {
		t.Fatalf("expected 2 admins after promotion, got %d", len(usersArr))
	}

	// Admin cannot demote themselves.
	resp, _ = doPatchClient(adminClient, env.server.URL+"/api/admin/users/"+adminUserID+"/role",
		map[string]string{"role": "user"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("admin self-demote: expected 400, got %d", resp.StatusCode)
	}

	// Admin can demote another admin (the previously promoted user).
	resp, _ = doPatchClient(adminClient, env.server.URL+"/api/admin/users/"+normalUserID+"/role",
		map[string]string{"role": "user"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin demote user: expected 200, got %d", resp.StatusCode)
	}
}

func TestE2E_AdminUserSuspensionAndSessionRevocation(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	adminClient := registerUserClient(t, env, "SecurityAdmin", "security-admin@test.com")
	targetClient := registerUserClient(t, env, "SecurityUser", "security-user@test.com")

	resp, data := doGetClient(adminClient, env.server.URL+"/api/admin/users?q=security-user", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin users list: expected 200, got %d", resp.StatusCode)
	}
	usersArr, _ := data["users"].([]interface{})
	if len(usersArr) != 1 {
		t.Fatalf("expected exactly one matching user, got %d", len(usersArr))
	}
	targetID := usersArr[0].(map[string]interface{})["id"].(string)

	resp, _ = doPatchClient(adminClient, env.server.URL+"/api/admin/users/"+targetID+"/suspension", map[string]interface{}{
		"suspended": true,
		"reason":    "abuse review",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("suspend user: expected 200, got %d", resp.StatusCode)
	}

	resp, _ = doGetClient(targetClient, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("suspended auth/me: expected 403, got %d", resp.StatusCode)
	}

	resp, _ = doPatchClient(adminClient, env.server.URL+"/api/admin/users/"+targetID+"/suspension", map[string]interface{}{
		"suspended": false,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unsuspend user: expected 200, got %d", resp.StatusCode)
	}

	resp, data = doPostClient(targetClient, env.server.URL+"/api/auth/login", map[string]interface{}{
		"email":    "security-user@test.com",
		"password": "password123",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login after unsuspend: expected 200, got %d", resp.StatusCode)
	}

	resp, data = doPostClient(adminClient, env.server.URL+"/api/admin/users/"+targetID+"/revoke-sessions", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("revoke sessions: expected 200, got %d — %v", resp.StatusCode, data)
	}
	if _, ok := data["sessionVersion"].(float64); !ok {
		t.Fatal("expected sessionVersion in revoke response")
	}

	resp, _ = doGetClient(targetClient, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked auth/me: expected 401, got %d", resp.StatusCode)
	}
}

func TestE2E_AdminDetailedHealthIncludesWorkersAndHistory(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	adminClient := registerUserClient(t, env, "HealthAdmin", "health-admin@test.com")
	workerID := uuid.New().String()
	now := time.Now().UTC()
	jobID := uuid.New()
	failureID := uuid.New()
	if _, err := env.db.Exec(
		`INSERT INTO worker_status (id, runtime_mode, queue_mode, last_heartbeat_at, last_task_type, last_job_id, last_task_status, last_task_started_at, last_task_finished_at, last_error, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		workerID, "standalone", "redis", now.Format(time.RFC3339Nano), "conversion:image-to-png", jobID.String(), "failed", now.Add(-2*time.Minute).Format(time.RFC3339Nano), now.Add(-time.Minute).Format(time.RFC3339Nano), "engine failed", now.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert worker status: %v", err)
	}
	if _, err := env.db.Exec(
		`INSERT INTO worker_failures (id, worker_id, task_type, job_id, error, failed_at) VALUES (?, ?, ?, ?, ?, ?)`,
		failureID.String(), workerID, "conversion:image-to-png", jobID.String(), "engine failed", now.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert worker failure: %v", err)
	}
	jobRowID := uuid.New()
	fileRowID := uuid.New()
	if _, err := env.db.Exec(
		`INSERT INTO files (id, user_id, guest_session_id, internal_name, original_name, size, mime_type, format_family, detected_extension, metadata, uploaded_at)
		 VALUES (?, NULL, NULL, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fileRowID.String(), "fixture", "fixture.png", 128, "image/png", "image", "png", `{}`, now.Add(-20*time.Minute).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert file row: %v", err)
	}
	if _, err := env.db.Exec(
		`INSERT INTO jobs (id, user_id, file_id, capability_id, output_format, status, progress, created_at, started_at, completed_at)
		 VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?)`,
		jobRowID.String(), fileRowID.String(), "image-to-png", "png", string(domain.JobSucceeded), 100, now.Add(-10*time.Minute).Format(time.RFC3339Nano), now.Add(-9*time.Minute).Format(time.RFC3339Nano), now.Add(-8*time.Minute).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert job row: %v", err)
	}

	resp, data := doGetClient(adminClient, env.server.URL+"/api/admin/health", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin health: expected 200, got %d — %v", resp.StatusCode, data)
	}
	runtime, _ := data["runtime"].(map[string]interface{})
	queue, _ := runtime["queue"].(map[string]interface{})
	history, _ := queue["history"].([]interface{})
	if len(history) != 3 {
		t.Fatalf("expected 3 history windows, got %d", len(history))
	}
	workers, _ := runtime["workers"].(map[string]interface{})
	workerRows, _ := workers["workers"].([]interface{})
	if len(workerRows) < 1 {
		t.Fatal("expected at least one worker row")
	}
	firstWorker := workerRows[0].(map[string]interface{})
	failures, _ := firstWorker["recentFailures"].([]interface{})
	if len(failures) < 1 {
		t.Fatal("expected at least one recent failure")
	}
}

func TestE2E_AdminSupportControls(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	adminClient := registerUserClient(t, env, "SupportAdmin", "support-admin@test.com")

	resp, fileData := uploadPNGClient(adminClient, env.server.URL, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d — %v", resp.StatusCode, fileData)
	}
	fileID, _ := fileData["id"].(string)
	caps, _ := fileData["capabilities"].([]interface{})
	if len(caps) == 0 {
		t.Skip("no capabilities for PNG in this runtime")
	}
	capID, _ := caps[0].(map[string]interface{})["id"].(string)

	resp, jobData := doPostClient(adminClient, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create conversion: expected 201, got %d — %v", resp.StatusCode, jobData)
	}
	queuedJobID, _ := jobData["id"].(string)

	resp, intakeData := doPatchClient(adminClient, env.server.URL+"/api/admin/support/queue/intake", map[string]interface{}{
		"paused": true,
		"reason": "maintenance",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pause intake: expected 200, got %d — %v", resp.StatusCode, intakeData)
	}
	if intakeData["jobIntakePaused"] != true {
		t.Fatalf("expected paused intake state, got %v", intakeData["jobIntakePaused"])
	}

	resp, blockedData := doPostClient(adminClient, env.server.URL+"/api/conversions", map[string]string{
		"fileId":       fileID,
		"capabilityId": capID,
	}, "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("conversion while paused: expected 503, got %d — %v", resp.StatusCode, blockedData)
	}

	resp, drainData := doPostClient(adminClient, env.server.URL+"/api/admin/support/queue/drain", map[string]interface{}{
		"limit": 25,
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("drain queue: expected 200, got %d — %v", resp.StatusCode, drainData)
	}
	cancelled, _ := drainData["cancelled"].(float64)
	if cancelled < 1 {
		t.Fatalf("expected at least one cancelled queued job, got %v", cancelled)
	}

	var drainedStatus string
	if err := env.db.QueryRow(`SELECT status FROM jobs WHERE id = ?`, queuedJobID).Scan(&drainedStatus); err != nil {
		t.Fatalf("load drained job status: %v", err)
	}
	if drainedStatus != string(domain.JobCancelled) {
		t.Fatalf("expected queued job cancelled after drain, got %q", drainedStatus)
	}

	resp, intakeData = doPatchClient(adminClient, env.server.URL+"/api/admin/support/queue/intake", map[string]interface{}{
		"paused": false,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resume intake: expected 200, got %d — %v", resp.StatusCode, intakeData)
	}
	if intakeData["jobIntakePaused"] != false {
		t.Fatalf("expected resumed intake state, got %v", intakeData["jobIntakePaused"])
	}

	staleWorkerID := uuid.New().String()
	staleTime := time.Now().UTC().Add(-2 * time.Hour)
	if _, err := env.db.Exec(
		`INSERT INTO worker_status (id, runtime_mode, queue_mode, last_heartbeat_at, last_task_status, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		staleWorkerID, "standalone", "redis", staleTime.Format(time.RFC3339Nano), "idle", staleTime.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert stale worker: %v", err)
	}

	resp, pruneData := doPostClient(adminClient, env.server.URL+"/api/admin/support/workers/prune-stale", map[string]interface{}{
		"staleMinutes": 60,
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("prune stale workers: expected 200, got %d — %v", resp.StatusCode, pruneData)
	}
	deleted, _ := pruneData["deleted"].(float64)
	if deleted < 1 {
		t.Fatalf("expected at least one pruned worker, got %v", deleted)
	}

	resp, healthData := doGetClient(adminClient, env.server.URL+"/api/admin/health", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin health: expected 200, got %d — %v", resp.StatusCode, healthData)
	}
	runtime, _ := healthData["runtime"].(map[string]interface{})
	queueData, _ := runtime["queue"].(map[string]interface{})
	controls, _ := queueData["controls"].(map[string]interface{})
	if controls["jobIntakePaused"] != false {
		t.Fatalf("expected health controls to report resumed intake, got %v", controls["jobIntakePaused"])
	}

	resp, auditData := doGetClient(adminClient, env.server.URL+"/api/admin/audit?group=admin&limit=100", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin audit list: expected 200, got %d — %v", resp.StatusCode, auditData)
	}
	events, _ := auditData["events"].([]interface{})
	seenPause := false
	seenResume := false
	seenDrain := false
	seenPrune := false
	for _, row := range events {
		event, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		eventType, _ := event["eventType"].(string)
		switch eventType {
		case string(domain.AuditAdminQueuePaused):
			seenPause = true
		case string(domain.AuditAdminQueueResumed):
			seenResume = true
		case string(domain.AuditAdminQueueDrained):
			seenDrain = true
		case string(domain.AuditAdminWorkersPruned):
			seenPrune = true
		}
	}
	if !seenPause || !seenResume || !seenDrain || !seenPrune {
		t.Fatalf("expected support audit events (pause/resume/drain/prune), got %v", events)
	}
}

func TestE2E_AdminAuditListAndExport(t *testing.T) {
	env := setupE2E(t)
	defer env.server.Close()

	adminClient := registerUserClient(t, env, "AuditAdmin", "audit-admin@test.com")
	normalClient := registerUserClient(t, env, "AuditUser", "audit-user@test.com")

	// Non-admin cannot access admin audit endpoints.
	resp, _ := doGetClient(normalClient, env.server.URL+"/api/admin/audit", "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin audit list: expected 403, got %d", resp.StatusCode)
	}

	// Trigger at least one admin audit event.
	resp, _ = doPutClient(adminClient, env.server.URL+"/api/admin/footer-message", map[string]string{
		"message": "Audit trail marker",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin footer update: expected 200, got %d", resp.StatusCode)
	}

	// Admin can list only admin_* events.
	resp, data := doGetClient(adminClient, env.server.URL+"/api/admin/audit?group=admin&limit=10", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin audit list: expected 200, got %d — %v", resp.StatusCode, data)
	}
	total, _ := data["total"].(float64)
	if total < 1 {
		t.Fatalf("expected at least 1 admin audit event, got %v", total)
	}

	events, ok := data["events"].([]interface{})
	if !ok || len(events) == 0 {
		t.Fatalf("expected non-empty audit events list, got %v", data["events"])
	}
	first := events[0].(map[string]interface{})
	eventType, _ := first["eventType"].(string)
	if !strings.HasPrefix(eventType, "admin_") {
		t.Fatalf("expected admin_* event type, got %q", eventType)
	}

	// Admin can export CSV for admin events.
	respRaw, body := doGetRawClient(adminClient, env.server.URL+"/api/admin/audit/export?group=admin&limit=50", "")
	if respRaw.StatusCode != http.StatusOK {
		t.Fatalf("admin audit export: expected 200, got %d", respRaw.StatusCode)
	}
	contentType := respRaw.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/csv") {
		t.Fatalf("expected text/csv content type, got %q", contentType)
	}
	if !strings.Contains(string(body), "eventType") {
		t.Fatalf("expected CSV header in export body, got %q", string(body))
	}
	if !strings.Contains(string(body), "admin_footer_updated") {
		t.Fatalf("expected exported event type admin_footer_updated, got %q", string(body))
	}
}

func TestE2E_UserCannotDownloadAnotherUsersArtifact(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	// Create two users
	clientA := registerUserClient(t, env, "UserA", "userA@test.com")
	clientB := registerUserClient(t, env, "UserB", "userB@test.com")

	// Get UserA's ID
	respA, meA := doGetClient(clientA, env.server.URL+"/api/auth/me", "")
	if respA.StatusCode != http.StatusOK {
		t.Fatalf("auth me A: expected 200, got %d", respA.StatusCode)
	}
	userAID, _ := meA["id"].(string)
	if userAID == "" {
		t.Fatal("expected user A id")
	}

	// Create a file, job, and artifact owned by UserA
	now := time.Now().UTC()
	fileID := uuid.New()
	jobID := uuid.New()
	artifactID := uuid.New()
	artifactDir := filepath.Join(env.tmpDir, "storage", "artifacts", artifactID.String())
	if err := os.MkdirAll(artifactDir, 0o750); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	artifactPath := filepath.Join(artifactDir, "secret.txt")
	if err := os.WriteFile(artifactPath, []byte("user-a-secret"), 0o600); err != nil {
		t.Fatalf("write artifact file: %v", err)
	}

	if _, err := env.db.Exec(
		`INSERT INTO files (id, user_id, guest_session_id, internal_name, original_name, size, mime_type, format_family, detected_extension, metadata, uploaded_at)
		 VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fileID.String(), userAID, "fixture", "fixture.txt", 13, "text/plain", "document", "txt", `{}`, now.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert file row: %v", err)
	}
	if _, err := env.db.Exec(
		`INSERT INTO jobs (id, user_id, file_id, capability_id, output_format, status, progress, created_at, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		jobID.String(), userAID, fileID.String(), "doc-to-txt", "txt", string(domain.JobSucceeded), 100, now.Add(-2*time.Minute).Format(time.RFC3339Nano), now.Add(-90*time.Second).Format(time.RFC3339Nano), now.Add(-time.Minute).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert job row: %v", err)
	}
	if _, err := env.db.Exec(
		`INSERT INTO artifacts (id, user_id, job_id, file_id, file_name, mime_type, size, storage_path, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifactID.String(), userAID, jobID.String(), fileID.String(), "secret.txt", "text/plain", 13, artifactPath, now.Format(time.RFC3339Nano), now.Add(2*time.Hour).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert artifact row: %v", err)
	}

	// UserA can download their own artifact
	respA, bodyA := downloadArtifactClient(clientA, env.server.URL, artifactID.String())
	if respA.StatusCode != http.StatusOK {
		t.Fatalf("user A download own artifact: expected 200, got %d", respA.StatusCode)
	}
	if string(bodyA) != "user-a-secret" {
		t.Fatalf("user A download: unexpected body %q", string(bodyA))
	}

	// UserB CANNOT download UserA's artifact
	respB, bodyB := downloadArtifactClient(clientB, env.server.URL, artifactID.String())
	if respB.StatusCode != http.StatusForbidden {
		t.Fatalf("user B download other's artifact: expected 403, got %d — body: %s", respB.StatusCode, string(bodyB))
	}
}

func TestE2E_GuestCannotDownloadAuthenticatedUsersArtifact(t *testing.T) {
	env := setupE2E(t)
	defer env.close()

	// Create an authenticated user
	clientUser := registerUserClient(t, env, "AuthUser", "auth@test.com")
	resp, me := doGetClient(clientUser, env.server.URL+"/api/auth/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth me: expected 200, got %d", resp.StatusCode)
	}
	userID, _ := me["id"].(string)
	if userID == "" {
		t.Fatal("expected user id")
	}

	// Create a file, job, and artifact owned by the authenticated user
	now := time.Now().UTC()
	fileID := uuid.New()
	jobID := uuid.New()
	artifactID := uuid.New()
	artifactDir := filepath.Join(env.tmpDir, "storage", "artifacts", artifactID.String())
	if err := os.MkdirAll(artifactDir, 0o750); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	artifactPath := filepath.Join(artifactDir, "auth-only.txt")
	if err := os.WriteFile(artifactPath, []byte("auth-only-content"), 0o600); err != nil {
		t.Fatalf("write artifact file: %v", err)
	}

	if _, err := env.db.Exec(
		`INSERT INTO files (id, user_id, guest_session_id, internal_name, original_name, size, mime_type, format_family, detected_extension, metadata, uploaded_at)
		 VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fileID.String(), userID, "fixture", "fixture.txt", 15, "text/plain", "document", "txt", `{}`, now.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert file row: %v", err)
	}
	if _, err := env.db.Exec(
		`INSERT INTO jobs (id, user_id, file_id, capability_id, output_format, status, progress, created_at, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		jobID.String(), userID, fileID.String(), "doc-to-txt", "txt", string(domain.JobSucceeded), 100, now.Add(-2*time.Minute).Format(time.RFC3339Nano), now.Add(-90*time.Second).Format(time.RFC3339Nano), now.Add(-time.Minute).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert job row: %v", err)
	}
	if _, err := env.db.Exec(
		`INSERT INTO artifacts (id, user_id, job_id, file_id, file_name, mime_type, size, storage_path, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifactID.String(), userID, jobID.String(), fileID.String(), "auth-only.txt", "text/plain", 15, artifactPath, now.Format(time.RFC3339Nano), now.Add(2*time.Hour).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert artifact row: %v", err)
	}

	// Guest (no auth) cannot download the authenticated user's artifact
	guestClient := newCookieClient(t)
	respGuest, bodyGuest := downloadArtifactClient(guestClient, env.server.URL, artifactID.String())
	if respGuest.StatusCode != http.StatusForbidden {
		t.Fatalf("guest download auth user artifact: expected 403, got %d — body: %s", respGuest.StatusCode, string(bodyGuest))
	}
}
