package email

import (
	"context"
	"testing"
	"time"
)

func TestSMTPTimeoutForContext_Default(t *testing.T) {
	if got := smtpTimeoutForContext(context.Background()); got != smtpDialTimeout {
		t.Fatalf("expected default SMTP timeout %v, got %v", smtpDialTimeout, got)
	}
}

func TestSMTPTimeoutForContext_UsesSoonerDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got := smtpTimeoutForContext(ctx)
	if got <= 0 || got > 2*time.Second {
		t.Fatalf("expected SMTP timeout to respect nearer deadline, got %v", got)
	}
}
