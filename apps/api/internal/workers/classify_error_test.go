package workers

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestClassifyError_FeatureFlag(t *testing.T) {
	msg := classifyError("feature flag", errors.New("any error"))
	expected := "Capacidad deshabilitada temporalmente. Intenta más tarde."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}

func TestClassifyError_FindEngine(t *testing.T) {
	msg := classifyError("find engine", errors.New("engine not found"))
	expected := "Motor de conversión no disponible. Intenta más tarde."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}

func TestClassifyError_ValidateOutput(t *testing.T) {
	msg := classifyError("validate output", errors.New("output mismatch"))
	expected := "La conversión no produjo un resultado válido."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}

func TestClassifyError_CreateTempDir(t *testing.T) {
	msg := classifyError("create temp dir", errors.New("permission denied"))
	expected := "Error de almacenamiento interno. Intenta más tarde."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}

func TestClassifyError_SaveArtifact(t *testing.T) {
	msg := classifyError("save artifact", errors.New("disk full"))
	expected := "Error de almacenamiento interno. Intenta más tarde."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}

func TestClassifyError_SignalKilled(t *testing.T) {
	msg := classifyError("execute", errors.New("process failed: signal: killed"))
	expected := "La conversión excedió el tiempo máximo permitido."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}

func TestClassifyError_ContextDeadlineExceeded(t *testing.T) {
	msg := classifyError("execute", context.DeadlineExceeded)
	expected := "La conversión excedió el tiempo máximo permitido."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}

func TestClassifyError_TimeoutInMessage(t *testing.T) {
	msg := classifyError("execute", fmt.Errorf("operation timeout after 30s"))
	expected := "La conversión excedió el tiempo máximo permitido."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}

func TestClassifyError_ExitStatus(t *testing.T) {
	msg := classifyError("execute", errors.New("command failed: exit status 1"))
	expected := "El motor de conversión no pudo procesar este archivo."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}

func TestClassifyError_GenericFallback(t *testing.T) {
	msg := classifyError("execute", errors.New("something weird happened"))
	expected := "La conversión falló por un error interno. Intenta de nuevo."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}

func TestClassifyError_DoesNotFalsePositiveOnUnskilled(t *testing.T) {
	// "killed" should not match inside "unskilled"
	msg := classifyError("execute", errors.New("worker was unskilled"))
	// This should NOT match "signal: killed" because it's a different substring
	// strings.Contains("worker was unskilled", "signal: killed") == false
	// strings.Contains("worker was unskilled", "exit status") == false
	// So it falls through to default
	expected := "La conversión falló por un error interno. Intenta de nuevo."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}

func TestClassifyError_DoesNotFalsePositiveOnTimeoutHandler(t *testing.T) {
	// "timeout" in "timeout_handler" should match — this is correct behavior
	msg := classifyError("execute", errors.New("timeout_handler triggered"))
	expected := "La conversión excedió el tiempo máximo permitido."
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}
