# Paperless-ngx integration

Status: planned reference integration; ingestion planning and a testable ingest runner are implemented, but no webhook server, Paperless API client or exporter is complete.

Use supported workflows/webhooks, REST API, or post-consume scripts; do not maintain a Paperless fork. [Upstream workflows documentation](https://docs.paperless-ngx.com/usage/).

## Intended flow

1. Trigger after processing reaches the selected stable representation.
2. Receive an authenticated webhook or invoke a local agent.
3. Retrieve source state and exact selected bytes through scoped credentials.
4. Distinguish original upload from archive/OCR output; commit each as its own representation when both are needed.
5. Submit using an idempotency key derived from source instance, object, version, representation, and event identity.
6. Export evidence to durable customer storage and optionally write evidence ID/status/URL to custom fields.

A periodic reconciliation job catches missed webhooks. Metadata write-back must not cause infinite evidence loops. Detect changes between read and hash; retry or bind explicitly to the captured version. Do not hash a mutable download and label it the current version without validation.

The current `integrations/paperless/agent` package provides deterministic planning, a small ingest runner and a minimal HTTP fetcher for this flow. It maps authenticated source events to explicit `original` or `archive` representations, tenant/source/version identity and a stable idempotency key. The runner fetches selected bytes through a narrow interface, refuses mutable-version fetches, bounds document reads, submits through durable ingestion and ignores events that only touch configured SigmaProof evidence fields so status write-back cannot loop into new evidence. The HTTP fetcher uses Paperless token auth and `/api/documents/{id}/download/` with `version` and `original=true` query parameters as needed. Webhook authentication, periodic reconciliation, durable export and write-back are still future work.

Document identifiers and filenames never enter public manifests. A UI with Verify, Download Evidence, and View History is a later interface design; confirm what the pinned Paperless version supports before promising embedded controls.

Acceptance includes duplicates, missed events, out-of-order notifications, source deletion, expired credentials, representation changes, concurrent updates, and failed evidence write-back. Use synthetic documents in fixtures.
