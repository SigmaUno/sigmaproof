# MVP definition and acceptance

## Included

Paperless integration without a fork; exact-byte document commitments with private random nonce; Merkle batching; Celestia test-network integration and complete witness capture; portable package export; independent offline and authenticated-anchor verification.

Metadata payload commitments, lifecycle events, signatures, policies, browser UI, OpenTimestamps, and qualified services follow in later phases. The MVP encoding must explicitly represent absent metadata and reserve versioning without pretending to validate absent features.

## Release gates

- A stable original or derived Paperless representation is selected and identified; a changed representation produces distinct evidence.
- Duplicate webhooks and worker restarts preserve idempotency and recover pending capture.
- Public payload inspection finds no document hash, private metadata, filename, source ID, or nonce; only approved opaque commitments and public protocol fields are emitted.
- Canonicalization, nonce lengths, domain separation, leaf/node hashing, tree sizes, and path direction have published interoperable vectors.
- Altered document, nonce, manifest, Merkle path, namespace, network, and provider witness fail appropriate checks.
- Offline verification succeeds without any network; it does not falsely report authenticated anchors.
- Independent anchor verification succeeds with historical blob retrieval disabled and SigmaProof unavailable. Real captured fixtures and independently authenticated trust inputs are required.
- Missing or untrusted checkpoints yield an explicit incomplete result.
- Customer-controlled exports survive a backup/restore drill and reproduce results on a clean environment.
- Parser limits and malformed-package tests prevent path traversal, uncontrolled memory allocation, and unexpected outbound requests.

## Demonstration

Import a synthetic PDF into Paperless, capture evidence, export the PDF and package, remove dependency on the source/core services, deny historical blob APIs, and verify independently. Publish reproducible instructions, version pins, proof sizes, timing, trust assumptions, and test output. No real customer data belongs in the demonstration.
