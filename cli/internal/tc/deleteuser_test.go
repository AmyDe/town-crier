package tc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type eraseCapture struct {
	calls  int
	method string
	path   string
	body   deleteUserRequest
}

func eraseServer(t *testing.T, status int, respBody string) (*httptest.Server, *eraseCapture) {
	t.Helper()
	c := &eraseCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.calls++
		c.method, c.path = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&c.body)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(server.Close)
	return server, c
}

func deleteUserEnv(stdin string) (Env, *bytes.Buffer, *bytes.Buffer) {
	var out, errBuf bytes.Buffer
	return Env{In: strings.NewReader(stdin), Out: &out, Err: &errBuf}, &out, &errBuf
}

func TestRunDeleteUser_ConfirmedEmailErasesThroughTheAPI(t *testing.T) {
	t.Parallel()
	server, got := eraseServer(t, http.StatusOK, `{"userId":"auth0|jo"}`)
	env, out, _ := deleteUserEnv("jo@example.com\n")

	code := runDeleteUser(context.Background(), clientFor(server), env, ParseArgs([]string{"delete-user", "--email", "jo@example.com"}))

	if code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got.calls != 1 || got.method != http.MethodPost || got.path != "/v1/admin/users/erase" {
		t.Fatalf("request = %d x %s %s, want one POST /v1/admin/users/erase", got.calls, got.method, got.path)
	}
	if got.body != (deleteUserRequest{Email: "jo@example.com"}) {
		t.Fatalf("request body = %+v", got.body)
	}
	if !strings.Contains(out.String(), "cannot be undone") || !strings.Contains(out.String(), "Type the email again to confirm: ") {
		t.Fatalf("stdout = %q, want warning and prompt", out.String())
	}
	if !strings.Contains(out.String(), "Erased user auth0|jo (jo@example.com)") {
		t.Fatalf("stdout = %q, want erased line", out.String())
	}
}

func TestRunDeleteUser_ConfirmationIgnoresSurroundingWhitespace(t *testing.T) {
	t.Parallel()
	server, got := eraseServer(t, http.StatusOK, `{"userId":"auth0|jo"}`)
	env, _, _ := deleteUserEnv("  jo@example.com  \r\n")

	code := runDeleteUser(context.Background(), clientFor(server), env, ParseArgs([]string{"delete-user", "--email", "jo@example.com"}))

	if code != exitOK || got.calls != 1 {
		t.Fatalf("exit code = %d calls = %d, want 0 and 1", code, got.calls)
	}
}

func TestRunDeleteUser_UnconfirmedEmailSendsNothing(t *testing.T) {
	t.Parallel()
	for name, stdin := range map[string]string{
		"different email": "jo@example.org\n",
		"different case":  "Jo@example.com\n",
		"empty line":      "\n",
		"end of input":    "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server, got := eraseServer(t, http.StatusOK, `{"userId":"auth0|jo"}`)
			env, _, errBuf := deleteUserEnv(stdin)

			code := runDeleteUser(context.Background(), clientFor(server), env, ParseArgs([]string{"delete-user", "--email", "jo@example.com"}))

			if code != exitUsage || got.calls != 0 {
				t.Fatalf("exit code = %d calls = %d, want 1 and 0", code, got.calls)
			}
			if !strings.Contains(errBuf.String(), "Nothing was deleted") {
				t.Fatalf("stderr = %q, want nothing-deleted message", errBuf.String())
			}
		})
	}
}

func TestRunDeleteUser_MissingEmailIsUsageError(t *testing.T) {
	t.Parallel()
	server, got := eraseServer(t, http.StatusOK, `{}`)
	env, _, errBuf := deleteUserEnv("")

	code := runDeleteUser(context.Background(), clientFor(server), env, ParseArgs([]string{"delete-user"}))

	if code != exitUsage || got.calls != 0 {
		t.Fatalf("exit code = %d calls = %d, want 1 and 0", code, got.calls)
	}
	if !strings.Contains(errBuf.String(), "Missing required argument: --email") {
		t.Fatalf("stderr = %q", errBuf.String())
	}
}

func TestRunDeleteUser_APIFailures(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		status  int
		body    string
		wantErr string
	}{
		"not found":    {http.StatusNotFound, "", "User not found: jo@example.com"},
		"server error": {http.StatusInternalServerError, "boom", "API error (500): boom"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server, _ := eraseServer(t, tc.status, tc.body)
			env, _, errBuf := deleteUserEnv("jo@example.com\n")

			code := runDeleteUser(context.Background(), clientFor(server), env, ParseArgs([]string{"delete-user", "--email", "jo@example.com"}))

			if code != exitRuntime {
				t.Fatalf("exit code = %d, want 2", code)
			}
			if !strings.Contains(errBuf.String(), tc.wantErr) {
				t.Fatalf("stderr = %q, want %q", errBuf.String(), tc.wantErr)
			}
		})
	}
}

func TestRun_HelpListsDeleteUserCommand(t *testing.T) {
	t.Parallel()
	env, out, _ := captureEnv()

	Run(context.Background(), env, []string{"help"})

	if !strings.Contains(out.String(), "delete-user") {
		t.Fatalf("help must list delete-user:\n%s", out.String())
	}
}
