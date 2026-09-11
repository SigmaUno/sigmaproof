# Experimental local proof CLI

Build with `make build`. The CLI can create and verify local, explicitly unanchored packages using the experimental [SEP-1 profile](../../spec/SEP-001.md). It does not contact SigmaProof or a network, and cannot authenticate publication, time or provenance. No Celestia adapter is implemented yet.

```sh
printf 'Synthetic SigmaProof document\n' > document.txt
./bin/sigmaproof create --unanchored --representation original document.txt document.sigmaproof
./bin/sigmaproof verify --offline --json document.txt document.sigmaproof
```

Creation makes a fresh randomized commitment and single-leaf manifest, then writes the private package without overwriting any destination. File permissions are 0600. Keep the package private because it contains a raw document digest and nonce. `original` versus `derived` is an explicit producer assertion, not a verified source classification.

`verify --offline` exits 0 only when structure, document digest and batch membership all pass; the result still states that the anchor is unavailable. Omitting `--offline` requires anchor authentication, so even a locally consistent package exits 3. Both modes make no network requests. A self-consistent substituted document/package pair can pass local checks; only independent anchoring can bind the manifest to external history.

| Exit | Meaning |
| --- | --- |
| 0 | Requested local checks passed, or creation/help/version succeeded |
| 1 | Invalid evidence, read/write failure or document size limit exceeded |
| 2 | Invalid command/arguments |
| 3 | Unsupported package feature or requested anchor authentication unavailable |

Flags precede the two paths. `--json` on verify outputs `format`, `document`, `batch_inclusion`, and `anchor`, each with status and reason. There is no overall authenticated boolean. File-open failures are printed to stderr and exit 1 before a report exists. Policies, signatures and timestamps are not evaluated.

Both commands default to a 64 MiB document limit; override with `--max-document-bytes N`. Zero permits only an empty document. Packages are limited to 682 bytes. CLI inputs must be regular files; readers of the library API must provide their own cancellation/deadlines. Package data cannot select a file or network address. The daemon still exposes only health/readiness endpoints.
