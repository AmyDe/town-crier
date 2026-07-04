-- Phase 2 (read-only variant): idempotent SELECT-only grant for a
-- least-privilege reader role. Run this against the APP database (e.g.
-- town_crier_prod) AFTER principal.sql has been executed against the admin
-- database. Every GRANT is idempotent.
--
-- Selected with `-readonly` in place of grants.sql when the mapped managed
-- identity must only ever read, never write — e.g. a dev-only job reading
-- prod data (see docs/adr/0038-dev-seed-least-privilege-prod-read.md).
--
-- DELIBERATELY ABSENT: INSERT, UPDATE, DELETE, sequence grants, and
-- ALTER DEFAULT PRIVILEGES, on top of the same withheld privileges as
-- grants.sql (SUPERUSER, CREATEDB, CREATEROLE, role/database OWNERSHIP,
-- DROP, CREATE EXTENSION, and any DDL). This role can read the applications
-- table and nothing else; it never gains rights to newly created tables.
--
-- Identifiers are templated in by cmd/pgbootstrap after validation.

GRANT USAGE ON SCHEMA public TO {{.Role}};
GRANT SELECT ON applications TO {{.Role}};
