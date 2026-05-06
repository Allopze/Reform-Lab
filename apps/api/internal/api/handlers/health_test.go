package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildHealthAlerts_CriticalDependencies(t *testing.T) {
	alerts := buildHealthAlerts(
		map[string]interface{}{"queuedJobs": 0, "runningJobs": 0},
		map[string]interface{}{"status": "down"},
		map[string]interface{}{"status": "down"},
		map[string]interface{}{"status": "down"},
	)

	if !alertCodesContain(alerts, "database_down") {
		t.Fatal("expected database_down alert")
	}
	if !alertCodesContain(alerts, "storage_down") {
		t.Fatal("expected storage_down alert")
	}
	if !alertCodesContain(alerts, "redis_down") {
		t.Fatal("expected redis_down alert")
	}
	if !hasCriticalAlert(alerts) {
		t.Fatal("expected at least one critical alert")
	}
}

func TestBuildHealthAlerts_OperationalWarnings(t *testing.T) {
	alerts := buildHealthAlerts(
		map[string]interface{}{"queuedJobs": 140, "runningJobs": 24},
		map[string]interface{}{"status": "up", "usedPercent": 90.4},
		map[string]interface{}{"status": "up"},
		map[string]interface{}{"status": "not_configured"},
	)

	if !alertCodesContain(alerts, "queue_backlog_high") {
		t.Fatal("expected queue_backlog_high alert")
	}
	if !alertCodesContain(alerts, "running_jobs_high") {
		t.Fatal("expected running_jobs_high alert")
	}
	if !alertCodesContain(alerts, "storage_pressure_warning") {
		t.Fatal("expected storage_pressure_warning alert")
	}
	if hasCriticalAlert(alerts) {
		t.Fatal("did not expect critical alerts in warning-only scenario")
	}
}

func TestBuildHealthAlerts_StorageCriticalOverridesWarning(t *testing.T) {
	alerts := buildHealthAlerts(
		map[string]interface{}{"queuedJobs": 0, "runningJobs": 0},
		map[string]interface{}{"status": "up", "usedPercent": 96.2},
		map[string]interface{}{"status": "up"},
		map[string]interface{}{"status": "not_configured"},
	)

	if !alertCodesContain(alerts, "storage_pressure_critical") {
		t.Fatal("expected storage_pressure_critical alert")
	}
	if alertCodesContain(alerts, "storage_pressure_warning") {
		t.Fatal("did not expect storage_pressure_warning when critical threshold is reached")
	}
}

func TestBuildHealthAlerts_StalledJobsDetected(t *testing.T) {
	alerts := buildHealthAlerts(
		map[string]interface{}{
			"queuedJobs":         8,
			"runningJobs":        2,
			"stalledJobs":        3,
			"stalledQueuedJobs":  2,
			"stalledRunningJobs": 1,
		},
		map[string]interface{}{"status": "up", "usedPercent": 40.0},
		map[string]interface{}{"status": "up"},
		map[string]interface{}{"status": "not_configured"},
	)

	if !alertCodesContain(alerts, "stalled_jobs_detected") {
		t.Fatal("expected stalled_jobs_detected alert")
	}
}

func TestRespondErrorIncludesStableEnvelope(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-ID", "req-123")

	respondError(rr, http.StatusTooManyRequests, "too many active jobs for this user")

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", rr.Code)
	}
	var body errorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error != "too many active jobs for this user" {
		t.Fatalf("expected legacy error string, got %q", body.Error)
	}
	if body.Message != body.Error {
		t.Fatalf("expected message to match legacy error, got %q", body.Message)
	}
	if body.Code != "too_many_active_jobs_for_this_user" {
		t.Fatalf("unexpected code %q", body.Code)
	}
	if body.RequestID != "req-123" {
		t.Fatalf("expected request id, got %q", body.RequestID)
	}
	if !body.Retryable {
		t.Fatal("expected 429 to be retryable")
	}
}

func alertCodesContain(alerts []healthAlert, code string) bool {
	for _, alert := range alerts {
		if alert.Code == code {
			return true
		}
	}
	return false
}
