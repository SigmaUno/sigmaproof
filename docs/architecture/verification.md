# Verification model

Status: proposed

## Levels

| Level | Inputs | Result |
| --- | --- | --- |
| 1: offline | Object bytes and portable package | Digests, commitments, event links, Merkle paths, signatures against supplied keys, internal consistency |
| 2: authenticated anchor | Level 1 plus independently authenticated anchor state/trust context | Publication inclusion and provider-specific external assertions |
| 3: reconstruction | Historical provider data, often archival access | Rebuild publication and regenerate forensic evidence |

Level 2 may also work offline when sufficient independently trusted checkpoint material has been provisioned. Network access alone does not confer authenticity. A self-consistent carried header or certificate is not automatically a trusted root.

## Result model

Each check reports `verified`, `failed`, `unavailable`, `unsupported`, or `not_applicable`, with reason and evidence reference. Policy reports `satisfied`, `not_satisfied`, or `indeterminate`. Unavailable network state and unsupported mandatory algorithms cannot silently count as success.

Report claimed source time, ingestion time, independently observed anchor time, and qualified timestamp time separately. Chain ordering does not establish exact wall-clock creation time. A valid signature establishes possession of a key; identity and authorization require additional trust evidence.

The experimental CLI now supports `sigmaproof verify --offline --json document.pdf document.sigmaproof` for the unanchored profile. Exit 0 means requested local integrity checks passed; default verification requires anchor authentication and returns 3 while that remains unavailable. Invalid evidence/I/O errors return 1; usage errors return 2; unsupported package features return 3. Both modes perform no networking. See the [CLI contract](../../cmd/sigmaproof/README.md). Caller-selected trust/policy configuration and anchored profiles are not implemented; this local mode does not evaluate policies or establish external history.

## Completeness

Event links detect modification and missing interior links relative to an authenticated chain head. They do not reveal an omitted final suffix, withheld events never committed, or a fork when the recipient sees only one branch. Completeness requires known checkpoints, externally committed sequence heads, or additional monitoring. An old valid proof can remain valid while being stale.

## Browser verifier

Hash and validate documents locally; do not upload bytes. Display network requests needed for anchor or trust validation. Avoid automatic requests to arbitrary URLs carried by untrusted packages. Bundle or explicitly configure trusted adapters and trust roots. Show integrity and policy conclusions separately.
