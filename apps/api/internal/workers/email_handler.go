package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/allopze/reform-lab/apps/api/internal/email"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/rs/zerolog"
)

// EmailHandler processes email delivery tasks.
type EmailHandler struct {
	Email  *email.Service
	Logger zerolog.Logger
}

// ProcessPayload deserializes an email task and sends the email.
func (h *EmailHandler) ProcessPayload(ctx context.Context, _ string, data []byte) error {
	var payload queue.EmailTaskPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal email payload: %w", err)
	}

	logger := h.Logger.With().
		Str("template", payload.TemplateKey).
		Str("recipient_domain", emailDomain(payload.To)).
		Logger()

	msg, err := h.Email.RenderTemplate(ctx, payload.TemplateKey, payload.Vars)
	if err != nil {
		logger.Error().Err(err).Msg("email template render failed")
		return fmt.Errorf("render template %q: %w", payload.TemplateKey, err)
	}

	msg.To = payload.To

	if err := h.Email.Send(ctx, msg); err != nil {
		logger.Error().Err(err).Msg("email send failed")
		return fmt.Errorf("send email: %w", err)
	}

	logger.Info().Msg("email sent")
	return nil
}

func emailDomain(address string) string {
	parts := strings.SplitN(strings.TrimSpace(strings.ToLower(address)), "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "unknown"
	}
	return parts[1]
}
