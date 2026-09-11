# Merkle primitive

Implements [RFC 6962 section 2.1](https://www.rfc-editor.org/rfc/rfc6962.html#section-2.1) using SHA-256: empty root hashes zero bytes, leaf hashes prefix `00`, and internal nodes hash prefix `01` followed by two 32-byte child hashes. Split at the largest power of two strictly below the tree size; never duplicate an odd leaf. Paths contain sibling hashes from leaf to root. Direction is derived from the zero-based index and tree size.

`Root` and `Path` accept raw entries (not already-prefixed leaf hashes). `Verify` checks nonempty size, index bounds, exact path length, and the resulting root. Its path depth is bounded to 64 for uint64 tree sizes. Generation is O(n) per root/path; service-level batch/input limits remain the caller's responsibility.

This is a tree primitive, not a finalized SigmaProof wire format or document verifier. The caller must authenticate the manifest including tree size, algorithm and root. Some different size claims can describe the same local path structure, so the root alone does not authenticate the claimed size. Duplicate entries also cannot establish unique position from their bytes alone.

The [fixtures](testdata/rfc6962.json) cover all leaves for sizes 1, 2, 3, 5, 7, 8, 9, 16 and 17, plus the empty root. Reproduce with `python3 scripts/generate_merkle_vectors.py` from the repository root. The generator combines nodes iteratively; Go uses recursive splits. This supplies a second-language cross-check, not independent external protocol review. Negative tests cover changed entries/siblings, extra/truncated paths, wrong index, a wrong-size case and invalid bounds. A fuzz target exercises malformed proof inputs.
