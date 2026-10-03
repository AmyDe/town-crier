package tc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"
)

const (
	pollingPath = "/v1/admin/polling"
	// requestGap is PlanIt's minimum gap between two requests from one client.
	requestGap = 60 * time.Second
	// inFlightLimit bounds how long after a switch-off an already scheduled
	// request can still start: the pacer schedules at most one spacing ahead.
	inFlightLimit = 60 * time.Second
)

const pollingSwitchUsage = "Usage: tc polling-switch --to <dev|prod|off> [--reason <text>] [--allow-missing true]"

// pollingEnvNames is the fixed order both polling commands report in.
var pollingEnvNames = []string{"dev", "prod"}

var errPollingMissing = errors.New("no polling switch (older API)")

type pollingStatus struct {
	Enabled        bool       `json:"enabled"`
	Source         string     `json:"source"`
	Reason         string     `json:"reason"`
	UpdatedAt      *time.Time `json:"updatedAt"`
	LastCallAt     *time.Time `json:"lastCallAt"`
	LastCallStatus *int       `json:"lastCallStatus"`
	CallsToday     int        `json:"callsToday"`
}

type pollingSetRequest struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

// pollingEnv is one environment's client and its last known status. Err is set
// when the status could not be read.
type pollingEnv struct {
	name   string
	client *Client
	status pollingStatus
	err    error
}

