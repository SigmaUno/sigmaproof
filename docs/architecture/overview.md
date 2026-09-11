# System architecture

Status: proposed

## Components and boundaries

| Component | Responsibility | Trust boundary |
| --- | --- | --- |
| Source DMS | Authoritative bytes, versions, workflows | Customer operational system |
| Integration agent | Fetch exact representation, hash locally, authenticate ingestion | Source credentials and tenant |
| Evidence core | Commitments, version identity, event links, batching | Evidence operator |
| Provider adapters | Submit, poll, capture proof, verify assertions | Independent anchor/service |
| Package exporter | Preserve complete evidence and immutable revisions | Customer retention system |
| Verifier | Parse defensively, verify cryptography, authenticate anchors, evaluate policy | Recipient-selected trust roots |

Document bytes normally remain inside the customer's environment. Tenant-scoped source identifiers and metadata stay private. A commitment is not a globally meaningful document ID; randomized commitments for the same bytes differ.

## Data flow

1. Agent obtains stable bytes and explicitly identifies original versus derived/OCR representation.
2. It computes document and optional metadata digests and creates a cryptographically random 32-byte nonce.
3. Core records a commitment and receipt of ingestion transactionally, using an idempotency key scoped to tenant and source version.
4. A batch worker freezes the ordered leaf set and creates the root and manifest.
5. Providers submit the exact bound manifest or commitment; asynchronous receipts progress through pending, submitted, proof-captured, authenticated, or failed states.
6. Export is marked complete only when required portable material is present. Submission success alone is insufficient.
7. Customer-controlled storage receives an immutable package revision. Subsequent anchors and renewals append evidence without rewriting the historical assertions.

Exact encodings and transitions require SEP review. Metadata is optional in MVP; its absence must have an unambiguous encoding and must never appear as verified metadata.

## Anchor contracts

The concept's Submit/Verify interface needs asynchronous lifecycle support. Proposed operations are Submit(batch, idempotencyKey), Poll(receipt), Capture(receipt), and Verify(bundle, trustContext). Implementations must declare supported network, proof format, finality assumptions, and required trust inputs.

Trust-service adapters separately represent timestamp tokens, signatures, seals, qualification evidence, and service validation. A single boolean return cannot express these guarantees.

## Reliability

Use a durable queue/outbox for committed ingestion, bounded retries with jitter, reconciliation after ambiguous submissions, and visible terminal failures. Duplicate requests must not create unintentional new evidence events. Do not log secrets, private package contents, or raw source identifiers. Separate read/export authorization from administration and provider credentials.

## Implementation layout

`cmd/` holds CLI and daemon skeleton entrypoints, `internal/` holds service implementation, `pkg/proof/` is the future independently reusable verifier, `integrations/` holds adapters, and `spec/` defines language-independent behavior. The verifier must not import the daemon or require its database.
