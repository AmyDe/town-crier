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

const deleteUserUsage = "Usage: tc delete-user --email <email> | --user-id <id>"

// deleteUserRequest is the POST /v1/admin/users/erase body; exactly one field
// is set.
type deleteUserRequest struct {
	Email  string `json:"email,omitempty"`
	UserID string `json:"userId,omitempty"`
}

type deleteUserResponse struct {
	UserIDs []string `json:"userIds"`
}

type deleteUserFailure struct {
	ErasedUserIDs []string `json:"erasedUserIds"`
	FailedUserID  string   `json:"failedUserId"`
}

func runDeleteUser(ctx context.Context, client *Client, env Env, args *ParsedArgs) int {
	email, hasEmail := args.GetOptional("email")
	userID, hasUserID := args.GetOptional("user-id")
	email, userID = strings.TrimSpace(email), strings.TrimSpace(userID)
	hasEmail, hasUserID = hasEmail && email != "", hasUserID && userID != ""
	if hasEmail == hasUserID {
		fmt.Fprintln(env.Err, "Pass exactly one of --email and --user-id.")
		fmt.Fprintln(env.Err, deleteUserUsage)
		return exitUsage
	}

	req, target, label := deleteUserRequest{Email: email}, email, "email"
	if hasUserID {
		req, target, label = deleteUserRequest{UserID: userID}, userID, "user id"
	}

	if hasEmail {
		fmt.Fprintf(env.Out, "WARNING: This permanently erases every account with the email %s:\n", target)
	} else {
		fmt.Fprintf(env.Out, "WARNING: This permanently erases the account %s:\n", target)
	}
	fmt.Fprintln(env.Out, "its profile, watch zones, saved applications, notifications, devices and Auth0 login.")
	fmt.Fprintln(env.Out, "This cannot be undone.")
	fmt.Fprintf(env.Out, "Type the %s again to confirm: ", label)
	line, _ := bufio.NewReader(env.In).ReadString('\n')
	fmt.Fprintln(env.Out)
	if strings.TrimSpace(line) != target {
		fmt.Fprintf(env.Err, "The %s did not match. Nothing was deleted.\n", label)
		return exitUsage
	}

	resp, err := client.Post(ctx, "/v1/admin/users/erase", req)
	if err != nil {
		fmt.Fprintf(env.Err, "API error: %s\n", err)
		return exitRuntime
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))

	if resp.StatusCode == http.StatusNotFound {
		fmt.Fprintf(env.Err, "User not found: %s. The match is exact, so try 'tc list-users --search <term>'.\n", target)
		return exitRuntime
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		reportDeleteUserFailure(env, resp.StatusCode, body)
		return exitRuntime
	}

	var out deleteUserResponse
	if err := json.Unmarshal(body, &out); err != nil {
		fmt.Fprintf(env.Err, "The erasure finished, but the response could not be read: %s\n", err)
		return exitRuntime
	}
	for _, id := range out.UserIDs {
		fmt.Fprintf(env.Out, "Erased user %s\n", id)
	}
	return exitOK
}

func reportDeleteUserFailure(env Env, status int, body []byte) {
	var failure deleteUserFailure
	if status != http.StatusInternalServerError || json.Unmarshal(body, &failure) != nil {
		fmt.Fprintf(env.Err, "API error (%d): %s\n", status, string(body))
		return
	}
	for _, id := range failure.ErasedUserIDs {
		fmt.Fprintf(env.Out, "Erased user %s\n", id)
	}
	if failure.FailedUserID == "" {
		fmt.Fprintln(env.Err, "API error (500): the email lookup failed. Run the same command again.")
		return
	}
	fmt.Fprintf(env.Err, "API error (500): the erasure of user %s did not finish.\n", failure.FailedUserID)
	fmt.Fprintf(env.Err, "Finish it with: tc delete-user --user-id %s\n", failure.FailedUserID)
}
