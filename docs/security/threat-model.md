# Threat model

Status: initial design review; not a security audit.

Assets include original bytes, private nonces, source/version mappings, policy definitions, signing keys, provider credentials, preserved witnesses, and verifier trust roots. Adversaries may control a DMS administrator, evidence database, core server, network endpoint, package input, or one anchor. Multiple providers can still share operators or failure modes.

| Threat | Intended control | Residual limitation |
| --- | --- | --- |
| Document/metadata changes | Commitments and digest checks | Uncommitted data is outside the assertion |
| Interior event modification | Linked hashes and authenticated head | Suffix omission, forks, or unrecorded events need additional checkpoints/monitoring |
| Database replacement | Compare against preserved externally anchored evidence | Attacker can withhold evidence or present an old valid state |
| Core compromise | External commitments protect previously authenticated history | New false assertions remain possible; source truth is not proven |
| Candidate-document matching | Private random nonce; publish only opaque roots | Package disclosure enables matching; traffic/batch timing may leak relationships |
| Anchor compromise | Policy-defined independent anchors | Counts alone do not establish independence; fail policy when required trust is lost |
| Historical pruning | Export complete witness early | Loss of customer bundle or trustworthy checkpoint can defeat verification |
| Key theft | KMS/HSM, scoped roles, rotation, incident response | Signatures during compromise may be unreliable |
| Hostile package | Bounded parser, reject unsupported critical fields, no arbitrary fetching | Requires fuzzing and security review before release |
| Webhook forgery/replay | Authentication, tenant isolation, idempotency, source re-fetch | Compromised source can still lie |

Authenticity of claimed creation time, identity, authorization, history completeness, and regulatory qualification must each have explicit supporting evidence. Never infer all of them from one successful Merkle check.
