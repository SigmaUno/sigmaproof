# Paperless-ngx integration

Status: planned reference integration; no adapter is implemented.

Use supported workflows/webhooks, REST API, or post-consume scripts; do not maintain a Paperless fork. [Upstream workflows documentation](https://docs.paperless-ngx.com/usage/).

## Intended flow

1. Trigger after processing reaches the selected stable representation.
2. Receive an authenticated webhook or invoke a local agent.
3. Retrieve source state and exact selected bytes through scoped credentials.
4. Distinguish original upload from archive/OCR output; commit each as its own representation when both are needed.
5. Submit using an idempotency key derived from source instance, object, version, representation, and event identity.
6. Export evidence to durable customer storage and optionally write evidence ID/status/URL to custom fields.

A periodic reconciliation job catches missed webhooks. Metadata write-back must not cause infinite evidence loops. Detect changes between read and hash; retry or bind explicitly to the captured version. Do not hash a mutable download and label it the current version without validation.

Document identifiers and filenames never enter public manifests. A UI with Verify, Download Evidence, and View History is a later interface design; confirm what the pinned Paperless version supports before promising embedded controls.

Acceptance includes duplicates, missed events, out-of-order notifications, source deletion, expired credentials, representation changes, concurrent updates, and failed evidence write-back. Use synthetic documents in fixtures.
