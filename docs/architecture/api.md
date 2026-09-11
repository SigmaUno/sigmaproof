# Core API proposal

Status: design only; no endpoints are implemented.

| Method | Path | Purpose |
| --- | --- | --- |
| POST | /v1/evidence | Ingest commitment for a source object/version |
| GET | /v1/evidence/{id} | Inspect evidence and capture status |
| POST | /v1/events | Append a lifecycle assertion |
| GET | /v1/documents/{id}/events | Retrieve ordered events and known head |
| GET | /v1/batches/{id} | Inspect batch and provider state |
| POST | /v1/verify | Optional server-side verification of explicit inputs |
| POST | /v1/export | Export package revision |
| GET | /v1/policies/{id} | Fetch a versioned policy |

All private resources require authenticated tenant-scoped access. Ingestion requires an idempotency key, source instance/object/version identity, representation, algorithm identifiers, and protocol version. Do not accept a caller's arbitrary URL for server-side document retrieval. The default workflow sends locally calculated evidence, not document contents.

Asynchronous acceptance should return a resource ID and pending state; only captured and authenticated evidence can be reported as such. Specify conflicts, request limits, validation errors, retries, cancellation, pagination, and error codes in OpenAPI before implementation. Export must identify missing/pending requirements instead of fabricating a complete proof.

The independent verifier does not call this API. A remote verification convenience endpoint does not replace local verification.
