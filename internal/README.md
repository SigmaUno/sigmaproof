# Internal packages

`cli` contains command dispatch, `server` contains development health/readiness handlers, and `buildinfo` contains build metadata.

Future packages: `evidence`, `events`, `merkle`, `policy`, `anchors/celestia`, `anchors/opentimestamps`, `trust`, and `storage`. Create packages when implementation begins; avoid empty exported abstractions. The independent verifier must remain usable without service internals.

The [storage package](storage/README.md) provides tenant-scoped transactional ingestion, frozen batches, pending outbox records and local package reconstruction. Authentication/API wiring and provider workers are not implemented yet.
