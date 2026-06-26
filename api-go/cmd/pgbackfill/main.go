// Command pgbackfill copies the master planning Applications from prod Cosmos
// into the dev Postgres + PostGIS database, for the Cosmos -> Postgres migration
// (docs/memo/0010, epic #645; GH#657 Slice 3). It is idempotent and
// re-runnable: each record is written with the Postgres store's
// INSERT ... ON CONFLICT (authority_code, planit_name) DO UPDATE, so a re-run
// reconciles rather than duplicates.
//
// Source (read): prod Cosmos, authenticated with the account KEY (read from the
// COSMOS_KEY environment variable, never a flag, so it stays out of shell
// history). Applications are public planning data; WatchZones and any per-user
// PII are deliberately out of scope. The tool enumerates the whole Applications
// container with a cross-partition `SELECT * FROM c` query, paged via a
// continuation token.
//
// Target (write): dev Postgres, authenticated passwordless as the Entra ADMIN
// (DefaultAzureCredential) — the owner/admin role has full DML, whereas the
// least-priv towncrier_api managed identity is only usable from inside Azure.
//
// A record with an absent or malformed location is carried with a NULL location
// (not skipped) — see applications.DecodeDocument and the Postgres Upsert.
//
// Usage (run locally, prod Cosmos key in the environment):
//
//	export COSMOS_KEY="$(az cosmosdb keys list -n cosmos-town-crier-shared \
//	    -g rg-town-crier-shared --type keys --query primaryMasterKey -o tsv)"
//	pgbackfill -pg-host <fqdn> -pg-admin-user <aad-admin-upn> \
//	    [-cosmos-db town-crier-prod] [-pg-db town_crier_dev] \
//	    [-batch-size 500] [-limit 0]
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
)

// defaultCosmosEndpoint is the shared Cosmos account hosting prod and dev.
const defaultCosmosEndpoint = "https://cosmos-town-crier-shared.documents.azure.com:443/"

// defaultCosmosDB is the prod logical database the backfill reads from.
const defaultCosmosDB = "town-crier-prod"

// defaultPostgresDB is the dev database the backfill writes to.
const defaultPostgresDB = "town_crier_dev"

// enumerateQuery reads every application across all partitions. Plain
// cross-partition reads are allowed by the Cosmos Gateway; only ORDER BY /
// aggregates are refused cross-partition, so this uses neither.
const enumerateQuery = "SELECT * FROM c"

// docPager yields successive pages of raw Applications-container document
// bodies. The azcosmos query pager satisfies it via cosmosPager; tests inject a
// hand-written fake. Defining it consumer-side keeps the backfill loop testable
// with no network.
type docPager interface {
	More() bool
	NextPage(ctx context.Context) ([][]byte, error)
}

// appUpserter writes one application idempotently. *applications.PostgresStore
// satisfies it; tests inject a fake.
type appUpserter interface {
	Upsert(ctx context.Context, a applications.PlanningApplication) error
}

// backfillOptions tune the run: limit caps the number of records processed (0 =
// all, a small value enables a smoke run); logEvery sets the progress-log
// cadence in records (0 = no progress logs).
type backfillOptions struct {
	limit    int
	logEvery int
}

// summary is the final tally the tool prints.
type summary struct {
	read     int
	upserted int
	errors   int
}

// backfill drains every page from pager, decodes each document with the shared
// applications transform, and upserts it into store. It is the testable core,
// decoupled from Cosmos and Postgres by the docPager / appUpserter seams.
//
// Resilience: an undecodable document or a single failed Upsert is counted and
// skipped (the run is re-runnable, so a later run reconciles) — only a source
// paging error or context cancellation aborts the whole run with an error.
func backfill(ctx context.Context, pager docPager, store appUpserter, opts backfillOptions, logger *slog.Logger) (summary, error) {
	var s summary
	for pager.More() {
		if opts.limit > 0 && s.read >= opts.limit {
			break
		}
		if err := ctx.Err(); err != nil {
			return s, fmt.Errorf("backfill cancelled after %d records: %w", s.read, err)
		}
		page, err := pager.NextPage(ctx)
		if err != nil {
			return s, fmt.Errorf("read source page after %d records: %w", s.read, err)
		}
		for _, raw := range page {
			if opts.limit > 0 && s.read >= opts.limit {
				break
			}
			s.read++
			app, derr := applications.DecodeDocument(raw)
			if derr != nil {
				s.errors++
				logger.WarnContext(ctx, "skip undecodable application", "error", derr)
				continue
			}
			if uerr := store.Upsert(ctx, app); uerr != nil {
				if ctx.Err() != nil {
					return s, fmt.Errorf("backfill cancelled upserting %q after %d records: %w", app.Name, s.read, uerr)
				}
				s.errors++
				logger.WarnContext(ctx, "upsert failed", "application", app.Name, "authority", app.AreaID, "error", uerr)
				continue
			}
			s.upserted++
			if opts.logEvery > 0 && s.read%opts.logEvery == 0 {
				logger.InfoContext(ctx, "backfill progress", "read", s.read, "upserted", s.upserted, "errors", s.errors)
			}
		}
	}
	return s, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	os.Exit(run(os.Args[1:], logger))
}

