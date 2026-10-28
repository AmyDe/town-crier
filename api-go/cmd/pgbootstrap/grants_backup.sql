-- Phase 2 (backup variant): idempotent SELECT-only grant for the nightly
-- pg-backup job's reader role. Run against the APP database AFTER principal.sql
-- has been executed against the admin database. Selected with `-backup`.
--
-- Covers the user-owned tables in internal/pgbackup.Tables and
-- goose_db_version, which the manifest reads for the schema version. A guard
-- test keeps this file in step with that list.
--
-- DELIBERATELY ABSENT: INSERT, UPDATE, DELETE, sequence grants and ALTER
-- DEFAULT PRIVILEGES. A bug in the backup job cannot write, and the role never
-- gains rights to newly created tables.
--
-- Identifiers are templated in by cmd/pgbootstrap after validation.

GRANT USAGE ON SCHEMA public TO {{.Role}};
GRANT SELECT ON users TO {{.Role}};
GRANT SELECT ON watch_zones TO {{.Role}};
GRANT SELECT ON saved_applications TO {{.Role}};
GRANT SELECT ON notifications TO {{.Role}};
GRANT SELECT ON notification_state TO {{.Role}};
GRANT SELECT ON device_registrations TO {{.Role}};
GRANT SELECT ON offer_codes TO {{.Role}};
GRANT SELECT ON offer_code_redemptions TO {{.Role}};
GRANT SELECT ON goose_db_version TO {{.Role}};
