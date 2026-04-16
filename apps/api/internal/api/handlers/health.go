package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"golang.org/x/sys/unix"
)

const (
	queueBacklogWarningThreshold = 100
	runningJobsWarningThreshold  = 20
	storageUsedWarningPercent    = 85.0
	storageUsedCriticalPercent   = 95.0
)

type healthAlert struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
}

// PublicHealth returns a minimal public health check.
func PublicHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "ok",
		})
	}
}

// DetailedHealth returns service policy and operational snapshots for admins.
type DetailedHealthHandler struct {
	ArtifactTTLHours    int
	ArtifactTTLByFamily map[string]int

	Jobs              repository.JobRepository
	RuntimeControls   repository.RuntimeControlRepository
	Workers           repository.WorkerStatusRepository
	Database          *sql.DB
	StorageBasePath   string
	QueueMode         string
	WorkerConcurrency int
	RedisURL          string
}

func (h *DetailedHealthHandler) Handle(w http.ResponseWriter, r *http.Request) {
	queueSnapshot := h.queueSnapshot(r.Context())
	storageSnapshot := h.storageSnapshot()
	databaseSnapshot := h.databaseSnapshot(r.Context())
	redisSnapshot := h.redisSnapshot()
	alerts := buildHealthAlerts(queueSnapshot, storageSnapshot, databaseSnapshot, redisSnapshot)

	overallStatus := "ok"
	if isDownDependency(databaseSnapshot["status"]) || isDownDependency(storageSnapshot["status"]) {
		overallStatus = "degraded"
	}
	if normalizedQueueMode(h.QueueMode) == "redis" && isDownDependency(redisSnapshot["status"]) {
		overallStatus = "degraded"
	}
	if hasCriticalAlert(alerts) {
		overallStatus = "degraded"
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": overallStatus,
		"retention": map[string]interface{}{
			"artifactTTLHours":         h.ArtifactTTLHours,
			"artifactTTLHoursByFamily": h.ArtifactTTLByFamily,
		},
		"featureFlags": capabilities.DefaultFlags.Snapshot(),
		"runtime": map[string]interface{}{
			"queue":   queueSnapshot,
			"storage": storageSnapshot,
			"workers": h.workerSnapshot(r.Context()),
		},
		"dependencies": map[string]interface{}{
			"database": databaseSnapshot,
			"redis":    redisSnapshot,
		},
		"alerts": alerts,
	})
}

