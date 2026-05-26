# Customer Entity Service

## Tech Stack

| Layer     | Technology               |
| --------- | ------------------------ |
| Language  | Go 1.26.3                |
| Framework | Gin                      |
| Database  | PostgreSQL 15+           |
| Driver    | pgx v5 (connection pool) |

## Project Structure

```text
entity-service/
├── cmd/api/main.go              # Entry point — wires all layers and starts the server
├── internal/
│   ├── config/config.go         # Env-based config, builds PostgreSQL DSN
│   ├── db/
│   │   ├── postgres.go          # pgxpool setup and connection
│   │   └── migrate.go           # Schema migration runner
│   ├── domain/entity.go         # Shared domain types (Case, Page, inputs)
│   ├── service/
│   │   ├── interfaces.go        # CaseRepository and CaseService interfaces
│   │   └── entity_service.go    # Business logic — pagination, validation
│   ├── repository/
│   │   ├── entity_repo.go       # SQL queries against the "case" table
│   │   └── tx.go                # Transaction helper
│   ├── handler/
│   │   ├── entity_handler.go    # HTTP handler — bind JSON, call service, respond
│   │   └── health_handler.go    # /healthz and /readyz probes
│   ├── server/
│   │   ├── server.go            # Gin engine setup, middleware registration
│   │   └── routes.go            # URL → handler mapping
│   ├── middleware/
│   │   ├── logger.go            # Request logging
│   │   ├── recovery.go          # Panic recovery → 500
│   │   └── timeout.go           # Per-request context deadline
│   └── apierror/errors.go       # Sentinel errors and JSON error responder
├── migrations/                  # SQL migration files (up/down)
├── queries/                     # Raw SQL queries (sqlc source)
├── deploy/                      # Dockerfile and docker-compose
├── sqlc.yaml                    # sqlc code generation config
├── .env.example                 # Environment variable template
└── Makefile                     # Common dev targets
```

## Prerequisites

- Go 1.21+
- PostgreSQL 15+ (local via Docker or Azure)
- (Optional) [sqlc](https://sqlc.dev/) for query code generation

## Quick Start

### 1. Clone and install dependencies

```bash
git clone https://github.com/wso2-open-operations/cs-tools
cd cs-tools/entity-service
go mod download
```

### 2. Configure environment

```bash
cp .env.example .env
```

Edit `.env` with your database credentials:

```env
DB_HOST=localhost
DB_PORT=5434
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=your_db
DB_SSLMODE=disable       # use "require" for Azure PostgreSQL
```

### 3. Run

```bash
go run cmd/api/main.go
```

Server starts at `http://localhost:8080`.

## Request Flow

```text
HTTP Request
  └── Gin Router
        └── Middleware (logger, recovery, timeout)
              └── Handler          — bind JSON, validate
                    └── Service    — business logic, pagination
                          └── Repository  — SQL query
                                └── PostgreSQL
```

## Environment Variables

| Variable    | Required | Default   | Description       |
| ----------- | -------- | --------- | ----------------- |
| DB_HOST     | Yes      | localhost | PostgreSQL host   |
| DB_PORT     | Yes      | 5432      | PostgreSQL port   |
| DB_USER     | Yes      | postgres  | Database user     |
| DB_PASSWORD | Yes      | —         | Database password |
| DB_NAME     | Yes      | postgres  | Database name     |
| DB_SSLMODE  | No       | require   | SSL mode          |

> `.env` file is loaded automatically if present. Absent `.env` is silently ignored; a malformed one causes a fatal startup error.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
