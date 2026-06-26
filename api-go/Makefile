# Convenience targets for the local Postgres + PostGIS integration test loop.
# The default `go test ./...` (no tag) needs none of this and never touches Docker.

COMPOSE      ?= docker compose -f docker-compose.yml
TEST_DB_DSN  ?= postgres://towncrier:towncrier@localhost:5433/towncrier_test?sslmode=disable

.PHONY: db-up db-down test-integration

# Boot Postgres and block until the healthcheck passes.
db-up:
	$(COMPOSE) up -d --wait

# Stop Postgres and drop its data volume.
db-down:
	$(COMPOSE) down -v

# Bring the database up, then run the integration-tagged suite against it.
test-integration: db-up
	TEST_DATABASE_URL="$(TEST_DB_DSN)" go test -tags=integration ./...