func buildHealthAlerts(
	queueSnapshot map[string]interface{},
	storageSnapshot map[string]interface{},
	databaseSnapshot map[string]interface{},
	redisSnapshot map[string]interface{},
) []healthAlert {
	alerts := make([]healthAlert, 0, 6)

	if statusFromSnapshot(databaseSnapshot) == "down" {
		alerts = append(alerts, healthAlert{
			Code:        "database_down",
			Severity:    "critical",
			Summary:     "Database unavailable",
			Description: "The database dependency is down. Admin operations may fail until connectivity is restored.",
		})
	}

	if statusFromSnapshot(storageSnapshot) == "down" {
		alerts = append(alerts, healthAlert{
			Code:        "storage_down",
			Severity:    "critical",
			Summary:     "Storage unavailable",
			Description: "Storage access failed. Uploads and artifact persistence may be impacted.",
		})
	}

	if statusFromSnapshot(redisSnapshot) == "down" {
		alerts = append(alerts, healthAlert{
			Code:        "redis_down",
			Severity:    "critical",
			Summary:     "Redis queue dependency unavailable",
			Description: "Redis is configured for queue mode but cannot be reached.",
		})
	}

	if usedPercent, ok := floatFromSnapshot(storageSnapshot, "usedPercent"); ok {
		if usedPercent >= storageUsedCriticalPercent {
			alerts = append(alerts, healthAlert{
				Code:        "storage_pressure_critical",
				Severity:    "critical",
				Summary:     "Storage pressure is critical",
				Description: fmt.Sprintf("Storage usage is at %.1f%%, above the %.0f%% critical threshold.", usedPercent, storageUsedCriticalPercent),
			})
		} else if usedPercent >= storageUsedWarningPercent {
			alerts = append(alerts, healthAlert{
				Code:        "storage_pressure_warning",
				Severity:    "warning",
				Summary:     "Storage pressure is elevated",
				Description: fmt.Sprintf("Storage usage is at %.1f%%, above the %.0f%% warning threshold.", usedPercent, storageUsedWarningPercent),
			})
		}
	}

	if queuedJobs, ok := intFromSnapshot(queueSnapshot, "queuedJobs"); ok && queuedJobs >= queueBacklogWarningThreshold {
		alerts = append(alerts, healthAlert{
			Code:        "queue_backlog_high",
			Severity:    "warning",
			Summary:     "Queue backlog is high",
			Description: fmt.Sprintf("There are %d queued jobs, above the %d-job warning threshold.", queuedJobs, queueBacklogWarningThreshold),
		})
	}

	if runningJobs, ok := intFromSnapshot(queueSnapshot, "runningJobs"); ok && runningJobs >= runningJobsWarningThreshold {
		alerts = append(alerts, healthAlert{
			Code:        "running_jobs_high",
			Severity:    "warning",
			Summary:     "Running jobs count is high",
			Description: fmt.Sprintf("There are %d running jobs, above the %d-job warning threshold.", runningJobs, runningJobsWarningThreshold),
		})
	}

	if stalledJobs, ok := intFromSnapshot(queueSnapshot, "stalledJobs"); ok && stalledJobs > 0 {
		stalledQueued, _ := intFromSnapshot(queueSnapshot, "stalledQueuedJobs")
		stalledRunning, _ := intFromSnapshot(queueSnapshot, "stalledRunningJobs")
		alerts = append(alerts, healthAlert{
			Code:     "stalled_jobs_detected",
			Severity: "warning",
			Summary:  "Stalled jobs detected",
			Description: fmt.Sprintf(
				"There are %d stalled jobs (%d queued, %d running). Review /admin/jobs with stalled filter for details.",
				stalledJobs,
				stalledQueued,
				stalledRunning,
			),
		})
	}

	return alerts
}

func hasCriticalAlert(alerts []healthAlert) bool {
	for _, alert := range alerts {
		if alert.Severity == "critical" {
			return true
		}
	}
	return false
}

func statusFromSnapshot(snapshot map[string]interface{}) string {
	value, ok := snapshot["status"].(string)
	if !ok {
		return ""
	}
	return value
}

func intFromSnapshot(snapshot map[string]interface{}, key string) (int, bool) {
	value, ok := snapshot[key]
	if !ok {
		return 0, false
	}

	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		if math.IsNaN(typed) {
			return 0, false
		}
		return int(math.Round(typed)), true
	default:
		return 0, false
	}
}

