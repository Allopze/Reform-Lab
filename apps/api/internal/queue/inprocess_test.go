package queue

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"
)

func TestInProcessQueueLogsHandlerErrors(t *testing.T) {
	var buf bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer log.SetOutput(previousWriter)
	defer log.SetFlags(previousFlags)

	q := NewInProcessQueueWithLimit(func(_ context.Context, taskType string, _ []byte) error {
		if taskType != "convert:test" {
			t.Fatalf("unexpected task type %q", taskType)
		}
		return errors.New("boom")
	}, 1)

	err := q.Enqueue(context.Background(), "convert:test", TaskPayload{
		JobID:        "job-1",
		FileID:       "file-1",
		CapabilityID: "cap-1",
		InputPath:    "/tmp/input",
		OutputFormat: "pdf",
	}, TaskOptions{Timeout: time.Second})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := q.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got := buf.String(); !strings.Contains(got, "[InProcessQueue] task convert:test failed: boom") {
		t.Fatalf("expected handler error to be logged, got %q", got)
	}
}
