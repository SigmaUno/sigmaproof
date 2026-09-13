# Development daemon

Run `go run ./cmd/sigmaproofd` or `./bin/sigmaproofd` after `make build`. The default listen address is `127.0.0.1:8080`; override with `-listen`.

Without `-store`, the daemon exposes only `GET /healthz` and reports `GET /readyz` as 503. Passing `-store PATH` opens a private local bbolt database and enables the experimental engine endpoints. Store-backed mode refuses non-loopback listen addresses because it has no authentication:

- `POST /v1/ingest` hashes a base64 document, creates a private nonce once, and stores tenant-scoped evidence idempotently.
- `GET /v1/evidence/{id}?tenant=...` reports development state without witness bytes, document digest or nonce.
- `POST /v1/batches/freeze` freezes FIFO pending evidence into an exact manifest and leaves it pending submission.
- `GET /v1/batches/{id}?tenant=...` reports frozen batch membership, manifest bytes and local submission status.
- `GET /v1/outbox?tenant=...&limit=...` lists retryable pending manifest bytes as hex; `unknown` outcomes can stay pending for reconciliation. It does not claim authenticated publication.
- `GET /v1/evidence/{id}/package?tenant=...` exports the explicitly unanchored local package for frozen evidence.

HTTP timeouts and graceful SIGINT/SIGTERM shutdown are configured. Provider submission, receipt capture, authentication and production deployment remain future work. See the [architecture](../../docs/architecture/overview.md) and [development OpenAPI draft](../../docs/architecture/dev-api.openapi.yaml).

Minimal local flow:

```sh
./bin/sigmaproofd -store ./sigmaproof.db &
pid=$!
trap 'kill "$pid"' EXIT
until curl -fsS http://127.0.0.1:8080/readyz >/dev/null; do sleep 0.2; done
printf 'invoice bytes' | base64
curl -sS -X POST http://127.0.0.1:8080/v1/ingest \
  -H 'Content-Type: application/json' \
  -d '{"tenant":"tenant-a","idempotency_key":"paperless:1:v1","source":{"instance":"paperless","object":"1","version":"v1"},"representation":"original","document_base64":"aW52b2ljZSBieXRlcw=="}'
curl -sS -X POST http://127.0.0.1:8080/v1/batches/freeze \
  -H 'Content-Type: application/json' \
  -d '{"tenant":"tenant-a","request_key":"batch-1","limit":10}'
curl -sS 'http://127.0.0.1:8080/v1/outbox?tenant=tenant-a&limit=10'
```
