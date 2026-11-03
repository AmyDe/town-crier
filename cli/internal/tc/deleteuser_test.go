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
	body   map[string]any
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

func runDeleteUserWith(t *testing.T, server *httptest.Server, stdin string, args ...string) (int, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	env, out, errBuf := deleteUserEnv(stdin)
	code := runDeleteUser(context.Background(), clientFor(server), env, ParseArgs(append([]string{"delete-user"}, args...)))
	return code, out, errBuf
}

func TestRunDeleteUser_ConfirmedEmailErasesThroughTheAPI(t *testing.T) {
	t.Parallel()
	server, got := eraseServer(t, http.StatusOK, `{"userIds":["auth0|new","auth0|old"]}`)

	code, out, _ := runDeleteUserWith(t, server, "jo@example.com\n", "--email", "jo@example.com")

	if code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got.calls != 1 || got.method != http.MethodPost || got.path != "/v1/admin/users/erase" {
		t.Fatalf("request = %d x %s %s, want one POST /v1/admin/users/erase", got.calls, got.method, got.path)
	}
	if len(got.body) != 1 || got.body["email"] != "jo@example.com" {
		t.Fatalf("request body = %v, want only the email", got.body)
	}
	for _, want := range []string{"every account with the email jo@example.com", "cannot be undone", "Type the email again to confirm: ", "Erased user auth0|new\n", "Erased user auth0|old\n"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout = %q, want %q", out.String(), want)
		}
	}
}

func TestRunDeleteUser_ConfirmedUserIDErasesThroughTheAPI(t *testing.T) {
	t.Parallel()
	server, got := eraseServer(t, http.StatusOK, `{"userIds":["auth0|jo"]}`)

	code, out, _ := runDeleteUserWith(t, server, "auth0|jo\n", "--user-id", "auth0|jo")

	if code != exitOK || got.calls != 1 {
		t.Fatalf("exit code = %d calls = %d, want 0 and 1", code, got.calls)
	}
	if len(got.body) != 1 || got.body["userId"] != "auth0|jo" {
		t.Fatalf("request body = %v, want only the user id", got.body)
	}
	if !strings.Contains(out.String(), "Type the user id again to confirm: ") || !strings.Contains(out.String(), "Erased user auth0|jo\n") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestRunDeleteUser_ConfirmationIgnoresSurroundingWhitespace(t *testing.T) {
	t.Parallel()
	server, got := eraseServer(t, http.StatusOK, `{"userIds":["auth0|jo"]}`)

	code, _, _ := runDeleteUserWith(t, server, "  jo@example.com  \r\n", "--email", "jo@example.com")

	if code != exitOK || got.calls != 1 {
		t.Fatalf("exit code = %d calls = %d, want 0 and 1", code, got.calls)
	}
}

func TestRunDeleteUser_UnconfirmedTargetSendsNothing(t *testing.T) {
	t.Parallel()
	for name, stdin := range map[string]string{
		"different email": "jo@example.org\n",
		"different case":  "Jo@example.com\n",
		"empty line":      "\n",
		"end of input":    "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server, got := eraseServer(t, http.StatusOK, `{"userIds":["auth0|jo"]}`)

			code, _, errBuf := runDeleteUserWith(t, server, stdin, "--email", "jo@example.com")

			if code != exitUsage || got.calls != 0 {
				t.Fatalf("exit code = %d calls = %d, want 1 and 0", code, got.calls)
			}
			if !strings.Contains(errBuf.String(), "Nothing was deleted") {
				t.Fatalf("stderr = %q, want nothing-deleted message", errBuf.String())
			}
		})
	}
}

func TestRunDeleteUser_NeedsExactlyOneTarget(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"neither":     nil,
		"blank email": {"--email", " "},
		"both":        {"--email", "jo@example.com", "--user-id", "auth0|jo"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server, got := eraseServer(t, http.StatusOK, `{}`)

			code, _, errBuf := runDeleteUserWith(t, server, "", args...)

			if code != exitUsage || got.calls != 0 {
				t.Fatalf("exit code = %d calls = %d, want 1 and 0", code, got.calls)
			}
			if !strings.Contains(errBuf.String(), "Pass exactly one of --email and --user-id.") {
				t.Fatalf("stderr = %q", errBuf.String())
			}
		})
	}
}

func TestRunDeleteUser_APIFailures(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		status   int
		body     string
		wantErr  []string
		wantOut  string
		notInErr string
	}{
		"not found": {
			status:  http.StatusNotFound,
			wantErr: []string{"User not found: jo@example.com"},
		},
		"cascade failed after one erasure": {
			status:  http.StatusInternalServerError,
			body:    `{"erasedUserIds":["auth0|new"],"failedUserId":"auth0|old"}`,
			wantErr: []string{"the erasure of user auth0|old did not finish", "Finish it with: tc delete-user --user-id auth0|old"},
			wantOut: "Erased user auth0|new\n",
		},
		"lookup failed": {
			status:   http.StatusInternalServerError,
			body:     `{"erasedUserIds":[]}`,
			wantErr:  []string{"the email lookup failed. Run the same command again."},
			notInErr: "--user-id",
		},
		"unexpected body": {
			status:  http.StatusBadGateway,
			body:    "boom",
			wantErr: []string{"API error (502): boom"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server, _ := eraseServer(t, tc.status, tc.body)

			code, out, errBuf := runDeleteUserWith(t, server, "jo@example.com\n", "--email", "jo@example.com")

			if code != exitRuntime {
				t.Fatalf("exit code = %d, want 2", code)
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(errBuf.String(), want) {
					t.Fatalf("stderr = %q, want %q", errBuf.String(), want)
				}
			}
			if tc.wantOut != "" && !strings.Contains(out.String(), tc.wantOut) {
				t.Fatalf("stdout = %q, want %q", out.String(), tc.wantOut)
			}
			if tc.notInErr != "" && strings.Contains(errBuf.String(), tc.notInErr) {
				t.Fatalf("stderr = %q, must not contain %q", errBuf.String(), tc.notInErr)
			}
		})
	}
}

func TestRun_HelpListsDeleteUserCommand(t *testing.T) {
	t.Parallel()
	env, out, _ := captureEnv()

	Run(context.Background(), env, []string{"help"})

	if !strings.Contains(out.String(), "delete-user") || !strings.Contains(out.String(), "--user-id") {
		t.Fatalf("help must list delete-user and --user-id:\n%s", out.String())
	}
}
