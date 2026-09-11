# Roadmap

M0 targets September 17, 2026; M1 targets October 8, 2026, subject to the gates and capacity in the [release plan](docs/delivery/release-plan.md). Later phases remain undated. A buildable skeleton exists; no evidence engine is implemented.

| Milestone | Deliverable | Exit criterion |
| --- | --- | --- |
| M0 — Protocol foundation | Threat model, encoding decisions, Celestia capture spike | Reviewed draft and real pruning-independent proof experiment |
| M1 — Paperless and Celestia MVP | Agent, commitments, batches, capture/export, CLI | All [MVP gates](docs/product/mvp.md) pass |
| M2 — Lifecycle and multi-anchor | Metadata, events, org signatures, policies, browser verifier, OpenTimestamps | Per-assertion verification and policy tests with independent providers |
| M3 — EU trust and renewal | QTSP timestamps/seals, validation evidence, renewal, reviewed profiles | Historical trust validation and renewal demonstrated; specialist review |
| M4 — Enterprise integrations | S3, Nextcloud, SharePoint/M365, ERP, KMS/HSM, SIEM | Documented adapter contracts and integration-specific acceptance |

## First engineering sequence

1. Define trust/checkpoint requirements and execute the Celestia feasibility spike.
2. Resolve canonical serialization and publish commitment/Merkle vectors.
3. Specify the envelope and implement a standalone bounded parser/verifier.
4. Implement transactional ingestion, batching, provider capture, and export.
5. Add Paperless reconciliation and demonstrate the full acceptance scenario.

Do not defer the historical-proof experiment until after building the service: it determines whether the central product claim is supportable.

## Tracked engineering work

- M0: [encoding and vectors](https://github.com/SigmaUno/sigmaproof/issues/2), [Celestia experiment](https://github.com/SigmaUno/sigmaproof/issues/3), [envelope and verification results](https://github.com/SigmaUno/sigmaproof/issues/4).
- M1: [independent verifier](https://github.com/SigmaUno/sigmaproof/issues/5), [ingestion and batching](https://github.com/SigmaUno/sigmaproof/issues/6), [capture and export](https://github.com/SigmaUno/sigmaproof/issues/7), [Paperless agent](https://github.com/SigmaUno/sigmaproof/issues/8), [acceptance demonstration](https://github.com/SigmaUno/sigmaproof/issues/9).
- M2: [lifecycle and multi-anchor](https://github.com/SigmaUno/sigmaproof/issues/10).
- M3: [qualified services and renewal](https://github.com/SigmaUno/sigmaproof/issues/11).
- M4: [enterprise integration](https://github.com/SigmaUno/sigmaproof/issues/12).

comet-archive is excluded from M0/M1. [Post-release evaluation #18](https://github.com/SigmaUno/sigmaproof/issues/18) tracks optional archive/recovery infrastructure; portable verification must work with archive services disabled.
