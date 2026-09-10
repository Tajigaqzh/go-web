# Repository Guidelines

## Project Structure & Module Organization

This is a Go 1.26 REST API built with Gin. Keep routing in `router/`, request handling in `controller/`, reusable rules in `service/`, and persistence models in `model/`. Authentication and authorization live in `middleware/` and `authz/`; shared responses belong in `resp/`. Configuration is under `config/`, while generated Swagger files are in `docs/`. Tests live in `main_test.go`, with SQLite and miniredis helpers in `testutil/`.

All business endpoints use the `/api/v1` prefix. Introduce `/api/v2` for incompatible API changes instead of changing v1 behavior in place.

## Build, Test, and Development Commands

- `Copy-Item config.yaml.example config.yaml`: create a local Windows configuration.
- `docker compose up -d`: start MySQL 8 and Redis 7.
- `go mod download`: download module dependencies.
- `go run .`: run the API on the configured port (default `8080`).
- `go build ./...`: compile all packages.
- `go test ./...`: run the complete test suite.
- `go test ./... -run TestName`: run one matching test.
- `go run github.com/swaggo/swag/cmd/swag init --parseDependency --parseInternal --output docs`: regenerate Swagger artifacts after annotation changes.

## Coding Style & Naming Conventions

Use UTF-8 and preserve Chinese documentation comments. Format Go changes with `gofmt`; use its tab indentation. Exported identifiers use `PascalCase`, internal identifiers use `camelCase`, and package names remain short and lowercase. Keep controllers thin and return responses through `resp`. Do not manually edit generated files in `docs/`.

## Testing Guidelines

Use Go's `testing` package with `testify`. Name tests `TestFeatureScenario`, for example `TestRefreshRateLimitByIP`. Tests must remain isolated and should use `testutil.SetupTestDB` and `SetupTestAuth`, not local MySQL or Redis. Add coverage for route status codes, permission failures, and rate-limit boundaries. Run `go test ./...` before submitting.

## Commit & Pull Request Guidelines

The repository history is too small to establish a firm convention. Use concise, imperative commit subjects such as `add login rate limiting`. Keep unrelated changes separate. Pull requests should explain behavior changes, list affected routes or configuration, mention Swagger regeneration, and include test results. Link the relevant issue when one exists; screenshots are only needed for rendered Swagger or other visible UI changes.

## Security & Configuration Tips

Never commit `config.yaml`, JWT secrets, passwords, or logs. Public authentication routes must retain Redis-backed rate limits. Production deployments must replace or disable the seeded `admin/admin` and `auditor/auditor` accounts and add proxy, CDN, or WAF-level traffic protection.
