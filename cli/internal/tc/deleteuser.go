package tc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const deleteUserUsage = "Usage: tc delete-user --email <email>"

// deleteUserRequest is the POST /v1/admin/users/erase body.
type deleteUserRequest struct {
	Email string `json:"email"`
}

type deleteUserResponse struct {
	UserID string `json:"userId"`
}

func runDeleteUser(ctx context.Context, client *Client, env Env, args *ParsedArgs) int {
	email, err := args.GetRequired("email")
	if err != nil || strings.TrimSpace(email) == "" {
		fmt.Fprintln(env.Err, "Missing required argument: --email")
		fmt.Fprintln(env.Err, deleteUserUsage)
		return exitUsage
	}
	email = strings.TrimSpace(email)

	fmt.Fprintf(env.Out, "WARNING: This permanently erases the account for %s:\n", email)
	fmt.Fprintln(env.Out, "its profile, watch zones, saved applications, notifications, devices and Auth0 login.")
	fmt.Fprintln(env.Out, "This cannot be undone.")
	fmt.Fprint(env.Out, "Type the email again to confirm: ")
	line, _ := bufio.NewReader(env.In).ReadString('\n')
	fmt.Fprintln(env.Out)
	if strings.TrimSpace(line) != email {
		fmt.Fprintln(env.Err, "The email did not match. Nothing was deleted.")
		return exitUsage
	}

	resp, err := client.Post(ctx, "/v1/admin/users/erase", deleteUserRequest{Email: email})
	if err != nil {
		fmt.Fprintf(env.Err, "API error: %s\n", err)
		return exitRuntime
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		fmt.Fprintf(env.Err, "User not found: %s. The match is exact, so try 'tc list-users --search <term>'.\n", email)
		return exitRuntime
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
		fmt.Fprintf(env.Err, "API error (%d): %s\n", resp.StatusCode, string(body))
		fmt.Fprintln(env.Err, "The erasure did not finish. Run the command again; the API log has the user id.")
		return exitRuntime
	}

	var out deleteUserResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRespBytes)).Decode(&out); err != nil {
		fmt.Fprintf(env.Err, "The account was erased, but the response could not be read: %s\n", err)
		return exitRuntime
	}
	fmt.Fprintf(env.Out, "Erased user %s (%s)\n", out.UserID, email)
	return exitOK
}
