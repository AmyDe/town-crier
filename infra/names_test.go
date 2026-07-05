package main

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// resourceNamesEnvPath is the single source of truth for Azure resource names consumed
// by the GitHub Actions workflows (cd-dev.yml, cd-prod.yml, seo-refresh.yml,
// dev-container-app-cleanup.yml), relative to this package.
const resourceNamesEnvPath = "../.github/config/resource-names.env"

// parseResourceNamesEnv reads a flat KEY=value file into a map. resource-names.env is
// deliberately free of comments and blank lines — it gets cat-appended straight into
// GitHub Actions' $GITHUB_ENV, whose parser requires strict KEY=value lines — so a line
// without "=" is treated as a malformed file, not a comment to skip.
func parseResourceNamesEnv(t *testing.T, path string) map[string]string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("%s: line %q has no '=' separator", path, line)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return values
}

// TestResourceNamesEnv_MatchesNamingConventions asserts that every value committed to
// .github/config/resource-names.env matches what environment.go's own naming functions
// produce for the real Pulumi resources — the same functions the resource-creation call
// sites in environment.go use — so a name changed in one place without the other fails
// CI (issue #835 slice 6 acceptance criterion).
func TestResourceNamesEnv_MatchesNamingConventions(t *testing.T) {
	t.Parallel()

	got := parseResourceNamesEnv(t, resourceNamesEnvPath)

	cases := []struct {
		key  string
		want string
	}{
		{"RESOURCE_GROUP_DEV", ResourceGroupName("dev")},
		{"STORAGE_ACCOUNT_DEV", StorageAccountName("dev")},
		{"SWA_NAME_DEV", StaticWebAppName("dev")},
		{"CONTAINER_APP_API_DEV", ContainerAppAPIName("dev")},
		{"STORAGE_ACCOUNT_PROD", StorageAccountName("prod")},
		{"SWA_NAME_PROD", StaticWebAppName("prod")},
		{"CONTAINER_APP_API_PROD", ContainerAppAPIName("prod")},
		// The image repo names have no per-env suffix — they are the OTEL_SERVICE_NAME
		// literals shared by dev and prod, doubling as the ACR repository names.
		{"IMAGE_REPO_API", ImageRepoAPI},
		{"IMAGE_REPO_WORKER", ImageRepoWorker},
	}

	seen := make(map[string]bool, len(cases))
	for _, tc := range cases {
		seen[tc.key] = true
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()

			gotValue, ok := got[tc.key]
			if !ok {
				t.Fatalf("%s is missing from %s; the naming convention produces %q", tc.key, resourceNamesEnvPath, tc.want)
			}
			if gotValue != tc.want {
				t.Fatalf("%s=%q in %s does not match the naming convention, which produces %q — update resource-names.env (or environment.go, if the convention itself changed)", tc.key, gotValue, resourceNamesEnvPath, tc.want)
			}
		})
	}

	// Coverage check: a key added to resource-names.env without a matching case above
	// would otherwise pass silently and drift unnoticed.
	for key := range got {
		if !seen[key] {
			t.Errorf("%s defines %s, which this test does not assert — add a case to TestResourceNamesEnv_MatchesNamingConventions", resourceNamesEnvPath, key)
		}
	}
}
