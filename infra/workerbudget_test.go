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

	"application-locations-load":          "applicationLocationsLoadBudget",
	"application-locations-reconcile":     "applicationLocationsReconcileBudget",
	"application-locations-reconcile-all": "applicationLocationsReconcileAllBudget",
}

var workerModeBudgetSources = map[string]string{
	"application-locations-load":          "../api-go/internal/worker/applicationlocations.go",
	"application-locations-reconcile":     "../api-go/internal/worker/applicationlocations.go",
	"application-locations-reconcile-all": "../api-go/internal/worker/applicationlocations.go",
}

const defaultBudgetSource = "../api-go/internal/worker/dispatch.go"

func TestWorkerBudgetsStayUnderReplicaTimeouts(t *testing.T) {
	t.Parallel()
	envSrc, err := os.ReadFile("environment.go")
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
			srcPath, ok := workerModeBudgetSources[mode]
			if !ok {
				srcPath = defaultBudgetSource
			}
			src, err := os.ReadFile(srcPath)
			if err != nil {
				t.Fatal(err)
			}
			m := regexp.MustCompile(`(?m)^\s*(?:const )?` + name + `\s*=\s*(\d+) \* time\.Minute`).FindStringSubmatch(string(src))
			if m == nil {
				t.Fatalf("budget constant %s not found in %s", name, srcPath)
			}
			mins, _ := strconv.Atoi(m[1])
			budget := time.Duration(mins) * time.Minute
			if budget >= timeout {
				t.Errorf("%s budget %v must be below replica timeout %v", mode, budget, timeout)
			}
		})
	}
}

func TestWorkerJobNamesFitContainerAppsLimit(t *testing.T) {
	t.Parallel()
	envSrc, err := os.ReadFile("environment.go")
	if err != nil {
		t.Fatal(err)
	}
	nameRe := regexp.MustCompile(`createWorkerJob\(ctx, ec, "([^"]+)"`)
	matches := nameRe.FindAllStringSubmatch(string(envSrc), -1)
	if len(matches) == 0 {
		t.Fatal("no createWorkerJob calls found")
	}
	for _, m := range matches {
		for _, env := range []string{"dev", "prod"} {
			name := "job-tc-" + m[1] + "-" + env
			if len(name) > 32 {
				t.Errorf("job name %q is %d characters, limit is 32", name, len(name))
			}
		}
	}
}
