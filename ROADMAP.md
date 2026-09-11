# Roadmap

Milestones describe outcomes, not promised dates. No phase is implemented yet.

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