func floatFromSnapshot(snapshot map[string]interface{}, key string) (float64, bool) {
	value, ok := snapshot[key]
	if !ok {
		return 0, false
	}

	switch typed := value.(type) {
	case float64:
		if math.IsNaN(typed) {
			return 0, false
		}
		return typed, true
	case float32:
		v := float64(typed)
		if math.IsNaN(v) {
			return 0, false
		}
		return v, true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func (h *DetailedHealthHandler) queueSnapshot(ctx context.Context) map[string]interface{} {
	queuedJobs := -1
	runningJobs := -1
	stalledJobs := -1
	stalledQueuedJobs := -1
	stalledRunningJobs := -1

	if h.Jobs != nil {
		if count, err := h.Jobs.CountByStatuses(ctx, domain.JobQueued); err == nil {
			queuedJobs = count
		}
		if count, err := h.Jobs.CountByStatuses(ctx, domain.JobRunning); err == nil {
			runningJobs = count
		}
		if page, err := h.Jobs.ListForAdmin(ctx, repository.AdminJobFilter{Limit: 1}); err == nil {
			stalledJobs = page.StalledJobs
			stalledQueuedJobs = page.StalledQueuedJobs
			stalledRunningJobs = page.StalledRunningJobs
		}
	}
	history := []repository.AdminQueueHistoryPoint{}
	controls := map[string]interface{}{}
	if h.Jobs != nil {
		if points, err := h.Jobs.QueueHistory(ctx); err == nil {
			history = points
		}
	}
	if h.RuntimeControls != nil {
		if state, err := h.RuntimeControls.Get(ctx); err == nil {
			controls = map[string]interface{}{
				"jobIntakePaused": state.JobIntakePaused,
				"pauseReason":     state.PauseReason,
				"updatedAt":       state.UpdatedAt,
			}
		}
	}

	return map[string]interface{}{
		"mode":               normalizedQueueMode(h.QueueMode),
		"workerConcurrency":  h.WorkerConcurrency,
		"queuedJobs":         queuedJobs,
		"runningJobs":        runningJobs,
		"stalledJobs":        stalledJobs,
		"stalledQueuedJobs":  stalledQueuedJobs,
		"stalledRunningJobs": stalledRunningJobs,
		"history":            history,
		"controls":           controls,
	}
}

func (h *DetailedHealthHandler) workerSnapshot(ctx context.Context) map[string]interface{} {
	workers := []repository.WorkerStatusSnapshot{}
	if h.Workers != nil {
		if items, err := h.Workers.List(ctx, 3); err == nil {
			workers = items
		}
	}
	return map[string]interface{}{
		"count":   len(workers),
		"workers": workers,
	}
}

func (h *DetailedHealthHandler) storageSnapshot() map[string]interface{} {
	basePath := strings.TrimSpace(h.StorageBasePath)
	if basePath == "" {
		return map[string]interface{}{
			"status": "unknown",
		}
	}

	absPath, err := filepath.Abs(basePath)
	if err != nil {
		absPath = basePath
	}

	var stat unix.Statfs_t
	if err := unix.Statfs(absPath, &stat); err != nil {
		return map[string]interface{}{
			"status": "down",
			"path":   absPath,
			"error":  err.Error(),
		}
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	usedPercent := 0.0
	if totalBytes > 0 {
		usedPercent = (float64(totalBytes-freeBytes) / float64(totalBytes)) * 100
	}

	return map[string]interface{}{
		"status":      "up",
		"path":        absPath,
		"totalBytes":  totalBytes,
		"freeBytes":   freeBytes,
		"usedPercent": usedPercent,
	}
}

func (h *DetailedHealthHandler) databaseSnapshot(ctx context.Context) map[string]interface{} {
	if h.Database == nil {
		return map[string]interface{}{
			"status": "unknown",
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	startedAt := time.Now()
	err := h.Database.PingContext(checkCtx)
	latencyMs := float64(time.Since(startedAt).Microseconds()) / 1000

	if err != nil {
		return map[string]interface{}{
			"status":    "down",
			"latencyMs": latencyMs,
			"error":     err.Error(),
		}
	}

	return map[string]interface{}{
		"status":    "up",
		"latencyMs": latencyMs,
	}
}

func (h *DetailedHealthHandler) redisSnapshot() map[string]interface{} {
	if normalizedQueueMode(h.QueueMode) != "redis" || strings.TrimSpace(h.RedisURL) == "" {
		return map[string]interface{}{
			"status": "not_configured",
		}
	}

	targetAddr, err := redisDialAddress(h.RedisURL)
	if err != nil {
		return map[string]interface{}{
			"status": "down",
			"error":  err.Error(),
		}
	}

	startedAt := time.Now()
	conn, err := net.DialTimeout("tcp", targetAddr, 2*time.Second)
	latencyMs := float64(time.Since(startedAt).Microseconds()) / 1000
	if err != nil {
		return map[string]interface{}{
			"status":    "down",
			"address":   targetAddr,
			"latencyMs": latencyMs,
			"error":     err.Error(),
		}
	}
	_ = conn.Close()

	return map[string]interface{}{
		"status":    "up",
		"address":   targetAddr,
		"latencyMs": latencyMs,
	}
}

func normalizedQueueMode(mode string) string {
	trimmed := strings.TrimSpace(strings.ToLower(mode))
	if trimmed == "" {
		return "in-process"
	}
	return trimmed
}

func isDownDependency(status interface{}) bool {
	value, ok := status.(string)
	if !ok {
		return false
	}
	return value == "down"
}

func redisDialAddress(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse redis url: %w", err)
	}

	host := parsed.Host
	if host == "" {
		host = parsed.Path
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("redis host is empty")
	}
	if !strings.Contains(host, ":") {
		host = host + ":6379"
	}

	return host, nil
}

// respondJSON writes a JSON response.
func respondJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
