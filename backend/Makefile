.PHONY: run build docker-build lint test test-cover generate migrate-up migrate-down migrate-create clean

# Variables
BINARY_NAME=bohikor2
GO_FILES=$(shell find . -name '*.go' -not -path './db/sqlc/*')
MIGRATIONS_DIR=migrations


# Run the dev server
run:
	go run cmd/server/main.go

# Build the binary
build:
	go build -o bin/$(BINARY_NAME) cmd/server/main.go

# Build Docker image
docker-build:
	docker build -t $(BINARY_NAME):latest .

# Run linter
lint:
	golangci-lint run

# Run tests
test:
	go test -v -race -coverprofile=coverage.out ./...

# Business-logic packages we hold to a coverage bar. Excludes:
#   db/sqlc         -> auto-generated query wrappers (don't test generated code)
#   cmd/*           -> thin main() entrypoints
#   config/         -> env loading
#   repository/     -> pgx pool wrapper (needs a live DB, no logic)
#   database/       -> migration runner (covered by opt-in MIGRATION_TEST_DB_URL)
COVER_PKGS=github.com/Iknite-Space/bohikor2/internal/handler,github.com/Iknite-Space/bohikor2/internal/server,github.com/Iknite-Space/bohikor2/internal/middleware,github.com/Iknite-Space/bohikor2/internal/campay,github.com/Iknite-Space/bohikor2/internal/service,github.com/Iknite-Space/bohikor2/internal/authjwt,github.com/Iknite-Space/bohikor2/internal/email,github.com/Iknite-Space/bohikor2/internal/authpassword
COVER_TEST_PKGS=./internal/handler/... ./internal/server/... ./internal/middleware/... ./internal/campay/... ./internal/service/... ./internal/authjwt/... ./internal/email/... ./internal/authpassword/...

# Coverage over business-logic packages only (reproducible bar for `make test-cover`).
# The DB-backed code (server.New wiring, Real*Store query wrappers) is exercised
# by testcontainer integration tests in internal/server, which need Docker. Those
# tests SKIP automatically when Docker is unavailable, in which case the reported
# number is lower (that code shows as uncovered) but the suite still passes.
test-cover:
	go test -race -coverpkg=$(COVER_PKGS) -coverprofile=coverage.out $(COVER_TEST_PKGS)
	@go tool cover -func=coverage.out | tail -1

# Generate sqlc code
generate:
	cd db && sqlc generate

# Run migrations up
migrate-up:
	@DATABASE_URL=$$(grep '^DATABASE_URL=' .env | head -1 | cut -d'=' -f2-); \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "ERROR: DATABASE_URL is required"; \
		exit 1; \
	fi; \
	migrate -path $(MIGRATIONS_DIR) -database "$$DATABASE_URL" up

# Run migrations down (rollback last migration)
migrate-down:
	@DATABASE_URL=$$(grep '^DATABASE_URL=' .env | head -1 | cut -d'=' -f2-); \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "ERROR: DATABASE_URL is required"; \
		exit 1; \
	fi; \
	migrate -path $(MIGRATIONS_DIR) -database "$$DATABASE_URL" down 1

# Run migrations with force (to a specific version)
migrate-force:
	@DATABASE_URL=$$(grep '^DATABASE_URL=' .env | head -1 | cut -d'=' -f2-); \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "ERROR: DATABASE_URL is required"; \
		exit 1; \
	fi; \
	if [ -z "$(VERSION)" ]; then \
		echo "ERROR: VERSION is required (e.g. make migrate-force VERSION=1)"; \
		exit 1; \
	fi; \
	migrate -path $(MIGRATIONS_DIR) -database "$$DATABASE_URL" force $(VERSION)

# Create a new migration file
migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "ERROR: NAME is required (e.g. make migrate-create NAME=add_users_table)"; \
		exit 1; \
	fi
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(NAME)

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out

# Full clean including generated sqlc code
distclean: clean
	rm -rf db/sqlc/

# Install dependencies
deps:
	go mod download

# Install tools (golangci-lint, migrate, sqlc)
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
