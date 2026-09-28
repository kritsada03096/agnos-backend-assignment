# Agnos Hospital Middleware

Go/Gin API with PostgreSQL (pgx), bcrypt, JWT, configurable HIS integration, Docker Compose and Nginx. Staff can only search/import patients within their hospital.

## Quick start: local demo

Requires Docker Engine/Desktop and Docker Compose v2. All fixture patient data is fictional.

```powershell
Copy-Item .env.example .env
docker compose --profile demo up --build -d --wait
Invoke-RestMethod http://localhost:8080/health/ready
```

On Linux/macOS, use `cp .env.example .env` and `curl http://localhost:8080/health/ready`.

On Windows with Docker Desktop installed and `.env` already configured, `powershell -ExecutionPolicy Bypass -File .\scripts\start-local.ps1 -Verify` starts the stack, runs smoke and Go integration tests, and requests the `Agnos-Local` DBeaver connection when DBeaver is installed. The script reads the actual provisioning key from `.env`. Restart Windows first if WSL/Virtual Machine Platform was just enabled. Preserve an existing `.env`; its generated local credentials may differ from the examples below.

Compose starts PostgreSQL, applies migrations, registers two hospitals, starts the API and Nginx, and enables mock HIS. No staff accounts are seeded. Create one:

```powershell
$body = @{ username = 'staff01'; password = 'password123'; hospital = 'hospital-a' } | ConvertTo-Json
$provisioning = @{ 'X-Provisioning-Key' = 'local-demo-provisioning-key-change-before-deploy-123456' }
Invoke-RestMethod -Method Post -Uri http://localhost:8080/staff/create -ContentType application/json -Headers $provisioning -Body $body
$login = Invoke-RestMethod -Method Post -Uri http://localhost:8080/staff/login -ContentType application/json -Body $body
$headers = @{ Authorization = "Bearer $($login.data.access_token)" }
Invoke-RestMethod -Uri 'http://localhost:8080/patient/search?national_id=1111111111111' -Headers $headers
Invoke-RestMethod -Uri 'http://localhost:8080/patient/search?first_name=Demo' -Headers $headers
```

The first identity lookup imports the patient. Name-only searches find locally stored patients. Mock HIS accepts national ID `1111111111111` or passport `DEMO-PASSPORT` in both hospitals but returns different HNs (`HN-a-001`, `HN-b-001`). Other IDs return 404.

Run `./scripts/smoke.ps1` against the running demo to create accounts for both hospitals and verify login, HIS import, name search, combined filters and isolation. Each run creates new test usernames. If the provisioning key was changed, pass `-ProvisioningKey 'your-key'`.

## Architecture and structure

```text
Client -> Nginx -> Gin -> Auth middleware -> Handler -> Service
                                                        |-- Repository -> PostgreSQL
                                                        `-- Hospital client -> HIS

cmd/api/                API, migration and seed commands
cmd/mockhis/            Local demo HIS fixture server
internal/auth/          JWT signing and verification
internal/client/        HIS HTTP client
internal/config/        Validated environment configuration
internal/database/      Connection pool, migration runner and seed
internal/handler/       Input validation and HTTP responses
internal/middleware/    Staff authentication and provisioning protection
internal/model/         Domain records, filters and errors
internal/repository/    Hospital-scoped parameterized PostgreSQL queries
internal/service/       Business logic
migrations/             Embedded versioned SQL
deploy/                 Nginx configuration
docs/                   Architecture, API, ER diagram and planning
scripts/                End-to-end smoke test
```

## API

| Method | Route | Authorization |
| --- | --- | --- |
| POST | `/staff/create` | `X-Provisioning-Key` |
| POST | `/staff/login` | Username, password and hospital code |
| GET | `/patient/search` | Bearer JWT |
| GET | `/health/live` | None |
| GET | `/health/ready` | None; checks DB connectivity |

See [API specification](docs/api-spec.md) for complete inputs, responses and errors.

## Environment

Compose loads `.env`; the Go binary does **not** load dotenv files automatically. Export/set variables when running directly. Never commit `.env`.

| Variable | Meaning |
| --- | --- |
| `DB_USER`, `DB_PASSWORD`, `DB_NAME` | Compose DB credentials; example user/database `agnos` |
| `DB_PORT`, `HTTP_PORT` | Published local ports, defaults 5432 / 8080 |
| `DATABASE_URL` | Go database connection string; Compose supplies its own with host `postgres` |
| `JWT_SECRET` | Required, minimum 32 bytes |
| `STAFF_PROVISIONING_KEY` | Required administrative key, minimum 32 bytes |
| `JWT_TTL` | Positive token lifetime, default `1h`, maximum `24h` |
| `HIS_TIMEOUT` | Positive upstream timeout, default `5s`, maximum `15s` |
| `HTTP_ADDR` | Go listen address, default `:8080` |
| `HOSPITAL_A_API_URL` | Seeded base URL; example selects mock HIS |
| `HOSPITAL_B_API_URL` | Demo base URL; real Hospital B contract was not provided |

Replace sample secrets with independent random values for deployment. Use URL-safe DB passwords with the sample Compose URL interpolation, or explicitly supply a URL-encoded connection string in Compose.

To configure the supplied Hospital A endpoint, set `HOSPITAL_A_API_URL=https://hospital-a.api.co.th`, configure Hospital B as appropriate, and start without `--profile demo`. The supplied endpoint has not been verified as a live service; its authentication was not specified. After URL changes run `docker compose run --rm seed` to update registered endpoints.

