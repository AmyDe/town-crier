# 0038. Dedicated least-privilege identity + Postgres role for the dev-seed job's prod read

Date: 2026-07-04

## Status

Accepted

## Context

The dev-seed job (epic tc-grvu, GH #808) is a new hourly, dev-only Container
Apps Job that reads a small number of recently-changed applications from
`town_crier_prod` (read-only) and mirrors them into `town_crier_dev`, so that
a TestFlight build pointed at dev can exercise the real push-notification
pipeline. This is the first process that needs to read `town_crier_prod` from
outside the prod environment.

Researching how to grant it access surfaced a pre-existing gap: every
Container App and Job in **both** dev and prod today shares one Azure
user-assigned managed identity, `id-town-crier-cosmos-data`
(`infra/shared.go`), which is mapped to one cluster-global Postgres role,
`towncrier_api` (`api-go/cmd/pgbootstrap/grants.sql`). Postgres roles are
cluster-global and `grants.sql` is applied per-database, so `towncrier_api`
already holds full `SELECT/INSERT/UPDATE/DELETE` on **both** `town_crier_dev`
and `town_crier_prod` — there is no Postgres-level boundary between the two
databases today; isolation is by convention (which `POSTGRES_DB` a given
service happens to point at), not by grant.

The simplest way to ship the dev-seed job would have been to reuse
`cosmosDataIdentity`/`towncrier_api` and simply point a second pool at
`town_crier_prod` with `POSTGRES_DB` overridden. This was flagged to the user
directly before implementation; the user chose to fix it properly for this
new job rather than lean on the existing broad grant.

## Decision

The dev-seed job gets its own, dedicated Azure user-assigned managed identity,
`id-town-crier-dev-seed-reader` (`infra/shared.go`), mapped via
`pgbootstrap -readonly` to a new, dedicated Postgres role,
`towncrier_dev_seed_reader`, holding only:

```sql
GRANT USAGE ON SCHEMA public TO towncrier_dev_seed_reader;
GRANT SELECT ON applications TO towncrier_dev_seed_reader;
```

(`api-go/cmd/pgbootstrap/grants_readonly.sql`), deliberately omitting
`INSERT`/`UPDATE`/`DELETE`, sequence grants, and
`ALTER DEFAULT PRIVILEGES` — this role can read one table and nothing else,
and gains no rights on tables created by future migrations.

`cmd/pgbootstrap` gains a `-readonly` flag selecting `grants_readonly.sql` in
place of `grants.sql` for Phase 2; Phase 1 (`principal.sql`, the Entra
principal-create step) is unchanged and reused as-is, since it is already
generic. This identity is attached only to the dev-seed Container Apps Job
(`infra/environment.go`, tc-grvu.6) — no other Container App or Job gains it.

Rationale: a bug in the new job's SQL (or in any future job built the same
way) must not be able to write to prod. A dedicated identity mapped to a
`SELECT`-only role makes that structurally impossible rather than relying on
the job's own code discipline.

**Explicitly not addressed here:** the pre-existing condition that
`cosmosDataIdentity`/`towncrier_api` already has full DML on both databases.
Fixing that would touch every existing prod and dev service's credentials —
a much larger, higher-risk change than this QA-tooling feature warrants, and
must not happen as an incidental side effect of it. It is tracked separately
as tc-hif3.

The one-time `pgbootstrap -readonly` run against `town_crier_prod` that
actually creates `towncrier_dev_seed_reader` is a manual, Entra-admin-run CLI
step performed after this infra change merges and deploys — consistent with
`pgbootstrap`'s existing posture of never running as part of CD.

## Consequences

- The dev-seed job's blast radius on `town_crier_prod` is bounded at the
  Postgres grant level, not just by application code: even a bug that
  constructs the wrong SQL against the prod pool cannot mutate data.
- A second managed identity and Postgres role to provision and keep track of,
  and a second one-time manual `pgbootstrap` run per environment stack that
  needs it (documented in the dev-seed job's PR).
- `cmd/pgbootstrap` now supports two Phase-2 grant shapes (full DML,
  read-only); any future job needing a similarly scoped read can reuse
  `-readonly` rather than growing a third bespoke bootstrap path.
- The broader, pre-existing lack of a Postgres-level dev/prod boundary for
  every other service (`towncrier_api` on both databases) remains
  unaddressed and is tracked separately as tc-hif3; this ADR narrows the gap
  for one new consumer without closing it project-wide.
