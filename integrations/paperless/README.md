# Paperless-ngx integration

Status: planned reference integration; ingestion planning, a testable ingest runner, a minimal HTTP fetcher, authenticated webhook handling and a reconciliation loop are implemented, but no exporter is complete.

Use supported workflows/webhooks, REST API, or post-consume scripts; do not maintain a Paperless fork. [Upstream workflows documentation](https://docs.paperless-ngx.com/usage/).

## Intended flow

1. Trigger after processing reaches the selected stable representation.
2. Receive an authenticated webhook or invoke a local agent.
3. Retrieve source state and exact selected bytes through scoped credentials.
4. Distinguish original upload from archive/OCR output; commit each as its own representation when both are needed.
5. Submit using an idempotency key derived from source instance, object, version, representation, and event identity.
6. Export evidence to durable customer storage and optionally write evidence ID/status/URL to custom fields.

A periodic reconciliation job catches missed webhooks. Metadata write-back must not cause infinite evidence loops. Detect changes between read and hash; retry or bind explicitly to the captured version. Do not hash a mutable download and label it the current version without validation.

The current `integrations/paperless/agent` package provides deterministic planning, a small ingest runner, a minimal HTTP fetcher, an authenticated webhook adapter and a reconciliation loop for this flow. It maps authenticated source events to explicit `original` or `archive` representations, tenant/source/version identity and a stable idempotency key. The runner fetches selected bytes through a narrow interface, rejects reported version mismatches, bounds document reads, submits through durable ingestion and ignores events that only touch configured SigmaProof evidence fields so status write-back cannot loop into new evidence. The HTTP fetcher validates a content digest against bounded downloaded bytes under the contract below. The webhook adapter verifies a bounded JSON body with an operator-supplied HMAC header; configure Paperless workflow webhooks to send that header as a shared secret signature. Reconciliation replays current ready candidates through the same idempotent path and reports per-item failures. Durable export and write-back are still future work.

## HTTPFetcher capture contract

The supported reference is **Paperless-ngx v2.13.5, REST API v5**. This is a pinned compatibility contract, not a recommendation to deploy an old release. `APIVersion` may be empty or `"5"`; other versions are rejected until their semantics are verified. Requests use `Authorization: Token …` and `Accept: application/json; version=5`. Official pinned source:

- [REST framework settings](https://github.com/paperless-ngx/paperless-ngx/blob/v2.13.5/src/paperless/settings.py) allow API versions 1–5 and negotiate them through the Accept header.
- [DocumentViewSet in views.py](https://github.com/paperless-ngx/paperless-ngx/blob/v2.13.5/src/documents/views.py) implements `download`, `file_response`, `original_requested`, and `metadata`. `GET /api/documents/{id}/download/?original=true` selects the original. Without that query the endpoint selects the archive **only if it exists**, otherwise it returns the original. There is no historical document version selector in this download path.
- [Document model](https://github.com/paperless-ngx/paperless-ngx/blob/v2.13.5/src/documents/models.py) defines `has_archive_version` by the presence of an archive filename. [Consumer](https://github.com/paperless-ngx/paperless-ngx/blob/v2.13.5/src/documents/consumer.py) stores MD5 file checksums, including `archive_checksum`.

For **HTTPFetcher only**, `FetchRequest.Version` (and the supplying `Event.Version`, including webhook/reconciliation candidates) must be exactly the **64 lowercase hexadecimal characters of the expected SHA-256 digest of the selected representation's bytes**. `v1`, timestamps, Paperless MD5 checksums, and arbitrary revision labels are invalid. A trusted producer must calculate this SHA-256 from the selected stable content before creating the event; the metadata endpoint does not supply it. No such producer or historical-byte retrieval is implemented here. A stale event fails if current bytes differ; a retry requires the expected content to remain available, or a new event with its newly observed digest. The generic planner and Runner retain opaque version support for other Fetcher implementations.

The fetcher requires a positive numeric document ID and a base HTTP(S) URL without credentials, query, or fragment (a deployment path prefix is supported). It never sends the content digest as a `version` query parameter. It reads the current download into a private bounded buffer, computes SHA-256, rejects a mismatch with `ErrVersionChanged`, and returns the observed digest and only those validated bytes. It closes HTTP response bodies on success and failure; Runner also closes custom fetcher readers on a reported version mismatch.

For archives, `GET /api/documents/{id}/metadata/` must explicitly report `has_archive_version: true` and a valid `archive_checksum`. Missing/false archive state fails before downloading; malformed metadata fails closed. The downloaded bytes must match the archive MD5 checksum as well as the expected SHA-256. A second metadata read must still report an archive with the same checksum. This detects fallback to different original bytes, archive removal, and observed archive changes during capture. MD5 is only an upstream consistency check, never the evidence identity or a security digest. Identical original/archive content cannot be distinguished by content checks, and these reads do not provide an atomic snapshot or prevent changes after capture; evidence binds to the validated captured bytes, not a claim about later Paperless state.

`HTTPFetcher.MaxBytes` defaults to 64 MiB, independently of `Runner.MaxBytes` (also 64 MiB); set both when changing the document ceiling. Reads consume at most the configured limit plus one byte, and oversized responses fail with `ErrOversizedFetch`. Each metadata response has a separate 1 MiB ceiling. No evidence ingestion occurs after validation fails.

Document identifiers and filenames never enter public manifests. A UI with Verify, Download Evidence, and View History is a later interface design; confirm what the pinned Paperless version supports before promising embedded controls.

Acceptance includes duplicates, missed events, out-of-order notifications, source deletion, expired credentials, representation changes, concurrent updates, and failed evidence write-back. Use synthetic documents in fixtures.
