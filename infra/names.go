package main

import "fmt"

// Azure resource-naming conventions, extracted into pure functions so they have exactly
// one definition each. These are called both by the real resource-creation sites below
// and by names_test.go, which asserts .github/config/resource-names.env — the single
// source of truth consumed by the GitHub Actions workflows — matches what this program
// actually names each resource (issue #835 slice 6). Do not change the literal formats
// without also updating resource-names.env: Pulumi resource identity is keyed off these
// strings (see main.go), so a format change is a rename, not a refactor.

const (
	// ImageRepoAPI is the ACR repository name for the API container image. It has no
	// per-env suffix (one image repo, tagged per release, shared by dev and prod) and
	// doubles as the API's OTEL_SERVICE_NAME.
	ImageRepoAPI = "town-crier-api-go"
	// ImageRepoWorker is the ACR repository name for the worker container image. It
	// has no per-env suffix and doubles as the worker's OTEL_SERVICE_NAME.
	ImageRepoWorker = "town-crier-worker-go"
)

// ResourceGroupName returns the Azure resource group name for env ("dev" or "prod").
func ResourceGroupName(env string) string {
	return fmt.Sprintf("rg-town-crier-%s", env)
}

// ContainerAppAPIName returns the Azure Container App name for the Go API, for env.
func ContainerAppAPIName(env string) string {
	return fmt.Sprintf("ca-town-crier-api-go-%s", env)
}

// StaticWebAppName returns the Azure Static Web App (landing page) name for env.
func StaticWebAppName(env string) string {
	return fmt.Sprintf("swa-town-crier-%s", env)
}

// StorageAccountName returns the Azure Storage Account name for env. Storage account
// names are 3-24 chars, lowercase alphanumeric only, globally unique — no hyphens.
func StorageAccountName(env string) string {
	return fmt.Sprintf("sttowncrier%s", env)
}
