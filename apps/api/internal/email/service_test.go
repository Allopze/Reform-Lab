package email

import (
	"context"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/config"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/rs/zerolog"
)

// --- Mocks ---

type mockSiteSettings struct {
	values map[string]string
}

func (m *mockSiteSettings) GetValue(_ context.Context, key string) (string, bool, error) {
	v, ok := m.values[key]
	return v, ok, nil
}

func (m *mockSiteSettings) UpsertValue(_ context.Context, key, value string, _ time.Time) error {
	m.values[key] = value
	return nil
}

func (m *mockSiteSettings) UpsertValues(_ context.Context, values map[string]string, _ time.Time) error {
	for key, value := range values {
		m.values[key] = value
	}
	return nil
}

type mockEmailTemplateRepo struct {
	templates map[string]*domain.EmailTemplate
}

func (m *mockEmailTemplateRepo) GetByKey(_ context.Context, key string) (*domain.EmailTemplate, error) {
	return m.templates[key], nil
}

func (m *mockEmailTemplateRepo) Upsert(_ context.Context, tmpl *domain.EmailTemplate) error {
	m.templates[tmpl.Key] = tmpl
	return nil
}

func (m *mockEmailTemplateRepo) ListAll(_ context.Context) ([]domain.EmailTemplate, error) {
	var result []domain.EmailTemplate
	for _, t := range m.templates {
		result = append(result, *t)
	}
	return result, nil
}

func (m *mockEmailTemplateRepo) Delete(_ context.Context, key string) error {
	delete(m.templates, key)
	return nil
}

// --- Tests ---

func TestResolveSMTPConfig_EnvOnly(t *testing.T) {
	cfg := &config.Config{
		SMTPHost:     "mail.example.com",
		SMTPPort:     587,
		SMTPUser:     "user@example.com",
		SMTPPassword: "secret",
		SMTPFrom:     "noreply@example.com",
		SMTPUseTLS:   true,
	}
	settings := &mockSiteSettings{values: map[string]string{}}
	svc := NewService(cfg, settings, nil, testLogger())

	result := svc.ResolveSMTPConfig(context.Background())

	if result.Host != "mail.example.com" {
		t.Errorf("expected host mail.example.com, got %s", result.Host)
	}
	if result.Port != 587 {
		t.Errorf("expected port 587, got %d", result.Port)
	}
	if !result.UseTLS {
		t.Error("expected UseTLS true")
	}
	if !result.Configured() {
		t.Error("expected Configured() true")
	}
}

func TestResolveSMTPConfig_SettingsOverride(t *testing.T) {
	cfg := &config.Config{
		SMTPHost:     "env-host.com",
		SMTPPort:     587,
		SMTPUser:     "env-user",
		SMTPPassword: "env-pass",
		SMTPFrom:     "env@example.com",
		SMTPUseTLS:   true,
	}
	settings := &mockSiteSettings{values: map[string]string{
		SettingSMTPHost:     "admin-host.com",
		SettingSMTPPort:     "465",
		SettingSMTPUser:     "admin-user",
		SettingSMTPPassword: "admin-pass",
		SettingSMTPFrom:     "admin@example.com",
		SettingSMTPUseTLS:   "false",
	}}
	svc := NewService(cfg, settings, nil, testLogger())

	result := svc.ResolveSMTPConfig(context.Background())

	if result.Host != "admin-host.com" {
		t.Errorf("expected admin-host.com, got %s", result.Host)
	}
	if result.Port != 465 {
		t.Errorf("expected port 465, got %d", result.Port)
	}
	if result.User != "admin-user" {
		t.Errorf("expected admin-user, got %s", result.User)
	}
	if result.Password != "admin-pass" {
		t.Errorf("expected admin-pass, got %s", result.Password)
	}
	if result.From != "admin@example.com" {
		t.Errorf("expected admin@example.com, got %s", result.From)
	}
	if result.UseTLS {
		t.Error("expected UseTLS false (overridden)")
	}
}

