package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	htmltmpl "html/template"
	"net"
	"net/smtp"
	"strconv"
	texttmpl "text/template"
	"time"

	"github.com/allopze/reform-lab/apps/api/config"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/rs/zerolog"
)

// SMTP site_settings keys.
const (
	SettingSMTPHost     = "smtp_host"
	SettingSMTPPort     = "smtp_port"
	SettingSMTPUser     = "smtp_user"
	SettingSMTPPassword = "smtp_password"
	SettingSMTPFrom     = "smtp_from"
	SettingSMTPUseTLS   = "smtp_use_tls"
)

// Service handles email template rendering and SMTP delivery.
type Service struct {
	cfg       *config.Config
	settings  repository.SiteSettingRepository
	templates repository.EmailTemplateRepository
	logger    zerolog.Logger
}

// NewService creates an email service.
func NewService(
	cfg *config.Config,
	settings repository.SiteSettingRepository,
	templates repository.EmailTemplateRepository,
	logger zerolog.Logger,
) *Service {
	return &Service{
		cfg:       cfg,
		settings:  settings,
		templates: templates,
		logger:    logger.With().Str("component", "email").Logger(),
	}
}

// ResolveSMTPConfig reads SMTP config from .env defaults, then overrides
// with any values stored in site_settings (admin panel).
func (s *Service) ResolveSMTPConfig(ctx context.Context) domain.SMTPConfig {
	cfg := domain.SMTPConfig{
		Host:     s.cfg.SMTPHost,
		Port:     s.cfg.SMTPPort,
		User:     s.cfg.SMTPUser,
		Password: s.cfg.SMTPPassword,
		From:     s.cfg.SMTPFrom,
		UseTLS:   s.cfg.SMTPUseTLS,
	}

	// Override with site_settings if present.
	if v, ok, _ := s.settings.GetValue(ctx, SettingSMTPHost); ok && v != "" {
		cfg.Host = v
	}
	if v, ok, _ := s.settings.GetValue(ctx, SettingSMTPPort); ok && v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.Port = p
		}
	}
	if v, ok, _ := s.settings.GetValue(ctx, SettingSMTPUser); ok {
		cfg.User = v
	}
	if v, ok, _ := s.settings.GetValue(ctx, SettingSMTPPassword); ok && v != "" {
		cfg.Password = v
	}
	if v, ok, _ := s.settings.GetValue(ctx, SettingSMTPFrom); ok && v != "" {
		cfg.From = v
	}
	if v, ok, _ := s.settings.GetValue(ctx, SettingSMTPUseTLS); ok {
		switch v {
		case "true", "1":
			cfg.UseTLS = true
		case "false", "0":
			cfg.UseTLS = false
		}
	}

	return cfg
}

// RenderTemplate loads the template by key and renders it with the given variables.
func (s *Service) RenderTemplate(ctx context.Context, templateKey string, vars map[string]string) (*domain.EmailMessage, error) {
	tmpl, err := s.templates.GetByKey(ctx, templateKey)
	if err != nil {
		return nil, fmt.Errorf("load template %q: %w", templateKey, err)
	}
	if tmpl == nil {
		return nil, fmt.Errorf("template %q not found", templateKey)
	}

	// Render subject (plain text — no HTML escaping)
	subjectTmpl, err := texttmpl.New("subject").Parse(tmpl.Subject)
	if err != nil {
		return nil, fmt.Errorf("parse subject template: %w", err)
	}
	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, vars); err != nil {
		return nil, fmt.Errorf("render subject: %w", err)
	}

	// Render body (HTML — contextual auto-escaping)
	bodyTmpl, err := htmltmpl.New("body").Parse(tmpl.BodyHTML)
	if err != nil {
		return nil, fmt.Errorf("parse body template: %w", err)
	}
	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, vars); err != nil {
		return nil, fmt.Errorf("render body: %w", err)
	}

	return &domain.EmailMessage{
		Subject:  subjectBuf.String(),
		BodyHTML: bodyBuf.String(),
	}, nil
}

// Send delivers an email message via SMTP.
func (s *Service) Send(ctx context.Context, msg *domain.EmailMessage) error {
	smtpCfg := s.ResolveSMTPConfig(ctx)
	if !smtpCfg.Configured() {
		return fmt.Errorf("SMTP not configured")
	}

	return s.sendViaSMTP(smtpCfg, msg)
}

// SendTestEmail sends a test email to the given address.
func (s *Service) SendTestEmail(ctx context.Context, to string) error {
	smtpCfg := s.ResolveSMTPConfig(ctx)
	if !smtpCfg.Configured() {
		return fmt.Errorf("SMTP not configured")
	}

	msg := &domain.EmailMessage{
		To:       to,
		Subject:  "Reform Lab — Correo de prueba",
		BodyHTML: "<p>Este es un correo de prueba desde Reform Lab. Si lo recibes, la configuración SMTP es correcta.</p>",
	}

	return s.sendViaSMTP(smtpCfg, msg)
}

// Configured returns true if SMTP has enough config to attempt delivery.
func (s *Service) Configured(ctx context.Context) bool {
	return s.ResolveSMTPConfig(ctx).Configured()
}

func (s *Service) sendViaSMTP(cfg domain.SMTPConfig, msg *domain.EmailMessage) error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	// Build RFC 2822 message
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", cfg.From)
	fmt.Fprintf(&buf, "To: %s\r\n", msg.To)
	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "\r\n")
	buf.WriteString(msg.BodyHTML)

	var auth smtp.Auth
	if cfg.User != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	}

	if cfg.UseTLS {
		return s.sendWithTLS(addr, cfg.Host, auth, cfg.From, msg.To, buf.Bytes())
	}
	return smtp.SendMail(addr, auth, cfg.From, []string{msg.To}, buf.Bytes())
}

func (s *Service) sendWithTLS(addr, host string, auth smtp.Auth, from, to string, body []byte) error {
	tlsCfg := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("TLS dial: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SMTP client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("SMTP write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP close data: %w", err)
	}

	return client.Quit()
}
