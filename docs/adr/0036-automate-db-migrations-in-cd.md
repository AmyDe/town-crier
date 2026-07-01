# 0036. Automate DB migrations in CD

Date: 2026-07-01

## Status

Accepted

Builds on [0032](0032-consolidate-datastore-on-postgres-postgis.md) (Postgres +
PostGIS as the sole datastore). Implements the migration-automation half of GH
issue [#745](https://github.com/AmyDe/town-crier/issues/745).

## Context

On 2026-07-01 prod (v0.15.58) shipped code that queried `notifications.read_at`,
but the column did not exist: the prod database was five goose migrations behind
(v11 against the code's v16). Every notification and unread query returned 500,
breaking the applications list and the map for paying customers.

The root cause is that **CI/CD applies no database migrations.** The only applier,
`api-go/cmd/pgmigrate`, was written as a manual, one-time operation "run by the
Entra admin (a locally `az login`-ed human), not part of any deploy" (see the
package doc in `api-go/cmd/pgmigrate/main.go`). This project has no manual
operations. The agent runs everything and the owner never ran `pgmigrate`, so
migrations effectively never applied and the schema drifted silently until a
release depended on a column that was never added.

We are fixing this by running migrations automatically in CD, before the new
image serves traffic. This ADR records the **identity and execution model** for
running DDL in CD. The design has to clear four constraints that fall out of how
the database is currently secured:

1. **The app role is DML-only by design.** `api-go/cmd/pgbootstrap` creates the
   Entra-mapped role `towncrier_api` and grants it only SELECT / INSERT / UPDATE /
   DELETE (`api-go/cmd/pgbootstrap/grants.sql`, `principal.sql`); SUPERUSER,
   CREATEDB, CREATEROLE, ownership, DROP and all DDL are deliberately absent. The
   app therefore cannot migrate itself at startup. A privileged identity must run
   the DDL.

2. **Table ownership is the crux.** `grants.sql` runs
   `ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ... TO towncrier_api` with **no
   `FOR ROLE` clause**, so Postgres only auto-grants the app DML on tables created
   by *the role that executed that ALTER* — the human Entra admin who ran bootstrap.
   The existing tables (migrations 0001–0016) were created by the owner and are
   covered. But if a **different** identity runs `goose up` in future, the new
   tables are owned by that identity's role and fall outside the owner's default
   privileges, so `towncrier_api` silently gets no DML on them. That is the same
   class of outage we are fixing, one layer down. Whatever we choose must guarantee
   the app role keeps DML on tables created by future migrations, regardless of who
   owns them.

3. **Network reachability.** `psql-town-crier-shared` has
   `PublicNetworkAccess: Enabled` plus the firewall rule `allow-azure-services`
   (the all-zeros 0.0.0.0 special rule; see `infra/shared.go`). GitHub-hosted
   runners run on Azure infrastructure, so they reach the server through that rule.
   The dev API already depends on it; a developer laptop, by contrast, needs a
   manual firewall rule. This is a known security smell (any Azure tenant can reach
   the public endpoint) but it is the existing baseline, not something this ADR
   re-fixes.

4. **Passwordless auth already works.** `cmd/pgmigrate` uses
   `DefaultAzureCredential` to fetch an Entra token for scope
   `https://ossrdbms-aad.database.windows.net/.default` and uses it as the
   connection password (`migrateDSN` percent-encodes the admin UPN). In GitHub
   Actions, `azure/login@v3` (OIDC) signs the CI service principal in via the Azure
   CLI, so `DefaultAzureCredential` resolves through `AzureCLICredential` and yields
   a Postgres token for the CI SP on the runner. No stored secret is involved.

The issue framed two identity options:

- **Option A — the CI service principal becomes a second Postgres Entra admin.**
  `town-crier-github-actions` (objectId `8efcb7cf-f17e-4a93-aab5-df7bc3c2c2cc`,
  Pulumi key `ciServicePrincipalId`) is already Contributor on the subscription and
  User Access Administrator on both resource groups; it builds and pushes every
  image and runs `pulumi up` for the whole stack, so it already effectively controls
  the database server resource. Adding it as a Postgres admin is one declarative
  Pulumi resource — `infra/shared.go` already creates the human admin via
  `dbforpostgresql.NewAdministrator` and its comment notes "The CI service principal
  can be added as a second Administrator later if CI bootstrap is needed." Migrations
  then run as a gated CD job authenticating as the CI SP.

- **Option B — a dedicated migration principal** (a separate managed identity or app
  registration used only for DDL). Cleaner least-privilege in isolation, but
  provisioning a new identity and its federated credential is directory-level work
  the CI SP cannot self-provision (it can create role assignments but not app
  registrations or federated credentials). That reintroduces a manual bootstrap
  step, which violates the project's no-manual-ops principle, for marginal benefit.

## Decision

Adopt **Option A**. The CI service principal is added as a second Postgres Entra
admin, and migrations run as a gated CD job on the GitHub runner, authenticating
as the CI SP over OIDC, before the API and worker deploy. The table-ownership
hazard is closed the same way regardless of identity, so Option B's isolation buys
little against its manual-provisioning cost.

Concretely, this is the design slice 2 will implement:

- **Identity.** Add the CI SP as a second `dbforpostgresql.NewAdministrator` on the
  single shared server in `infra/shared.go`, alongside the existing
  `psql-town-crier-shared-aad-admin` (object id from `ciServicePrincipalId`,
  `PrincipalType: "ServicePrincipal"`). It is fully declarative IaC, no manual step,
  and both databases live on the one shared server so a single admin resource
  covers dev and prod. **Open point for the implementer:** for a service principal,
  the Flexible-Server AAD login name is the SP's application display name, not a UPN.
  Confirm the exact `-admin-user` value the runner must pass (expected
  `town-crier-github-actions`) and the `PrincipalName` the `NewAdministrator`
  resource needs, and verify both against the live cd-dev run before wiring prod.

- **Execution.** Add a new gated job to `.github/workflows/cd-dev.yml` and
  `.github/workflows/cd-prod.yml` that runs `pgmigrate` against the correct database
  (`town_crier_dev` / `town_crier_prod`) after infrastructure is up and **before**
  the API and worker deploy jobs. The job `needs:` the infra job (transitively
  including the shared stack, so the CI-SP admin resource exists), and the
  `api-go-deploy` / `worker-deploy` jobs (dev) and `api-go` / `worker` jobs (prod)
  gain a `needs:` on the migrate job. It authenticates via the existing
  `azure-login` action, so `DefaultAzureCredential` yields the CI SP's Postgres
  token. A non-zero exit from `pgmigrate` **fails the deploy loudly** — never a
  silent skip. `goose.Up` is idempotent, so a run against an up-to-date database is
  a no-op.

- **Table-ownership guarantee.** Every deploy, immediately after `goose up`, the
  migrator runs an idempotent post-migration grant sweep to `towncrier_api`:

  ```sql
  GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA public TO towncrier_api;
  GRANT USAGE, SELECT                ON ALL SEQUENCES IN SCHEMA public TO towncrier_api;
  ```

  This lives in the `cmd/pgmigrate` / `postgres.Migrate` path
  (`api-go/internal/platform/postgres/migrate.go`), so it runs as the same
  privileged identity that just applied the migrations. It is ownership-agnostic:
  the migrator owns the tables this deploy just created, so its `GRANT` succeeds on
  them; tables owned by another role (for example the existing 0001–0016 tables
  owned by the human admin, or PostGIS system views) emit the benign "no privileges
  were granted" warning already documented in `grants.sql` and are skipped, but they
  keep their DML from the original bootstrap grant. The sweep is safe to repeat
  every deploy because `GRANT` is idempotent. This is what stops the drift bug from
  recurring one layer down: whoever runs a migration grants the app DML on the
  tables that migration created, at creation time, without relying on
  `ALTER DEFAULT PRIVILEGES` and its owner-scoping.

- **Ordering / expand-then-contract preserved.** Migrations must complete before the
  new image serves traffic (that is the whole point of the gating `needs:`). The
  additive, expand-then-contract migration convention stays, so a schema change can
  land a release ahead of the code that depends on it.

Prod is already recovered to v16 by a manual `pgmigrate` run; this ADR does not
re-migrate prod. `town_crier_dev` has never been migrated and is brought fully up to
date by the automation's first dev run — where, because the CI SP creates every
table from scratch, the sweep grants clean DML with no mixed ownership.

## Consequences

- The schema-drift outage class is closed at the root: every deploy applies pending
  migrations before serving, and fails loudly if a migration errors, so code can no
  longer ship ahead of the schema it needs.
- The table-ownership trap is closed for good. New tables get app DML from the
  post-migrate sweep no matter which identity created them, so switching the migrator
  from the human owner to the CI SP does not silently strip `towncrier_api` of DML.
- **Least privilege regresses, honestly.** The deploy SP gains Postgres admin
  (`azure_pg_admin`-equivalent) rights on the shared server, so an identity that can
  already build images and run `pulumi up` can now also run arbitrary DDL and read or
  write every table in both databases. The marginal blast radius is small because the
  CI SP already controls the server resource itself (it can reset the admin password
  or add itself as admin through Pulumi), but this is a real widening of the
  crown-jewel identity and should be treated as such.
- **Future hardening path (documented alternative, not this ADR's choice):** move DDL
  to a dedicated migration identity (Option B) once app-registration and
  federated-credential provisioning can itself be automated end to end without a
  manual bootstrap. At that point the deploy SP can drop its Postgres admin rights and
  the migrator becomes a least-privilege, single-purpose principal.
- The public-endpoint reachability smell (constraint 3) is unchanged and inherited,
  not introduced here; tightening it (private networking / VNet integration) remains a
  separate hardening item.
- Bootstrap (the `towncrier_api` role creation and `GRANT CONNECT`, done by
  `cmd/pgbootstrap`) remains a distinct concern from migrations and is assumed already
  applied per database. Folding bootstrap into CD, if wanted, is a later step; this ADR
  covers only DDL execution and the ongoing app-DML guarantee.
