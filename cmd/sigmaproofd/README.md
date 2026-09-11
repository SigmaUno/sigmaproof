# Daemon skeleton

Run `go run ./cmd/sigmaproofd` or `./bin/sigmaproofd` after `make build`. The default listen address is `127.0.0.1:8080`; override with `-listen`. `GET /healthz` returns 200; `GET /readyz` returns 503 because ingestion, storage and providers are not implemented. Unknown endpoints return 404. HTTP timeouts and graceful SIGINT/SIGTERM shutdown are configured. See the [architecture](../../docs/architecture/overview.md).
