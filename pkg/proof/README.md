# Independent proof library

`CreateUnanchored` creates a development package for one document. `Package.MarshalBinary`, `Parse` and `Read` implement the strictly bounded [unanchored envelope](../../spec/SEP-001.md); `Verify` streams document bytes and reports format, digest and batch inclusion checks separately from the always-unavailable anchor assertion. `IntegrityVerified` describes only local consistency.

The [commitment subpackage](commitment/README.md) provides private randomized commitments, the [Merkle subpackage](merkle/README.md) provides roots/paths, and the [batch subpackage](batch/README.md) binds algorithm/profile/count/root in an exact public manifest.

There are no daemon, database or network dependencies, external references, archive extraction or URL fetching. The envelope read is capped at 683 bytes; document reads use a caller-specified limit. Input readers are caller-provided and must have appropriate cancellation/deadlines. Serialized packages are private and immutable. Parsing rejects unknown features instead of ignoring them.

This implements local integrity only. Independently authenticated provider receipts, trust inputs, timestamps, policy evaluation, revisions and the complete anchored MVP envelope remain unfinished. Synthetic Python fixtures cross-check all encodings; they are not independent external review or real-network evidence.
