// Command pgbackfill-pollstate copies per-authority PollState documents from
// prod Cosmos into the Postgres `poll_state` table, for the Cosmos -> Postgres
// migration (docs/memo/0010, epic #645). It is the sibling of
// cmd/pgbackfill-zones (which copies WatchZones); this one copies the polling
// high-water-marks and pagination cursors that gate PlanIt fetches.
//
// NO Leases backfill: the Leases container holds a single ephemeral runtime-lock
// row that is re-created by the worker on first acquisition. Backfilling a stale
// lease would hold the poll lock for a TTL that has already elapsed on Cosmos,
// possibly delaying the first Postgres poll cycle. The worker creates the row
// atomically on first TryAcquire.
//
// The tool is idempotent and re-runnable: each state document is written with
// INSERT ... ON CONFLICT (authority_id) DO UPDATE, so a re-run reconciles
// rather than duplicates. A document that fails to decode or a single failed
// Save is counted and skipped (re-runnable); only a source paging error or
// context cancellation aborts the whole run.
//
// Source (read): prod Cosmos, authenticated with the account KEY (read from the
// COSMOS_KEY environment variable, never a flag, so it stays out of shell
// history). The tool enumerates the whole PollState container with a
// cross-partition `SELECT * FROM c` query.
//
// Target (write): Postgres, authenticated passwordless as the Entra ADMIN
// (DefaultAzureCredential). Pass -pg-db town_crier_prod for the prod copy; the
// default is the dev database, the safer failure mode if the flag is forgotten.
//
// Usage (run locally, prod Cosmos key in the environment):
//
//	export COSMOS_KEY="$(az cosmosdb keys list -n cosmos-town-crier-shared \
//	    -g rg-town-crier-shared --type read-only-keys --query primaryReadonlyMasterKey -o tsv)"
//	pgbackfill-pollstate -pg-host <fqdn> -pg-admin-user <aad-admin-upn> -pg-db town_crier_prod \
//	    [-cosmos-db town-crier-prod] [-batch-size 500] [-limit 0]
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

	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
)

const (
	defaultCosmosEndpoint = "https://cosmos-town-crier-shared.documents.azure.com:443/"
	defaultCosmosDB       = "town-crier-prod"
	defaultPostgresDB     = "town_crier_dev"

	// enumerateQuery reads every poll-state document across all partitions.
	// Cross-partition reads are permitted by the Cosmos Gateway; only ORDER BY /
	// aggregates are refused, so neither is used here.
	enumerateQuery = "SELECT * FROM c"
)

// docPager yields successive pages of raw PollState-container document bodies.
// The azcosmos query pager satisfies it via cosmosPager; tests inject a
// hand-written fake. Defining it consumer-side keeps the backfill loop testable
// with no network.
type docPager interface {
	More() bool
	NextPage(ctx context.Context) ([][]byte, error)
}

// pollStateUpserter writes one poll-state row idempotently. *polling.PostgresPollStateStore
// satisfies it via Save (INSERT ... ON CONFLICT (authority_id) DO UPDATE); tests
// inject a fake.
type pollStateUpserter interface {
	Save(ctx context.Context, authorityID int, lastPollTime, highWaterMark time.Time, cursor *polling.PollCursor) error
}

// backfillOptions tune the run.
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

// backfill drains every page from pager, decodes each document with
// polling.DecodeDocument, and saves it into store. It is the testable core,
// decoupled from Cosmos and Postgres by the docPager / pollStateUpserter seams.
//
// Resilience: an undecodable document or a single failed Save is counted and
// skipped (the run is re-runnable, so a later run reconciles) — only a source
// paging error or context cancellation aborts the whole run with an error.
func backfill(ctx context.Context, pager docPager, store pollStateUpserter, opts backfillOptions, logger *slog.Logger) (summary, error) {
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
			authorityID, state, derr := polling.DecodeDocument(raw)
			if derr != nil {
				s.errors++
				logger.WarnContext(ctx, "skip undecodable poll state", "error", derr)
				continue
			}
			if uerr := store.Save(ctx, authorityID, state.LastPollTime, state.HighWaterMark, state.Cursor); uerr != nil {
				if ctx.Err() != nil {
					return s, fmt.Errorf("backfill cancelled saving authority %d after %d records: %w", authorityID, s.read, uerr)
				}
				s.errors++
				logger.WarnContext(ctx, "save failed", "authority", authorityID, "error", uerr)
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
	fs := flag.NewFlagSet("pgbackfill-pollstate", flag.ContinueOnError)
	var (
		cosmosEndpoint  = fs.String("cosmos-endpoint", defaultCosmosEndpoint, "source Cosmos account endpoint")
		cosmosDB        = fs.String("cosmos-db", defaultCosmosDB, "source Cosmos logical database")
		cosmosContainer = fs.String("cosmos-container", platform.CosmosPollStateContainer, "source Cosmos container id")
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
		fmt.Fprintln(os.Stderr, "pgbackfill-pollstate: COSMOS_KEY environment variable is required")
		return 2
	}
	if *pgHost == "" || *pgAdminUser == "" {
		fmt.Fprintln(os.Stderr, "pgbackfill-pollstate: -pg-host and -pg-admin-user are required")
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
		fmt.Fprintf(os.Stderr, "pgbackfill-pollstate: build cosmos source: %v\n", err)
		return 1
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbackfill-pollstate: build azure credential: %v\n", err)
		return 1
	}
	pool, err := postgres.NewTokenCredentialPool(ctx, postgres.ConnParams{
		Host:    *pgHost,
		DB:      *pgDB,
		User:    *pgAdminUser,
		SSLMode: *pgSSLMode,
	}, cred)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbackfill-pollstate: build postgres pool: %v\n", err)
		return 1
	}
	defer pool.Close()

	store := polling.NewPostgresPollStateStore(pool)

	logger.InfoContext(ctx, "backfill starting",
		"cosmosEndpoint", *cosmosEndpoint, "cosmosDb", *cosmosDB, "cosmosContainer", *cosmosContainer,
		"pgHost", *pgHost, "pgDb", *pgDB, "batchSize", *batchSize, "limit", *limit)

	started := time.Now()
	s, err := backfill(ctx, pager, store, backfillOptions{limit: *limit, logEvery: *batchSize}, logger)
	logger.InfoContext(ctx, "backfill complete",
		"read", s.read, "upserted", s.upserted, "errors", s.errors, "elapsed", time.Since(started).String())

	if err != nil {
		fmt.Fprintf(os.Stderr, "pgbackfill-pollstate: aborted: %v\n", err)
		return 1
	}
	if s.errors > 0 {
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
// cross-partition pager over the whole PollState container.
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
	return &cosmosPager{pager: c.NewQueryItemsPager(enumerateQuery, azcosmos.NewPartitionKey(), opts)}, nil
}

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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
