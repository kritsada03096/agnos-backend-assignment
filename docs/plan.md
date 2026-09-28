# Development plan and requirement mapping

## Three-day sequence

| Day | Work | Acceptance |
| --- | --- | --- |
| 1 | API/ER/assumptions, scaffold, migrations, create/login/JWT | Hash credentials, enforce scoped uniqueness, reject invalid login |
| 2 | Patient repository, HIS client/import, filters and tests | Isolate hospitals, honor all filters after import, handle upstream errors |
| 3 | Docker/Nginx, demo, smoke test, CI, documentation | Reproducible stack/checks, clear assumptions and submission docs |

## Requirement mapping

| Requirement | Implementation |
| --- | --- |
| Go/Gin | `cmd/api`, `internal/handler` |
| Patient/Staff schemas | SQL migrations and ER diagram |
| Staff create/login | Handler/service/auth and endpoint tests |
| Login required; same-hospital results | JWT/current-staff middleware, scoped SQL and HIS selection |
| Eight optional filters | Query parser/repository, paginated no-filter listing |
| HIS identifier lookup | Hospital client and mock HIS |
| Positive/negative unit tests | Handler, auth, client, config and repository tests |
| Nginx + Go + PostgreSQL Compose | Dockerfile, Compose and Nginx config |
| Project structure / API / ER docs | Architecture, API specification, ER diagram |
| Google Doc and GitHub submission | Content prepared locally; publishing/sharing remains a separate step |

## Design choices beyond explicit requirements

JWT, bcrypt, pgx, Hospital entity, composite uniqueness, DB-first storage, AND filters, substring name matching, pagination, provisioning authorization, demo HIS, migration runner and CI are implementation decisions.

The actual Hospital A availability/authentication/error payloads and Hospital B contract are unspecified. Name-only discovery of unimported patients requires an additional upstream API or sync. Cached records have no expiry/refresh policy in this scope.

## Verification commands

`go test ./...`, `go vet ./...`, `go build ./...`. Set `TEST_DATABASE_URL` for real SQL integration tests. Start the demo Compose profile and run `scripts/smoke.ps1` for end-to-end checks. CI defines these checks; a workflow file does not itself prove an executed CI run.
