-- Phase 1: idempotent Entra principal create for the Town Crier API managed
-- identity. Run this against the ADMIN database (default: postgres) where the
-- pgaadauth extension is provided by Azure Database for PostgreSQL. Postgres
-- roles are cluster-global, so the role is immediately usable by the app
-- database once GRANT CONNECT is issued below.
--
-- DELIBERATELY ABSENT: SUPERUSER, CREATEDB, CREATEROLE, role/database OWNERSHIP,
-- DROP, CREATE EXTENSION, and any DDL. See grants.sql for the DML grants.
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
