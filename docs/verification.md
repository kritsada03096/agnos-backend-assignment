# Local verification

Run the commands in the README to reproduce checks. Unit/API/client tests run without an external database and cover the public API behavior, including hospital isolation and HIS failures.

Local checks completed on 2026-09-28 with Go 1.27.0 on Windows:

- `go vet ./...` and `go build ./...`: passed.
- `go test '-coverpkg=./internal/...' '-coverprofile=coverage.out' ./...`: passed; 74.6% combined statement coverage across internal packages, including currently unexecuted database code in the denominator.
- `go mod verify`: all modules verified.
- Compose/CI YAML parsing, smoke-script PowerShell parsing and `git diff --check`: passed. Syntax parsing is not a Docker runtime check.

Docker Desktop 4.91.0 (Docker CLI 29.8.0, Compose 5.5.1) and WSL 2.7.14 were installed on this Windows machine on 2026-09-28. Windows feature activation reports a required restart, so Docker Engine and PostgreSQL containers have not yet started. `docker compose --profile demo config --quiet` passed. DBeaver's `Agnos-Local` connection is saved with the local credentials, but its live connection has not been verified. The existing PostgreSQL JDBC driver is available.

Real PostgreSQL integration tests and the Docker/Nginx smoke test remain pending until after restart. Do not interpret passing unit tests as verification of the complete deployed stack. Run `powershell -ExecutionPolicy Bypass -File .\scripts\start-local.ps1 -Verify` after restarting Windows to start Docker, initialize the stack, run smoke/unit/integration tests and request the saved DBeaver connection. The startup script has been syntax-checked but cannot yet be exercised against a running engine.

The integration suite and CI workflow are prepared to run these remaining checks on an environment with PostgreSQL and Docker. CI has not been pushed or executed. The supplied Hospital A endpoint has not been contacted with patient data; all HTTP client tests use local fixtures.
