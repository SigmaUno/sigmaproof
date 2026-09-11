# Privacy and key management

Generate each nonce using a cryptographically secure random generator. Keep nonces, document digests, metadata, actor identifiers, and source mappings private. Batch manifests may include only reviewed non-identifying protocol fields and commitments; customer-selected IDs must never flow into public fields unchecked.

Randomization mitigates straightforward matching against known documents but does not guarantee anonymization. Protect packages with customer access controls, encryption at rest, retention rules, backups, and minimal logging. Evaluate applicable privacy obligations for the deployment; do not market public commitments as automatically outside GDPR.

Organizational signatures bind evidence to a key. Identity and authority depend on the trust context. Plan adapters for AWS KMS, Google Cloud KMS, Azure Key Vault, Vault, and HSMs after the core evidence format is stable. Do not store production private keys in configuration or examples.

Preserve algorithm and key identifiers, certificates/chains where applicable, signing scope, and validation evidence. Rotate keys without discarding past verification material. Document revocation and compromise times, historical status evidence, and what the verifier can conclude when evidence is unavailable. Qualified seals and timestamps require service-specific verification beyond a mathematically valid signature.
