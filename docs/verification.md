# Local verification: native Windows

Verified on 2026-09-28 with Go 1.27.0 and PostgreSQL 16.15.

## Running services and connections

- PostgreSQL Windows service `agnos-postgresql16`: running, automatic startup.
- PostgreSQL `listen_addresses`: `localhost` only, port 5432.
- Database/user: `agnos`. The app user has no superuser, create-database or create-role privileges; it owns its application database.
- Tables created: `hospitals`, `staff`, `patients`, `schema_migrations`.
- Native API: `http://127.0.0.1:8080`; `/health/ready` returned `{"success":true}`.
- Mock HIS: `http://127.0.0.1:8090`.
- DBeaver `Agnos-Local`: live connection verified from PostgreSQL `pg_stat_activity`, with both `DBeaver 24.0.0 - Main <agnos>` and metadata sessions from loopback.
- Demo data after smoke test: two hospitals, two test staff accounts, two imported fictional patients.

## Executed checks

- Native API and mock HIS builds passed.
- `go vet ./...` passed.
- `go test '-coverpkg=./internal/...' '-coverprofile=coverage.out' ./...` passed with `TEST_DATABASE_URL` set to the real local PostgreSQL instance. Combined internal-package statement coverage: **88.6%**.
- PostgreSQL integration test passed: migrations applied twice, per-hospital uniqueness, SQL filters including Thai names, nullable fields, escaped wildcards, pagination, and scoped upserts.
- `scripts/smoke.ps1` passed against the running native API: staff creation/login, HTTP HIS import, name search, additional filters and hospital isolation.
- Application database login and localhost binding were checked directly using `psql`.

## Starting again

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\start-native.ps1
```

Add `-Verify` to run Go tests and the smoke test again. Use `scripts/stop-native.ps1` to stop only the project API/mock processes before rebuilding changes. PostgreSQL data is kept in `C:\Program Files\PostgreSQL\16\data`; app logs and process state are in `.cache/native/`. Credentials remain in ignored `.env` and `.env.postgres-admin`.

## Docker remains unverified

Docker Desktop 4.91.0 and WSL 2.7.14 are installed, and Compose configuration validation passed. Docker Engine cannot start because Virtual Machine Platform activation was rolled back during Safe Mode (`CBS_E_SAFEMODE_ENTERED`, `0x800f0923`). The user entered Safe Mode after a blue screen/Recovery. Event 41 recorded `BugcheckCode=0`; no crash dump for this incident was found, so the underlying cause is unresolved. Virtualization has not been re-enabled.

The native tests do **not** verify the Docker image, Nginx proxy, Compose startup or CI execution. The Docker stack is retained for a compatible environment and uses a separate database volume. Stop native PostgreSQL or change the published port before running Compose PostgreSQL. The real Hospital A endpoint has not been contacted; local demonstrations use fictional mock data.
