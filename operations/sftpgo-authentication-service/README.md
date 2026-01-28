# SFTPGo Authentication Service

This project provides a secure, individual-account-based authentication and provisioning solution for [SFTPGo](https://github.com/drakkan/sftpgo), integrated with [Asgardeo](https://asgardeo.io) (or [WSO2 IS](https://github.com/wso2/wso2is)). 

**Key Goals & Capabilities:**
- **Centralized Identity**: Authenticates users against corporate credentials in Asgardeo, enforcing Multi-Factor Authentication (Password + OTP/TOTP).
- **Dynamic User Provisioning**: Uses SFTPGo's `Pre-login Hook` to check permissions and map Virtual Folders in real-time based on subscriptions or roles.
- **Granular Scope**: Supports distinct access patterns for internal staff (scoped to specific directories) vs. customers (access to subscription-based virtual folders).
- **Automated Management**: Automatically provisions missing physical directories via SFTPGo Admin APIs.


## Features

- **Dual Organization Support**: Separate Asgardeo organizations for internal and external users
- **Pre-Login User Provisioning**: Automatic user and folder configuration based on IdP roles
- **Keyboard-Interactive Authentication**: Multi-step authentication with MFA support (TOTP, OTP)
- **Dynamic Folder Management**: Automatic folder provisioning via SFTPGo Admin API
- **Session-Based Auth Flow**: Database-backed session management for authentication steps
- **Security Hardening**: SCIM injection protection, path traversal prevention, input validation

## Architecture

```
┌─────────────┐         ┌──────────────────┐         ┌─────────────────┐
│   SFTPGo    │────────▶│  Auth Service    │────────▶│   Asgardeo      │
│   Server    │◀────────│  (This Service)  │◀────────│   (Internal)    │
└─────────────┘         └──────────────────┘         └─────────────────┘
                               │      ▲
                               │      │               ┌─────────────────┐
                               │      └──────────────▶│   Asgardeo      │
                               │                      │   (External)    │
                               ▼                      └─────────────────┘
                        ┌──────────────┐
                        │   MySQL DB   │
                        │  (Sessions)  │
                        └──────────────┘
```

## Project Structure

```
.
├── cmd/server/main.go              # Application entry point
├── internal/
│   ├── config/                     # Configuration management
│   │   ├── config.go              # Environment variable loading
│   │   └── config_test.go         # Configuration tests
│   ├── handler/                    # HTTP request handlers
│   │   ├── handler.go             # Pre-login and auth handlers
│   │   └── utils.go               # Handler utilities
│   ├── service/                    # Business logic layer
│   │   ├── database.go            # Session management
│   │   ├── idp.go                 # Asgardeo integration
│   │   ├── sftpgo.go              # SFTPGo Admin API client
│   │   └── subscription.go        # External folder API
│   ├── models/                     # Data structures
│   ├── log/                        # Custom logger
│   └── util/                       # Shared utilities
├── db/migrations/                  # Database schema
├── openapi.yaml                    # API specification
└── Dockerfile                      # Container image
```

## Setup

### Prerequisites

- Go 1.23+
- MySQL database
- Two Asgardeo organizations (for internal and external users)
- SFTPGo server with admin API access

> **Note**: This project requires Asgardeo or WSO2 Identity Server (WSO2 IS) as the Identity Provider (IdP). It utilizes proprietary app-native authentication APIs specific to these platforms/products.


### Environment Variables

Copy `.env.example` to `.env` and configure:

#### Service Configuration
```bash
PORT="9090"                    # Server port
LOG_LEVEL="INFO"              # DEBUG, INFO, WARN, ERROR
HTTP_TIMEOUT="15"             # HTTP client timeout (seconds)
HOOK_API_KEY="your-key"       # API key for hooks (adds API-Key header requirement)
```

#### Internal Organization
```bash
INTERNAL_CLIENT_ID="your_internal_client_id"
INTERNAL_CLIENT_SECRET="your_internal_client_secret"
INTERNAL_IDP_BASE_PATH="https://api.asgardeo.io/t/internal-org"
SCIM_SCOPE="internal_user_mgt_view"
OAUTH_CALLBACK_URL="https://your-app/callback"
```

#### External Organization
```bash
EXTERNAL_CLIENT_ID="your_external_client_id"
EXTERNAL_CLIENT_SECRET="your_external_client_secret"
EXTERNAL_IDP_BASE_PATH="https://api.asgardeo.io/t/external-org"
```

#### SFTPGo Configuration
```bash
SFTPGO_API_BASE="http://localhost:8080/api/v2"
ADMIN_USER="admin"
ADMIN_KEY="your-sftpgo-admin-api-key"
FOLDER_PATH="/path/on/sftpgo/server"
DIR_PATH="/path/on/sftpgo/server"
CHECK_ROLE="internal"          # Role display name for internal users
```

#### Database Configuration
```bash
DB_CONN_STRING="user:password@tcp(127.0.0.1:3306)/sftpgo_sessions"
DB_MAX_OPEN_CONNS="25"         # Optional, default: 25
DB_MAX_IDLE_CONNS="25"         # Optional, default: 25
DB_CONN_MAX_LIFETIME="5m"      # Optional, default: 5m
```

#### Subscription APIs
```bash
SUBSCRIPTION_API="https://api.example.com/subscriptions?customerEmail=%s"
PROJECT_API="https://api.example.com/projects?projectKey=%s"
```

### Database Setup

Apply the migration to create the sessions table:

```bash
mysql -u youruser -p yourdatabase < db/migrations/001_create_sessions_table.up.sql
```

### Install Dependencies

```bash
go mod tidy
```

## Running the Service

### Development
```bash
go run ./cmd/server/main.go
```

### Production Build
```bash
go build -o sftpgo-authn-service ./cmd/server/main.go
./sftpgo-authn-service
```

### Docker
```bash
docker build -t sftpgo-authn-service .
docker run -p 9090:9090 --env-file .env sftpgo-authn-service
```

## API Endpoints

### POST /prelogin-hook
Pre-login user provisioning hook called by SFTPGo.
Requires `API-Key` header if `HOOK_API_KEY` is configured.

**Request:**
```json
{
  "id": 0,
  "username": "user@example.com"
}
```

**Response (200):**
```json
{
  "username": "user@example.com",
  "home_dir": "/data/user_example_com",
  "permissions": {
    "/": ["list"],
    "/project1": ["upload", "list", "download", "create_dirs", "delete"]
  },
  "status": 1,
  "virtual_folders": [
    {"name": "project1", "virtual_path": "/project1"}
  ]
}
```

### POST /auth-hook
Keyboard-interactive authentication hook.
Requires `API-Key` header if `HOOK_API_KEY` is configured.

**Request:**
```json
{
  "request_id": "req123",
  "step": 1,
  "username": "user@example.com",
  "answers": []
}
```

**Response:**
```json
{
  "instruction": "Enter your password:",
  "questions": ["Password:"],
  "echos": [false],
  "auth_result": 0
}
```

## How It Works

### User Type Detection
The service automatically detects user type based on email domain:
- **@wso2.com** → Internal organization
- **Others** → External organization

### User Provisioning
1. SFTPGo calls `/prelogin-hook` before login
2. Service fetches user from appropriate Asgardeo org
3. Determines permissions based on role
4. Provisions required folders via SFTPGo Admin API
5. Returns user configuration to SFTPGo

### Authentication Flow
1. SFTPGo calls `/auth-hook` with step 1
2. Service initiates flow with appropriate Asgardeo org
3. User provides credentials through keyboard-interactive prompts
4. Service manages session state in database
5. MFA steps handled if required
6. Final auth result returned to SFTPGo

## Security Considerations

- **API Key Auth**: Optional `API-Key` header validation for hooks
- **Input Validation**: Usernames validated for length and invalid characters
- **SCIM Injection Protection**: Quotes escaped in SCIM filter queries
- **Path Traversal Prevention**: Folder names validated for illegal characters
- **Prepared Statements**: All database queries use prepared statements
- **Secrets Management**: Sensitive config loaded from environment variables
- **Non-Root Container**: Docker image runs as unprivileged user (UID 10014)
- **HTTPS Support**: CA certificates included in container

## Testing

Run all tests:
```bash
go test ./...
```

Run with coverage:
```bash
go test -cover ./...
```

Static analysis:
```bash
go vet ./...
```

## Troubleshooting

### Common Issues

**Database connection fails:**
- Verify `DB_CONN_STRING` format: `user:password@tcp(host:port)/dbname`
- Check MySQL is running and accessible
- Ensure sessions table exists

**Authentication fails:**
- Check appropriate org credentials (INTERNAL vs EXTERNAL)
- Verify IdP base paths are correct
- Review logs for specific error messages

**User not provisioned:**
- Ensure user exists in appropriate Asgardeo org
- For external users, ensure the user is having a valid subscription
- Check SCIM scope permissions of appropriate client credentials
- Verify SFTPGo Admin API credentials

## Development

### Adding New Features
1. Update models in `internal/models/`
2. Implement business logic in `internal/service/`
3. Add handlers in `internal/handler/`
4. Update `openapi.yaml`
5. Add tests

### Code Style
- Follow Go conventions
- Use meaningful variable names
- Add comments for exported functions
- Keep functions focused and small