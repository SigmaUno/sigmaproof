# Core API

Status: production API design plus implemented development endpoints. The development endpoints require explicit `sigmaproofd -store PATH`, refuse non-loopback listen addresses, and are not a substitute for authenticated tenant credentials, provider workers or the reviewed production API. The implemented development surface is described in [dev-api.openapi.yaml](dev-api.openapi.yaml).

| Method | Path | Purpose |
| --- | --- | --- |
| POST | /v1/ingest | Development endpoint: ingest base64 document bytes for a source object/version |
| GET | /v1/evidence/{id} | Development endpoint: inspect redacted evidence state |
| POST | /v1/batches/freeze | Development endpoint: freeze FIFO pending evidence into a manifest |
| GET | /v1/batches/{id} | Development endpoint: inspect frozen batch membership and pending anchor state; future production expands provider state |
| GET | /v1/outbox | Development endpoint: list pending manifest payloads |
| GET | /v1/evidence/{id}/package | Development endpoint: export an explicitly unanchored local package |
| POST | /v1/events | Append a lifecycle assertion |
| GET | /v1/documents/{id}/events | Retrieve ordered events and known head |
| POST | /v1/verify | Optional server-side verification of explicit inputs |
| POST | /v1/export | Future production endpoint: export package revision |
| GET | /v1/policies/{id} | Fetch a versioned policy |

All private resources require authenticated tenant-scoped access before production exposure. The development endpoints currently accept the tenant in the request/query solely for local testing. Ingestion requires an idempotency key, source instance/object/version identity, representation, algorithm identifiers, and protocol version. Do not accept a caller's arbitrary URL for server-side document retrieval. The production workflow should send locally calculated evidence, not document contents; the development endpoint accepts bounded base64 bytes to exercise the local store before an integration agent exists.

Asynchronous acceptance should return a resource ID and pending state; only captured and authenticated evidence can be reported as such. Development evidence inspection reports `pending_batch` before freeze and `pending_submission` after freeze. It omits witness bytes, document digest and nonce. Development freeze, batch and outbox responses use `pending_submission` and `not_submitted` anchor status; they never claim provider submission, receipt capture or authentication. Export of pending/unbatched evidence returns `not_batched`; export of frozen evidence returns the explicitly unanchored profile only.

The development HTTP surface enforces methods, `application/json` POST bodies, JSON body bounds, a 64 MiB document byte limit, tenant/source/key text limits inherited from storage, batch/outbox limits of 1-1,024 and strict unknown-field rejection. Errors use stable JSON `{ "error": "...", "message": "..." }` envelopes with `method_not_allowed`, `unsupported_media_type`, `invalid_json`, `invalid_request`, `conflict`, `not_found`, `not_batched`, `no_pending_evidence` or `internal_error`. Exact ingestion retries are idempotent, including concurrent retries of the same document/source/key; changed document bytes conflict. A reviewed OpenAPI contract, authentication, authorization, rate limits, pagination, cancellation and provider-state resources remain required before supported external exposure.

The independent verifier does not call this API. A remote verification convenience endpoint does not replace local verification.
