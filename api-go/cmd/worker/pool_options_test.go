package main

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPoolOptions_GuardsOnlyLongRunningModes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode        string
		wantTimeout string
	}{
		{"seo-catalog", "600000"},
		{"application-locations-load", "6600000"},
		{"application-locations-reconcile", "600000"},
		{"application-locations-reconcile-all", "600000"},
		{"poll", ""},
		{"digest", ""},
		{"pg-purge", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			t.Parallel()
			cfg, err := pgxpool.ParseConfig("postgres://u@localhost:5432/db?sslmode=disable")
			if err != nil {
				t.Fatalf("ParseConfig() error = %v", err)
			}
			for _, opt := range poolOptions(tc.mode) {
				opt(cfg)
			}
			params := cfg.ConnConfig.RuntimeParams
			if got := params["statement_timeout"]; got != tc.wantTimeout {
				t.Errorf("statement_timeout = %q, want %q", got, tc.wantTimeout)
			}
			wantCheck := ""
			if tc.wantTimeout != "" {
				wantCheck = "10000"
			}
			if got := params["client_connection_check_interval"]; got != wantCheck {
				t.Errorf("client_connection_check_interval = %q, want %q", got, wantCheck)
			}
		})
	}
}
