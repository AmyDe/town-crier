package platform

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestSecretString_RedactsEverywhere(t *testing.T) {
	t.Parallel()

	s := NewSecret("super-secret-value")

	if got := s.String(); got != "[REDACTED]" {
		t.Errorf("String(): got %q, want [REDACTED]", got)
	}
	if got := s.Expose(); got != "super-secret-value" {
		t.Errorf("Expose(): got %q, want the raw value", got)
	}

	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if string(b) != `"[REDACTED]"` {
		t.Errorf("MarshalJSON: got %s, want \"[REDACTED]\"", b)
	}

	if got := s.LogValue().String(); got != "[REDACTED]" {
		t.Errorf("LogValue: got %q, want [REDACTED]", got)
	}
}

func TestSecretString_NotLeakedViaFmtOrSlog(t *testing.T) {
	t.Parallel()

	s := NewSecret("leak-me")

	var sb strings.Builder
	logger := slog.New(slog.NewTextHandler(&sb, nil))
	logger.Info("config", "cosmosKey", s)

	if strings.Contains(sb.String(), "leak-me") {
		t.Errorf("slog leaked the secret: %s", sb.String())
	}
}

func TestSecretString_EmptyIsEmptyExpose(t *testing.T) {
	t.Parallel()

	s := NewSecret("")
	if s.Expose() != "" {
		t.Errorf("Expose() on empty secret: got %q, want empty", s.Expose())
	}
	// Even an empty secret redacts in String — never an empty string that could
	// be mistaken for "no secret set".
	if s.String() != "[REDACTED]" {
		t.Errorf("String() on empty secret: got %q, want [REDACTED]", s.String())
	}
}