type pollingClock struct {
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

var realPollingClock = pollingClock{now: time.Now, sleep: sleepContext}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func newPollingEnvs(cfgs map[string]Config) []*pollingEnv {
	envs := make([]*pollingEnv, 0, len(pollingEnvNames))
	for _, name := range pollingEnvNames {
		envs = append(envs, &pollingEnv{name: name, client: NewClient(cfgs[name])})
	}
	return envs
}

func (e *pollingEnv) refresh(ctx context.Context) {
	e.status, e.err = e.call(ctx, http.MethodGet, nil)
}

func (e *pollingEnv) set(ctx context.Context, enabled bool, reason string) error {
	st, err := e.call(ctx, http.MethodPut, pollingSetRequest{Enabled: enabled, Reason: reason})
	if err != nil {
		return err
	}
	if st.Enabled != enabled {
		return fmt.Errorf("write returned enabled=%v", st.Enabled)
	}
	e.refresh(ctx)
	if e.err != nil {
		return fmt.Errorf("read back: %w", e.err)
	}
	if e.status.Enabled != enabled {
		return fmt.Errorf("read back enabled=%v, want %v", e.status.Enabled, enabled)
	}
	return nil
}

func (e *pollingEnv) call(ctx context.Context, method string, body any) (pollingStatus, error) {
	resp, err := e.client.do(ctx, method, pollingPath, body)
	if err != nil {
		return pollingStatus{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return pollingStatus{}, errPollingMissing
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
		return pollingStatus{}, fmt.Errorf("API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	var st pollingStatus
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRespBytes)).Decode(&st); err != nil {
		return pollingStatus{}, fmt.Errorf("decode polling status: %w", err)
	}
	return st, nil
}

// runPollingStatus implements `tc polling-status`: one row per environment.
func runPollingStatus(ctx context.Context, cfgs map[string]Config, env Env) int {
	envs := newPollingEnvs(cfgs)
	failed := false
	for _, e := range envs {
		e.refresh(ctx)
		failed = failed || e.err != nil
	}
	writePollingTable(env.Out, envs)
	if failed {
		return exitRuntime
	}
	return exitOK
}

// runPollingSwitch implements `tc polling-switch`: it turns every environment
// except the target off, waits until no request from them can still be less
// than a minute before the target's first request, then turns the target on.
// Each write is verified by reading it back. It never leaves two environments
// on: a failure stops before the target is turned on.
func runPollingSwitch(ctx context.Context, cfgs map[string]Config, env Env, args *ParsedArgs, clock pollingClock) int {
	target, err := args.GetRequired("to")
	target = strings.ToLower(target)
	if err != nil || (target != "off" && !isPollingEnv(target)) {
		fmt.Fprintln(env.Err, pollingSwitchUsage)
		return exitUsage
	}
	reason, ok := args.GetOptional("reason")
	if !ok || reason == "" {
		reason = "tc polling-switch --to " + target
	}
	allowMissing, _ := args.GetOptional("allow-missing")

	envs := newPollingEnvs(cfgs)
	for _, e := range envs {
		e.refresh(ctx)
	}
	fmt.Fprintln(env.Out, "Before:")
	writePollingTable(env.Out, envs)

	for _, e := range envs {
		if e.err == nil || (errors.Is(e.err, errPollingMissing) && e.name != target && allowMissing == "true") {
			continue
		}
		fmt.Fprintf(env.Err, "Cannot read %s: %v. Nothing changed.\n", e.name, e.err)
		if errors.Is(e.err, errPollingMissing) && e.name != target {
			fmt.Fprintf(env.Err, "Pass --allow-missing true to leave %s uncontrolled.\n", e.name)
		}
		return exitRuntime
	}

	readyAt := time.Time{}
	for _, e := range envs {
		if e.name == target || e.err != nil {
			continue
		}
		wasOn := e.status.Enabled
		if wasOn {
			fmt.Fprintf(env.Out, "Turning %s off...\n", e.name)
			if err := e.set(ctx, false, reason); err != nil {
				return pollingFailed(ctx, env, envs, fmt.Sprintf("turn %s off", e.name), err)
			}
			if t := clock.now().Add(inFlightLimit + requestGap); t.After(readyAt) {
				readyAt = t
			}
		}
		if e.status.LastCallAt != nil {
			if t := e.status.LastCallAt.Add(requestGap); t.After(readyAt) {
				readyAt = t
			}
		}
	}

	if target == "off" {
		return pollingDone(ctx, env, envs)
	}
	var te *pollingEnv
	for _, e := range envs {
		if e.name == target {
			te = e
		}
	}
	if te.status.Enabled {
		fmt.Fprintf(env.Out, "%s is already on.\n", target)
		return pollingDone(ctx, env, envs)
	}
	if wait := readyAt.Sub(clock.now()); wait > 0 {
		fmt.Fprintf(env.Out, "Waiting %s so the environments stay at least %s apart...\n", wait.Round(time.Second), requestGap)
		if err := clock.sleep(ctx, wait); err != nil {
			return pollingFailed(ctx, env, envs, "wait", err)
		}
	}
	fmt.Fprintf(env.Out, "Turning %s on...\n", target)
	if err := te.set(ctx, true, reason); err != nil {
		return pollingFailed(ctx, env, envs, fmt.Sprintf("turn %s on", target), err)
	}
	return pollingDone(ctx, env, envs)
}

func isPollingEnv(name string) bool {
	for _, n := range pollingEnvNames {
		if n == name {
			return true
		}
	}
	return false
}

func pollingDone(ctx context.Context, env Env, envs []*pollingEnv) int {
	for _, e := range envs {
		e.refresh(ctx)
	}
	fmt.Fprintln(env.Out, "After:")
	writePollingTable(env.Out, envs)
	return exitOK
}

func pollingFailed(ctx context.Context, env Env, envs []*pollingEnv, step string, err error) int {
	fmt.Fprintf(env.Err, "Failed to %s: %v\n", step, err)
	for _, e := range envs {
		e.refresh(context.WithoutCancel(ctx))
	}
	fmt.Fprintln(env.Err, "Current state:")
	writePollingTable(env.Err, envs)
	return exitRuntime
}

func writePollingTable(w io.Writer, envs []*pollingEnv) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ENV\tPOLLING\tSOURCE\tLAST CALL (UTC)\tSTATUS\tCALLS TODAY\tREASON")
	for _, e := range envs {
		if e.err != nil {
			fmt.Fprintf(tw, "%s\tunknown\t-\t-\t-\t-\t%v\n", e.name, e.err)
			continue
		}
		st := e.status
		polling := "off"
		if st.Enabled {
			polling = "on"
		}
		last, code := "-", "-"
		if st.LastCallAt != nil {
			last = st.LastCallAt.UTC().Format("2006-01-02 15:04:05")
			code = "timeout"
			if st.LastCallStatus != nil {
				code = fmt.Sprint(*st.LastCallStatus)
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n", e.name, polling, st.Source, last, code, st.CallsToday, st.Reason)
	}
	_ = tw.Flush()
}
