-- Phase 2: idempotent DML grants for the Town Crier API managed identity.
-- Run this against the APP database (default: town_crier_dev) AFTER principal.sql
-- has been executed against the admin database. Every GRANT is idempotent.
--
-- The WARNING "no privileges were granted for geography_columns/geometry_columns"
-- is expected and benign: those are PostGIS system views owned by another role.
-- ON ALL TABLES is correct here; do not enumerate app tables to suppress it.
--
-- DELIBERATELY ABSENT: SUPERUSER, CREATEDB, CREATEROLE, role/database OWNERSHIP,
-- DROP, CREATE EXTENSION, and any DDL. The API reads and writes rows; it never
-- alters schema. Schema ownership and migrations are a separate concern.
--
-- Identifiers are templated in by cmd/pgbootstrap after validation.

GRANT USAGE ON SCHEMA public TO {{.Role}};
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO {{.Role}};
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO {{.Role}};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO {{.Role}};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO {{.Role}};
