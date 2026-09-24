package logger

import (
	"testing"
)

func TestNew(t *testing.T) {
	logger, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logger == nil {
		t.Error("logger should not be nil")
	}
}
