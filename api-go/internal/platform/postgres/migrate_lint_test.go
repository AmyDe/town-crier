package postgres

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

// guardlessConcurrentMigrations predate the invalid-index guard of 0039.
var guardlessConcurrentMigrations = map[string]bool{
	"0018_application_search_indexes.sql":             true,
	"0024_watch_zone_boundary.sql":                    true,
	"0027_applications_authority_uid_index.sql":       true,
	"0028_applications_authority_app_state_index.sql": true,
	"0036_applications_real_date_index.sql":           true,
	"0037_applications_double_slash_name_index.sql":   true,
}

var createConcurrently = regexp.MustCompile(`(?im)^\s*CREATE\s+(UNIQUE\s+)?INDEX\s+CONCURRENTLY`)

func lintConcurrentMigrations(fsys fs.FS, allow map[string]bool) []string {
	var problems []string
	files, _ := fs.Glob(fsys, "migrations/*.sql")
	for _, f := range files {
		raw, err := fs.ReadFile(fsys, f)
		if err != nil {
			problems = append(problems, f+": "+err.Error())
			continue
		}
		body := string(raw)
		if !createConcurrently.MatchString(body) {
			continue
		}
		name := f[strings.LastIndex(f, "/")+1:]
		if !hasNoTransaction(body) {
			problems = append(problems, name+": CONCURRENTLY without -- +goose NO TRANSACTION")
		}
		if !strings.Contains(body, "indisvalid") && !allow[name] {
			problems = append(problems, name+": CONCURRENTLY index build without invalid-index guard")
		}
	}
	return problems
}

func TestEmbeddedMigrations_ConcurrentIndexesAreSafe(t *testing.T) {
	t.Parallel()
	if problems := lintConcurrentMigrations(migrationsFS, guardlessConcurrentMigrations); len(problems) > 0 {
		t.Fatalf("unsafe migrations:\n%s", strings.Join(problems, "\n"))
	}
}

func TestLintConcurrentMigrations_FlagsViolations(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"migrations/1_ok.sql":      {Data: []byte("-- +goose NO TRANSACTION\n-- +goose Up\nSELECT indisvalid;\nCREATE INDEX CONCURRENTLY IF NOT EXISTS a ON t (c);\n")},
		"migrations/2_notx.sql":    {Data: []byte("-- +goose Up\nSELECT indisvalid;\nCREATE INDEX CONCURRENTLY a ON t (c);\n")},
		"migrations/3_noguard.sql": {Data: []byte("-- +goose NO TRANSACTION\n-- +goose Up\nCREATE UNIQUE INDEX CONCURRENTLY a ON t (c);\n")},
		"migrations/4_plain.sql":   {Data: []byte("-- +goose Up\nCREATE INDEX a ON t (c);\n")},
	}
	got := lintConcurrentMigrations(fsys, nil)
	if len(got) != 2 || !strings.HasPrefix(got[0], "2_notx.sql") || !strings.HasPrefix(got[1], "3_noguard.sql") {
		t.Fatalf("problems = %v", got)
	}
}

func TestHasNoTransaction(t *testing.T) {
	t.Parallel()
	if !hasNoTransaction("-- +goose NO TRANSACTION\n-- +goose Up\n") {
		t.Fatal("want true")
	}
	if hasNoTransaction("-- +goose Up\nSELECT 1;\n") {
		t.Fatal("want false")
	}
}
