package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type recordingSiteSettings struct {
	writes []string
}

func (s *recordingSiteSettings) GetValue(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func (s *recordingSiteSettings) UpsertValue(_ context.Context, key, _ string, _ time.Time) error {
	s.writes = append(s.writes, key)
	return nil
}

func (s *recordingSiteSettings) UpsertValues(_ context.Context, values map[string]string, _ time.Time) error {
	for key := range values {
		s.writes = append(s.writes, key)
	}
	return nil
}

func TestSMTPSettingsUpdateValidatesSecretStorageBeforePersisting(t *testing.T) {
	settings := &recordingSiteSettings{}
	handler := &SMTPSettingsHandler{Settings: settings}
	body := `{"host":"mail.example.com","port":587,"user":"smtp-user","password":"secret","from":"noreply@example.com","use_tls":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/smtp-settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Update(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when secret storage is unavailable, got %d", rec.Code)
	}
	if len(settings.writes) != 0 {
		t.Fatalf("expected no partial settings writes, got %v", settings.writes)
	}
}
