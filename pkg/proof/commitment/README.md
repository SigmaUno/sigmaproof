# Experimental document commitments

Implements the exact 90-byte [draft SEP-2 profile](../../../spec/SEP-002.md). This package is a cryptographic building block; it does not parse `.sigmaproof` packages, authenticate a source or anchor, or establish document truth. The profile and exported API are experimental pending review.

- `New(reader, representation, maxBytes)` hashes exact bytes with bounded reads and a streaming digest, generates a fresh CSPRNG nonce, and returns private witness plus opaque commitment. Zero limit permits only an empty document. Negative limits and MaxInt64 are rejected to prevent limit arithmetic overflow. The reader must eventually return or provide its own cancellation/deadline.
- `Encode(witness)` returns the private fixed-width preimage. Never publish or log it.
- `Compute(witness)` reproduces an existing commitment; use `New` for new evidence so nonce generation is not accidentally skipped.
- `Verify(reader, witness, expectedCommitment, maxBytes)` rejects unsupported fields, checks the commitment and streams document bytes to check their digest. It performs no networking. The caller must independently authenticate the expected commitment through the manifest and anchor.

Witnesses contain sensitive material and must be retained privately. The service's future transactional ingestion layer must persist the witness once and reuse it for idempotent retries. This library does not provide persistence. Decoders must enforce digest/nonce lengths before constructing fixed-size arrays; this package is not a JSON parser.

The [test fixtures](testdata/sep2-draft.json) are generated in Python and checked in Go. Tests also exercise read errors, byte limits, unsupported fields, mutations and composition with the Merkle library using only the opaque commitment as the leaf entry. Test nonces are deterministic; production nonces are not.
