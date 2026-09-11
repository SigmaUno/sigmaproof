# Private transactional storage

Implements the local persistence slice of [#6](https://github.com/SigmaUno/sigmaproof/issues/6) using pinned bbolt v1.5.0. It is an internal library with no HTTP endpoints or daemon startup wiring yet. The offline proof library/CLI remain independent of storage.

## Contract

`Open(path)` uses a mode-0600 database, sync-enabled transactions and an exclusive process lock with a one-second acquisition timeout. New parent directories use mode 0700. Existing symlinks, nonregular or group/world-accessible database files are rejected. Choose a trusted local directory/filesystem; shared storage and multiple process writers are not supported. DB contents, backups and source indexes are private and unencrypted; customer storage access controls remain required.

- `Ingest(request)` atomically writes evidence, tenant-scoped idempotency and source indexes, and a FIFO pending entry. The source tuple is instance, object, immutable version and representation. Tenant/key/source components must be valid nonempty UTF-8, at most 256 bytes each, without NUL. Identical retries preserve the ID, witness and sequence. Another key for an identical source/witness aliases the same record. Changed source or witness under an existing key conflicts; a new key cannot bypass a changed witness for the same source tuple. New source versions and representations need distinct idempotency keys.
- `Evidence(tenant, id)` returns a private record only inside that tenant. Unknown tenants and inaccessible records both return not found. There is no global ID lookup.
- `Freeze(tenant, requestKey, limit)` atomically selects FIFO pending entries, creates the exact public manifest, assigns members their batch/index, removes those pending entries, and writes the batch request index and pending outbox entry. Limits are 1–1,024 per transaction (below the protocol's 65,536 cap). Retrying a successful freeze returns the same batch, including after restart or new ingestion. Changing its limit conflicts. Empty selections create no batch/request key.
- `Outbox(tenant, limit)` reads pending manifest records in batch-ID order. Returned bytes are copies. The ID is opaque local bookkeeping; **only the 61 manifest bytes are the proposed anchor payload**. This does not claim/lease/submit/acknowledge work. Every entry is still pending.
- `UnanchoredPackage(tenant, evidenceID)` reconstructs a local package from frozen records and verifies member indexes and the stored root before returning it. It cannot export pending/unbatched evidence or claim publication. Export must subsequently verify against original document bytes using the independent verifier.

Tenant identity must eventually come from authenticated credentials, not a caller-controlled body field. This library provides namespace isolation, not authentication. There is no HTTP access until #6's authentication, request schema, error mapping, rate/body limits and resource policy are implemented.

## Persistence and boundaries

A schema marker rejects unknown future schemas and foreign databases without migration. Each tenant has evidence, idempotency, source, pending, batch, batch-request and outbox buckets. JSON is private internal record encoding, not a signed or public format. Record decoding is capped at 128 KiB. Public manifests and exported packages retain their existing exact binary encodings. Identifiers are random 128-bit values, never source IDs or document hashes.

The agent must create and persist its nonce/witness before first ingestion and reuse it on retries. The store cannot determine nonce entropy or source authenticity. Regenerating a nonce for the same source version conflicts deliberately; silently accepting changed private proof material would hide an inconsistent retry.

Return values are exposed only after transaction success. bbolt serializes write transactions; there are no external calls inside them. Each frozen batch has one pending outbox entry. Deleting/claiming entries, provider receipts, leases, retry scheduling and reconciliation of ambiguous submissions remain future adapter work. In particular, this implementation makes no exactly-once network-delivery claim.

Tests cover concurrent duplicate ingestion/freezes, nonoverlapping distinct batches, rollback of all indexes/outbox changes, tenant isolation, private permissions, process locking, future/foreign schemas and corruption checks. A subprocess commits ingestion and a batch then calls `os.Exit` without `Close`; reopening recovers the same witness/batch/outbox and produces a locally verifiable package. This is process-exit recovery, not a power-loss test or backup/restore drill. Full retention, migration, quotas, backup/restore and production filesystem validation remain release work.

Reference: [bbolt transactions and operational caveats](https://github.com/etcd-io/bbolt/tree/v1.5.0). Dependency notices are in [THIRD_PARTY.md](../../THIRD_PARTY.md).