func run(args []string, logger *slog.Logger) int {
	fs := flag.NewFlagSet("pgbackfill", flag.ContinueOnError)
	var (
		cosmosEndpoint  = fs.String("cosmos-endpoint", defaultCosmosEndpoint, "source Cosmos account endpoint")
		cosmosDB        = fs.String("cosmos-db", defaultCosmosDB, "source Cosmos logical database")
		cosmosContainer = fs.String("cosmos-container", platform.CosmosApplicationsContainer, "source Cosmos container id")
		pgHost          = fs.String("pg-host", os.Getenv("POSTGRES_HOST"), "target Postgres server FQDN")
		pgDB            = fs.String("pg-db", defaultPostgresDB, "target Postgres database")
		pgAdminUser     = fs.String("pg-admin-user", os.Getenv("POSTGRES_ADMIN_USER"), "Entra admin principal (UPN) to write as")
		pgSSLMode       = fs.String("pg-sslmode", envOr("POSTGRES_SSLMODE", "require"), "target Postgres sslmode")
		batchSize       = fs.Int("batch-size", 500, "Cosmos page size and progress-log cadence")
		limit           = fs.Int("limit", 0, "max records to process (0 = all; a small value smoke-tests)")
		timeout         = fs.Duration("timeout", 0, "overall timeout (0 = none; SIGINT/SIGTERM still cancel)")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cosmosKey := os.Getenv("COSMOS_KEY")
	if cosmosKey == "" {
		fmt.Fprintln(os.Stderr, "pgbackfill: COSMOS_KEY environment variable is required (read-only prod Cosmos account key)")
		return 2
	}
	if *pgHost == "" || *pgAdminUser == "" {
		fmt.Fprintln(os.Stderr, "pgbackfill: -pg-host and -pg-admin-user are required")
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	pager, err := newCosmosPager(*cosmosEndpoint, cosmosKey, *cosmosDB, *cosmosContainer, *batchSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbackfill: build cosmos source: %v\n", err)
		return 1
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbackfill: build azure credential: %v\n", err)
		return 1
	}
	pool, err := postgres.NewTokenCredentialPool(ctx, postgres.ConnParams{
		Host:    *pgHost,
		DB:      *pgDB,
		User:    *pgAdminUser,
		SSLMode: *pgSSLMode,
	}, cred)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbackfill: build postgres pool: %v\n", err)
		return 1
	}
	defer pool.Close()

	store := applications.NewPostgresStore(pool)

	logger.InfoContext(ctx, "backfill starting",
		"cosmosEndpoint", *cosmosEndpoint, "cosmosDb", *cosmosDB, "cosmosContainer", *cosmosContainer,
		"pgHost", *pgHost, "pgDb", *pgDB, "batchSize", *batchSize, "limit", *limit)

	started := time.Now()
	s, err := backfill(ctx, pager, store, backfillOptions{limit: *limit, logEvery: *batchSize}, logger)
	logger.InfoContext(ctx, "backfill complete",
		"read", s.read, "upserted", s.upserted, "errors", s.errors, "elapsed", time.Since(started).String())

	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbackfill: aborted: %v\n", err)
		return 1
	}
	if s.errors > 0 {
		// Per-record errors do not fail the run (re-running reconciles), but make
		// the count loud so the operator can decide whether to re-run.
		logger.WarnContext(ctx, "backfill finished with per-record errors; re-run to reconcile", "errors", s.errors)
	}
	return 0
}

// cosmosPager adapts the azcosmos query pager to the docPager seam.
type cosmosPager struct {
	pager *runtime.Pager[azcosmos.QueryItemsResponse]
}

func (p *cosmosPager) More() bool { return p.pager.More() }

func (p *cosmosPager) NextPage(ctx context.Context) ([][]byte, error) {
	resp, err := p.pager.NextPage(ctx)
	if err != nil {
		return nil, fmt.Errorf("cosmos next page: %w", err)
	}
	return resp.Items, nil
}

// newCosmosPager builds a key-authenticated Cosmos client and returns a
// cross-partition pager over the whole container. This is thin, untested wiring
// around the official SDK.
func newCosmosPager(endpoint, key, db, container string, pageSize int) (docPager, error) {
	cred, err := azcosmos.NewKeyCredential(key)
	if err != nil {
		return nil, fmt.Errorf("build cosmos key credential: %w", err)
	}
	client, err := azcosmos.NewClientWithKey(endpoint, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("build cosmos client: %w", err)
	}
	c, err := client.NewContainer(db, container)
	if err != nil {
		return nil, fmt.Errorf("open container %q in %q: %w", container, db, err)
	}
	opts := &azcosmos.QueryOptions{PageSizeHint: clampPageSize(pageSize)}
	// An empty partition key drives the cross-partition path (default
	// cross-partition flag), matching platform.CosmosContainer's cross-partition reads.
	return &cosmosPager{pager: c.NewQueryItemsPager(enumerateQuery, azcosmos.NewPartitionKey(), opts)}, nil
}

// clampPageSize bounds the requested page size into the SDK's int32 hint, with a
// 1000-item ceiling (the Cosmos max item count per page) and a 100 default for a
// non-positive request.
func clampPageSize(n int) int32 {
	const maxPageSize = 1000
	if n <= 0 {
		return 100
	}
	if n > maxPageSize {
		return maxPageSize
	}
	return int32(n) //nolint:gosec // n is bounded to [1,1000] above, no overflow
}

// envOr returns the environment variable value for key, or fallback when unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