## Run Go locally

Requires Go 1.26+ (Docker/CI use 1.27). Versions are locked by `go.mod`/`go.sum`.

```powershell
docker compose up -d postgres
$env:DATABASE_URL = 'postgres://agnos:agnos_local_only@localhost:5432/agnos?sslmode=disable'
$env:JWT_SECRET = 'local-demo-jwt-secret-change-before-deploy-123456'
$env:STAFF_PROVISIONING_KEY = 'local-demo-provisioning-key-change-before-deploy-123456'
$env:HOSPITAL_A_API_URL = 'http://localhost:8090/a'
$env:HOSPITAL_B_API_URL = 'http://localhost:8090/b'
go run ./cmd/api migrate
go run ./cmd/api seed
go run ./cmd/api
```

Run `go run ./cmd/mockhis` in another terminal. Inside Compose the mock hostname is `mock-his`, not `localhost`. Do not run the Compose backend/Nginx on port 8080 simultaneously with a local API on the same port.

## Tests

```powershell
go test ./...
go vet ./...
go build ./...
go test '-coverpkg=./internal/...' '-coverprofile=coverage.out' ./...
go tool cover '-html=coverage.out'
```

Unit tests cover every assignment endpoint, password hashing, bad credentials, JWT claims/expiry/signatures, deleted/transferred staff, hospital isolation, filters, HIS import and upstream errors. HTTP client tests use local `httptest` servers.

Real PostgreSQL tests are skipped unless a development/test database is configured:

```powershell
$env:TEST_DATABASE_URL = 'postgres://agnos:agnos_local_only@localhost:5432/agnos?sslmode=disable'
go test -v ./internal/repository
```

These tests create and remove a unique schema, verify repeatable migrations, constraints, actual SQL filters including Thai names, nullable fields and cross-hospital upsert isolation. The DB user must have schema creation privileges.

CI runs formatting, vet, build, tests with PostgreSQL and the race detector, then Docker Compose and the smoke test. Locally, `go test -race ./...` requires a compatible C compiler.

## Migrations and DBeaver

Use `go run ./cmd/api migrate` or `docker compose run --rm migrate`. Embedded `*.up.sql` files run in filename order under a transaction/advisory lock and are recorded in `schema_migrations`. Add new numbered migrations; do not edit applied ones. Only forward migrations are provided. Seed is idempotent and updates hospital URLs without deleting patient/staff data. Compose data persists in `postgres_data`.

| DBeaver field | Demo value |
| --- | --- |
| Driver | PostgreSQL |
| Host / port | `localhost` / `5432` |
| Database / username | `agnos` / `agnos` |
| Password | `agnos_local_only`, or your `.env` value |

PostgreSQL and Nginx published ports bind to `127.0.0.1`. The backend uses `postgres:5432` inside Compose.

## Assumptions and limits

- All eight search filters are optional. No filters lists the authenticated hospital's local records with pagination.
- Filters combine with AND. Names match case-insensitive literal substrings in either Thai or English columns. Other filters use exact matching.
- DB-first fallback requires national ID or passport ID. Name-only searches cannot discover unimported HIS patients with the supplied API.
- Imported snapshots do not expire automatically. `last_synced_at` records import time; a refresh/sync policy remains future work.
- HIS endpoint selection is server-side and hospital-scoped. Returned identifiers are verified; upstream hospital IDs are ignored.
- Usernames and HNs are unique per hospital. Patient identifiers are strings and may repeat across hospitals.
- Every token is checked against current staff records. Refresh/logout/revocation-list and staff-management APIs are outside scope.
- Provisioning protection is an explicit design addition: the administrative key can create staff in any configured hospital and must not be distributed as a staff credential.
- Nginx rate-limits create/login. Local HTTP needs TLS termination for public deployment. Logs omit credentials, bodies and patient queries. Nginx-generated errors may use its own response format.

## Documentation and submission

- [Architecture](docs/architecture.md)
- [API specification](docs/api-spec.md)
- [ER diagram](docs/er-diagram.md)
- [Plan and requirement mapping](docs/plan.md)
- [Local verification and remaining environment checks](docs/verification.md)

These files prepare the planning content. Copy/export it into a Google Doc for the requested submission format. GitHub publication, external sharing and deployment are separate steps and have not been performed.

Implementation references: [Gin](https://gin-gonic.com/en/docs/), [pgxpool](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool), [JWT v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5), [bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt).
