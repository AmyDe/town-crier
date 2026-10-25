package main

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func buildAPIPoolConfig(t *testing.T, getenv func(string) string) (*pgxpool.Config, error) {
	t.Helper()
	opts, err := apiPoolOptions(getenv)
	if err != nil {
		return nil, err
	}
	cfg, perr := pgxpool.ParseConfig("postgres://u@localhost:5432/db?sslmode=disable")
	if perr != nil {
		t.Fatalf("ParseConfig() error = %v", perr)
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg, nil
}

func TestAPIPoolOptions_ParsesMaxConns(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		value   string
		want    int32
		wantErr string
	}{
		{"unset defaults to 10", "", 10, ""},
		{"minimum", "2", 2, ""},
		{"maximum", "40", 40, ""},
		{"middle", "25", 25, ""},
		{"below minimum", "1", 0, "POSTGRES_MAX_CONNS"},
		{"above maximum", "41", 0, "POSTGRES_MAX_CONNS"},
		{"not a number", "many", 0, "POSTGRES_MAX_CONNS"},
		{"negative", "-3", 0, "POSTGRES_MAX_CONNS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := buildAPIPoolConfig(t, func(k string) string {
				if k == "POSTGRES_MAX_CONNS" {
					return tc.value
				}
				return ""
			})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want it to mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("apiPoolOptions() error = %v", err)
			}
			if cfg.MaxConns != tc.want {
				t.Errorf("MaxConns = %d, want %d", cfg.MaxConns, tc.want)
			}
		})
	}
}

func TestAPIPoolOptions_AppliesStatementGuards(t *testing.T) {
	t.Parallel()
	cfg, err := buildAPIPoolConfig(t, func(string) string { return "" })
	if err != nil {
		t.Fatalf("apiPoolOptions() error = %v", err)
	}
	params := cfg.ConnConfig.RuntimeParams
	if got := params["statement_timeout"]; got != "30000" {
		t.Errorf("statement_timeout = %q, want 30000", got)
	}
	if got := params["client_connection_check_interval"]; got != "10000" {
		t.Errorf("client_connection_check_interval = %q, want 10000", got)
	}
}
