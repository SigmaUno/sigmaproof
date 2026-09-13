# Internal packages

`cli` contains command dispatch, `server` contains development health/readiness and opt-in local evidence HTTP handlers, and `buildinfo` contains build metadata.

Future packages: `evidence`, `events`, `policy`, `anchors/celestia`, `anchors/opentimestamps`, and `trust`. Create packages when implementation begins; avoid empty exported abstractions. The independent verifier must remain usable without service internals.

The [storage package](storage/README.md) provides tenant-scoped transactional ingestion, frozen batches, pending outbox records and local package reconstruction. The HTTP surface is a development wrapper only; authentication and provider workers are not implemented yet.
