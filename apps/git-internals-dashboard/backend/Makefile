BINARY := server

.PHONY: all run vet test build clean migrate migrate-down seed

all: build

run:
	go run ./cmd/server

vet:
	go vet ./...

# -p 1 serializes package test execution: DB-backed tests across packages
# insert raw-SQL fixtures directly into the shared compose Postgres (no
# per-run namespacing beyond a hardcoded literal per package), so two
# packages' test binaries running concurrently against the same instance can
# race each other even though no single package's own fixtures collide.
test: vet
	go test -race -p 1 ./...

build: test
	go build -o $(BINARY) ./cmd/server

clean:
	rm -f $(BINARY)

# Applies migrations/*.up.sql against DATABASE_URL via golang-migrate.
# Wired up in Phase 3.
migrate:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down 1

# Runs cmd/seed — synthetic fixtures unless GITHUB_TOKEN is set. Wired up in
# Phase 9.
seed:
	go run ./cmd/seed
