-- Idempotent least-privilege bootstrap for the Town Crier API managed identity.
--
-- Run once by the Entra admin (a locally `az login`-ed human) against the target
-- Azure Database for PostgreSQL Flexible Server database. It maps the API's
-- managed identity to a Postgres role and grants DML only. It is safe to re-run:
-- the principal create is guarded by a pg_roles existence check, and every GRANT
-- is idempotent.
--
-- DELIBERATELY ABSENT: SUPERUSER, CREATEDB, CREATEROLE, role/database OWNERSHIP,
-- and any DDL/CREATE/DROP. The API reads and writes rows; it never alters schema.
-- Schema ownership and migrations are a separate, later concern.
--
-- Identifiers are templated in by cmd/pgbootstrap, which validates the role and
-- database against a strict identifier pattern and the object id against a UUID
-- pattern before rendering, so this file is not a SQL-injection surface.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '{{.Role}}') THEN
        PERFORM pgaadauth_create_principal_with_oid('{{.Role}}', '{{.OID}}', 'service', false, false);
    END IF;
END
$$;

GRANT CONNECT ON DATABASE {{.DB}} TO {{.Role}};
GRANT USAGE ON SCHEMA public TO {{.Role}};
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO {{.Role}};
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO {{.Role}};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO {{.Role}};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO {{.Role}};
