# Experimental public batch manifests

Implements the proposed [SEP-5 profile](../../../spec/SEP-005.md). `New` builds a manifest from ordered opaque SEP-2 commitments; `MarshalBinary` produces exactly 61 public bytes. `Parse` rejects incorrect length/domain, unsupported profiles/algorithms and sizes outside 1–65,536 before returning a value. Parsing does not allocate based on the declared tree size.

`Verify` checks inclusion of a commitment relative to the manifest. It does not authenticate the manifest. `Digest` supplies a local content address, not a publication receipt. The future Celestia adapter must publish the exact manifest bytes and independently verify their inclusion. It must not publish only the tree root or substitute the local digest for this profile's payload.

The manifest is deterministic for an ordered batch. Its root and count are public; this reveals batch cardinality. Document bytes, raw document digests, nonces, source identifiers and representation fields are absent. Ingestion must pass only randomized commitments; this library cannot detect whether a caller mislabeled a raw document digest as a commitment.

Construction returns a value without retaining the input slice. The service must separately freeze/persist the ordered commitments and manifest in one durable transaction and reuse those bytes on retries. Per-path generation currently costs O(n); generating every path separately would cost O(n²), so the future batching service needs a cached tree or bulk path generator. The 65,536-entry limit is an experimental profile bound, not a measured throughput promise.

Python-generated fixtures cover ordered and reversed batches of sizes 1, 2, 3, 5, 8 and 9. Go tests check exact bytes/digests, all fixture memberships, strict parsing, privacy fields and a maximum-size batch. A dedicated test demonstrates why size authentication needs the full manifest even when an inclusion path still matches. Fuzzing requires every accepted byte string to round-trip exactly.
