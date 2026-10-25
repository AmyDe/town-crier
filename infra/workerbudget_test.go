package main

import (
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"
)

var workerModeBudgetConsts = map[string]string{
	"digest":             "digestBudget",
	"hourly-digest":      "hourlyDigestBudget",
	"dormant-cleanup":    "cycleBudget",
	"subscription-sweep": "cycleBudget",
	"appstore-reconcile": "cycleBudget",
	"pg-purge":           "cycleBudget",
	"seo-catalog":        "seoBudget",
}

func TestWorkerBudgetsStayUnderReplicaTimeouts(t *testing.T) {
	t.Parallel()
	envSrc, err := os.ReadFile("environment.go")
	if err != nil {
		t.Fatal(err)
	}
	dispatchSrc, err := os.ReadFile("../api-go/internal/worker/dispatch.go")
	if err != nil {
		t.Fatal(err)
	}

	timeouts := map[string]time.Duration{}
	jobRe := regexp.MustCompile(`createWorkerJob\(ctx, ec, "[^"]+", [^,]+, (\d+), "([^"]+)"\)`)
	for _, m := range jobRe.FindAllStringSubmatch(string(envSrc), -1) {
		secs, _ := strconv.Atoi(m[1])
		timeouts[m[2]] = time.Duration(secs) * time.Second
	}

	for mode, name := range workerModeBudgetConsts {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			timeout, ok := timeouts[mode]
			if !ok {
				t.Fatalf("no createWorkerJob call found for mode %q", mode)
			}
			m := regexp.MustCompile(`(?m)^\s*(?:const )?` + name + `\s*=\s*(\d+) \* time\.Minute`).FindStringSubmatch(string(dispatchSrc))
			if m == nil {
				t.Fatalf("budget constant %s not found in dispatch.go", name)
			}
			mins, _ := strconv.Atoi(m[1])
			budget := time.Duration(mins) * time.Minute
			if budget >= timeout {
				t.Errorf("%s budget %v must be below replica timeout %v", mode, budget, timeout)
			}
		})
	}
}
