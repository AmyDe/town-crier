package tc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"text/tabwriter"
	"time"
)

const (
	readPathPath        = "/v1/admin/read-path"
	readPathSwitchUsage = "Usage: tc read-path-switch --env <dev|prod> [--thin-sync <on|off>] [--thin-list <on|off>] [--thin-map <on|off>] [--reason <text>]"
)

var errReadPathMissing = errors.New("no read-path switches (older API)")

type readPathStatus struct {
	ThinSync  bool       `json:"thinSync"`
	ThinList  bool       `json:"thinList"`
	ThinMap   bool       `json:"thinMap"`
	Source    string     `json:"source"`
	Reason    string     `json:"reason"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

type readPathSwitchFlag struct {
	flag  string
	field string
	value func(readPathStatus) bool
}

var readPathSwitchFlags = []readPathSwitchFlag{
	{"thin-sync", "thinSync", func(s readPathStatus) bool { return s.ThinSync }},
	{"thin-list", "thinList", func(s readPathStatus) bool { return s.ThinList }},
	{"thin-map", "thinMap", func(s readPathStatus) bool { return s.ThinMap }},
}

// runReadPathSwitch implements `tc read-path-switch`: it sets the named
// switches of one environment, reads the state back and verifies it. Without a
// switch flag it only prints the state.
func runReadPathSwitch(ctx context.Context, cfgs map[string]Config, env Env, args *ParsedArgs) int {
	target, err := args.GetRequired("env")
	target = strings.ToLower(target)
	if err != nil || !slices.Contains(pollingEnvNames, target) {
		fmt.Fprintln(env.Err, readPathSwitchUsage)
		return exitUsage
	}

	want := map[string]bool{}
	body := map[string]any{}
	for _, f := range readPathSwitchFlags {
		raw, ok := args.GetOptional(f.flag)
		if !ok {
			continue
		}
		switch strings.ToLower(raw) {
		case "on":
			want[f.flag], body[f.field] = true, true
		case "off":
			want[f.flag], body[f.field] = false, false
		default:
			fmt.Fprintln(env.Err, readPathSwitchUsage)
			return exitUsage
		}
	}
	client := NewClient(cfgs[target])

	if len(body) == 0 {
		st, err := readPathCall(ctx, client, http.MethodGet, nil)
		if err != nil {
			fmt.Fprintf(env.Err, "Cannot read %s: %v\n", target, err)
			return exitRuntime
		}
		writeReadPathTable(env.Out, target, st)
		return exitOK
	}

	if reason, ok := args.GetOptional("reason"); ok && reason != "" {
		body["reason"] = reason
	} else {
		body["reason"] = "tc read-path-switch --env " + target
	}
	if _, err := readPathCall(ctx, client, http.MethodPut, body); err != nil {
		fmt.Fprintf(env.Err, "Failed to set %s: %v\n", target, err)
		return exitRuntime
	}
	st, err := readPathCall(ctx, client, http.MethodGet, nil)
	if err != nil {
		fmt.Fprintf(env.Err, "Failed to read back %s: %v\n", target, err)
		return exitRuntime
	}
	for _, f := range readPathSwitchFlags {
		if w, ok := want[f.flag]; ok && f.value(st) != w {
			fmt.Fprintf(env.Err, "Failed to set %s: read back --%s=%v, want %v\n", target, f.flag, f.value(st), w)
			return exitRuntime
		}
	}
	writeReadPathTable(env.Out, target, st)
	return exitOK
}

func readPathCall(ctx context.Context, client *Client, method string, body any) (readPathStatus, error) {
	resp, err := client.do(ctx, method, readPathPath, body)
	if err != nil {
		return readPathStatus{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return readPathStatus{}, errReadPathMissing
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
		return readPathStatus{}, fmt.Errorf("API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	var st readPathStatus
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRespBytes)).Decode(&st); err != nil {
		return readPathStatus{}, fmt.Errorf("decode read-path status: %w", err)
	}
	return st, nil
}

func writeReadPathTable(w io.Writer, name string, st readPathStatus) {
	onOff := func(b bool) string {
		if b {
			return "on"
		}
		return "off"
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ENV\tTHIN_SYNC\tTHIN_LIST\tTHIN_MAP\tSOURCE\tREASON")
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", name, onOff(st.ThinSync), onOff(st.ThinList), onOff(st.ThinMap), st.Source, st.Reason)
	_ = tw.Flush()
}
