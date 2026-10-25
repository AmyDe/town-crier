package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
)

const (
	apiStatementTimeout = 30 * time.Second
	apiConnCheck        = 10 * time.Second

	maxConnsEnv     = "POSTGRES_MAX_CONNS"
	defaultMaxConns = 10
	minMaxConns     = 2
	maxMaxConns     = 40
)

// apiPoolOptions returns the API database pool options: a 30 second server-side
// statement timeout and a pool size read from POSTGRES_MAX_CONNS (default 10,
// allowed range 2 to 40). A value outside the range or not a whole number is an
// error so startup fails with a clear message.
func apiPoolOptions(getenv func(string) string) ([]postgres.PoolOption, error) {
	maxConns := int32(defaultMaxConns)
	if raw := getenv(maxConnsEnv); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || n < minMaxConns || n > maxMaxConns {
			return nil, fmt.Errorf("%s=%q: must be a whole number from %d to %d", maxConnsEnv, raw, minMaxConns, maxMaxConns)
		}
		maxConns = int32(n)
	}
	return []postgres.PoolOption{
		postgres.WithMaxConns(maxConns),
		postgres.WithStatementGuards(apiStatementTimeout, apiConnCheck),
	}, nil
}
