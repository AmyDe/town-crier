package tc

import (
	"context"
	"strings"
	"testing"
)

func TestRun_Version(t *testing.T) {
	t.Parallel()
	env, out, _ := captureEnv()
	code := Run(context.Background(), env, []string{"version"})
	if code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out.String() != "tc 0.1.0\n" {
		t.Fatalf("stdout = %q, want %q", out.String(), "tc 0.1.0\n")
	}
}

func TestRun_Help(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}, nil} {
		env, out, _ := captureEnv()
		code := Run(context.Background(), env, args)
		if code != exitOK {
			t.Fatalf("args %v: exit code = %d, want 0", args, code)
		}
		got := out.String()
		if !strings.Contains(got, "tc — Town Crier admin CLI") || !strings.Contains(got, "generate-offer-codes") {
			t.Fatalf("args %v: help output unexpected:\n%s", args, got)
		}
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	t.Parallel()
	env, _, errBuf := captureEnv()
	// --url/--api-key supplied so config resolves regardless of the host machine.
	code := Run(context.Background(), env, []string{"bogus", "--url", "http://x", "--api-key", "k"})
	if code != exitUsage {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Unknown command: bogus") ||
		!strings.Contains(errBuf.String(), "Run 'tc help' for a list of commands.") {
		t.Fatalf("stderr = %q, want unknown-command guidance", errBuf.String())
	}
}

func TestRun_ConfigMissingReturns1(t *testing.T) {
	// Not parallel: mutates HOME so DefaultConfigPath points at an empty dir.
	t.Setenv("HOME", t.TempDir())
	env, _, errBuf := captureEnv()
	code := Run(context.Background(), env, []string{"list-users"})
	if code != exitUsage {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "API URL not configured") {
		t.Fatalf("stderr = %q, want URL-not-configured message", errBuf.String())
	}
}