func TestResolveSMTPConfig_Unconfigured(t *testing.T) {
	cfg := &config.Config{SMTPPort: 587}
	settings := &mockSiteSettings{values: map[string]string{}}
	svc := NewService(cfg, settings, nil, testLogger())

	result := svc.ResolveSMTPConfig(context.Background())

	if result.Configured() {
		t.Error("expected Configured() false when host is empty")
	}
}

func TestRenderTemplate_Success(t *testing.T) {
	cfg := &config.Config{}
	settings := &mockSiteSettings{values: map[string]string{}}
	templates := &mockEmailTemplateRepo{
		templates: map[string]*domain.EmailTemplate{
			"welcome": {
				Key:      "welcome",
				Subject:  "Bienvenido a {{.AppName}}",
				BodyHTML: "<h1>Hola {{.Name}}</h1>",
			},
		},
	}
	svc := NewService(cfg, settings, templates, testLogger())

	msg, err := svc.RenderTemplate(context.Background(), "welcome", map[string]string{
		"AppName": "Reform Lab",
		"Name":    "Juan",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Subject != "Bienvenido a Reform Lab" {
		t.Errorf("unexpected subject: %s", msg.Subject)
	}
	if msg.BodyHTML != "<h1>Hola Juan</h1>" {
		t.Errorf("unexpected body: %s", msg.BodyHTML)
	}
}

func TestRenderTemplate_NotFound(t *testing.T) {
	cfg := &config.Config{}
	settings := &mockSiteSettings{values: map[string]string{}}
	templates := &mockEmailTemplateRepo{templates: map[string]*domain.EmailTemplate{}}
	svc := NewService(cfg, settings, templates, testLogger())

	_, err := svc.RenderTemplate(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for missing template")
	}
}

func TestRenderTemplate_InvalidTemplate(t *testing.T) {
	cfg := &config.Config{}
	settings := &mockSiteSettings{values: map[string]string{}}
	templates := &mockEmailTemplateRepo{
		templates: map[string]*domain.EmailTemplate{
			"bad": {
				Key:      "bad",
				Subject:  "OK",
				BodyHTML: "{{.Unbalanced",
			},
		},
	}
	svc := NewService(cfg, settings, templates, testLogger())

	_, err := svc.RenderTemplate(context.Background(), "bad", nil)
	if err == nil {
		t.Fatal("expected error for malformed template")
	}
}

func TestConfigured_ReflectsSettings(t *testing.T) {
	cfg := &config.Config{SMTPPort: 587}
	settings := &mockSiteSettings{values: map[string]string{}}
	svc := NewService(cfg, settings, nil, testLogger())

	if svc.Configured(context.Background()) {
		t.Error("expected false when no host")
	}

	settings.values[SettingSMTPHost] = "smtp.example.com"
	if !svc.Configured(context.Background()) {
		t.Error("expected true after setting host via admin")
	}
}

func TestRenderTemplate_SubjectNoHTMLEscaping(t *testing.T) {
	cfg := &config.Config{}
	settings := &mockSiteSettings{values: map[string]string{}}
	templates := &mockEmailTemplateRepo{
		templates: map[string]*domain.EmailTemplate{
			"welcome": {
				Key:      "welcome",
				Subject:  "Hola {{.Name}} — novedades",
				BodyHTML: "<p>Hola {{.Name}}</p>",
			},
		},
	}
	svc := NewService(cfg, settings, templates, testLogger())

	msg, err := svc.RenderTemplate(context.Background(), "welcome", map[string]string{
		"Name": "O'Brian & Co",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Subject is plain text — must NOT have HTML entities.
	expected := "Hola O'Brian & Co — novedades"
	if msg.Subject != expected {
		t.Errorf("subject should be plain text, got %q, want %q", msg.Subject, expected)
	}
	// Body IS HTML — values should be contextually escaped.
	expectedBody := "<p>Hola O&#39;Brian &amp; Co</p>"
	if msg.BodyHTML != expectedBody {
		t.Errorf("body should have HTML-escaped values, got %q, want %q", msg.BodyHTML, expectedBody)
	}
}

func testLogger() zerolog.Logger {
	return zerolog.Nop()
}
