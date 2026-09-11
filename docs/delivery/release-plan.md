# v0.1.0 testnet developer preview

Planning baseline: September 11, 2026. Target: **October 8, 2026**, 27 elapsed days later. This is a conditional delivery plan, not a claim that the MVP already works. Public release requires all MVP gates; a foundation-only preview must be explicitly renamed and scoped if those gates fail.

## Scope and capacity

Ship one supported Paperless representation, exact-byte private randomized commitments, Merkle batches, one Celestia testnet, durable capture/export, and standalone CLI verification with independently configured trust inputs. Use the existing [MVP acceptance criteria](../product/mvp.md) as the release contract.

Defer lifecycle, metadata commitments, organizational signatures, policy engine, browser UI, other anchors, qualified services, enterprise integrations, mainnet, and hosted multi-tenant SaaS. Authentication and tenant isolation of any supported ingestion interface remain mandatory.

Assume two dedicated engineers (protocol/provider and core/integration) plus a reviewer available for four days. Budget approximately 38 engineer-days against about 40 available weekdays, with review/rework included below. This is aggressive: the Celestia spike is an unresolved feasibility dependency. Owner must confirm names and capacity by September 14 in [#14](https://github.com/SigmaUno/sigmaproof/issues/14). Roles below are proposed responsibilities, not accepted assignments. A single engineer cannot credibly commit to this entire scope in the same window.

comet-archive forking, integration and deployment are excluded from this first release. Capture the complete required Celestia witness at anchoring/capture time and preserve it in the portable package; verification uses that package and independently provisioned trust inputs with archive services disabled. The September 17 feasibility gate is unchanged. [Post-release evaluation #18](https://github.com/SigmaUno/sigmaproof/issues/18) covers optional historical recovery and is not a release dependency. “v1” in discussion refers to this first preview; the release version remains v0.1.0.

## Review findings

| Severity | Evidence at baseline | Release implication / action |
| --- | --- | --- |
| Blocker | cmd/, internal/, pkg/proof/ contained README placeholders only | No product path existed; land skeleton #13, then #5–#8 |
| Blocker | Celestia witness and independent checkpoint handling unresolved in SEP-6 and architecture | Prove real capture/authentication #3 before committing to anchored release |
| Blocker | SEP-1/2/5 still require exact encoding, envelope and tree rules | Resolve #2/#4 before format-dependent implementation |
| Blocker | LICENSING.md reserves owner selection; no LICENSE | Owner decision #14 before distribution |
| High | No parser, recovery, privacy, isolation or real-fixture tests | Implement acceptance tests in #5–#9 and review #15 |
| High | Existing milestone dates unset; no artifact/install process | Date milestones; complete #16/#17 |

The review covers the baseline repository and existing issues #2–#12. It is an architecture/readiness review, not a security audit of a working engine. The new skeleton supplies commands and operational probes only; it deliberately implements no draft cryptography or simulated anchor success.

## Delivery schedule

All dates are 2026. Estimates are engineer-days, including implementation tests.

| Work | Owner role | Budget | Due | Dependencies / completion evidence |
| --- | --- | --- | --- | --- |
| [#13 Skeleton and CI](https://github.com/SigmaUno/sigmaproof/issues/13) | Core | 1 | Sep 14 | Clean build, non-success verify, unavailable readiness, CI green |
| [#14 License and staffing](https://github.com/SigmaUno/sigmaproof/issues/14) | Project owner | Owner time | Sep 14 | Approved terms; named owners and capacity |
| [#3 Celestia feasibility](https://github.com/SigmaUno/sigmaproof/issues/3) | Protocol | 4 | Sep 17 | Real preserved witness; blob APIs denied; independent checkpoint and negative controls |
| [#2 Encoding and vectors](https://github.com/SigmaUno/sigmaproof/issues/2) | Core + protocol review | 2 | Sep 16 | Exact framing, absent metadata, tree edge cases; independent vectors |
| [#4 Envelope/results](https://github.com/SigmaUno/sigmaproof/issues/4) | Protocol | 1 | Sep 17 | #2/#3; bounded format, trust inputs and exit semantics frozen |
| [#5 Verifier/CLI](https://github.com/SigmaUno/sigmaproof/issues/5) | Protocol | 4 | Sep 23 | #2/#3/#4; offline positive and adversarial fixtures |
| [#6 Durable core](https://github.com/SigmaUno/sigmaproof/issues/6) | Core | 4 | Sep 22 | #2/#4; restart-safe outbox, idempotency and privacy checks |
| [#7 Capture/export](https://github.com/SigmaUno/sigmaproof/issues/7) | Protocol | 3 | Sep 28 | #3/#4/#6; initial end-to-end slice Sep 24, retries/export hardening Sep 28 |
| [#8 Paperless](https://github.com/SigmaUno/sigmaproof/issues/8) | Core | 3 | Sep 25 | #6; stable representation, reconciliation; #7 for final export validation |
| [#9 Demo and restore](https://github.com/SigmaUno/sigmaproof/issues/9) | Both | 4 | Oct 1 | #5–#8; synthetic end-to-end demonstration, clean restore |
| [#15 Security hardening](https://github.com/SigmaUno/sigmaproof/issues/15) | Both + reviewer | 4 + 4 review days | Oct 2 | Review incrementally from Sep 18; fix findings, attach gate evidence |
| [#16 Packaging/runbook](https://github.com/SigmaUno/sigmaproof/issues/16) | Core | 2 | Oct 5 | License + working core; candidate binaries, checksums, install/rollback |
| [#17 Release decision](https://github.com/SigmaUno/sigmaproof/issues/17) | Release owner + both | 6 | Oct 8 | Oct 6–8 reserved for clean candidate checks, fixes and release decision |

Critical path: #3 → #4 → #5/#7 → #9 → #15 → #17. Core storage scaffolding and encoding can proceed alongside the spike; format-dependent code waits for reviewed decisions. Do not close broad issues merely because their initial slice runs.

## Checkpoints and scope control

- **September 14:** confirm capacity, owners, license and testnet credentials/budget. If unavailable, reforecast immediately.
- **September 17 (M0):** real pruning-independent verification works and wire/trust contracts are reviewed. If not, stop promising a Celestia MVP for October 8. Record the technical blocker and decide whether to deliver a clearly labeled foundation preview; do not substitute mocks for evidence.
- **September 24:** a synthetic local source can ingest, capture, export and verify outside the daemon. Escalate missing critical-path work that day.
- **October 1:** Paperless demo and restore pass. Freeze features. Only blocker fixes and release work follow.
- **October 5:** candidate artifacts and clean-machine instructions ready. October 6–8 are reserved for final checks and corrections.
- **October 8 (M1):** release owner records go/no-go against every MVP gate. Open critical/high security findings, incomplete witnesses, false authentication results, failed recovery, or unresolved licensing are no-go conditions.

Maintain a daily update in #17: completed evidence, next task, blocker, owner, and revised forecast. Link each release gate to a test/fixture/log and reviewed commit. At least twice weekly review remaining effort against capacity. Cut deferred features first; never cut proof authentication, parser safety, privacy, or restore correctness to meet the date.

## Release evidence checklist

- [ ] #2–#9 and #13–#16 accepted with evidence, not just merged code.
- [ ] Every [MVP gate](../product/mvp.md) has a passing test or reproducible manual record.
- [ ] Real testnet versions, network, witness, independent checkpoint provenance and trust limits documented.
- [ ] Fresh-machine verification succeeds with SigmaProof/source services and historical blob APIs unavailable.
- [ ] Wrong bytes/nonce/path/manifest/network/namespace/witness fail; missing/untrusted checkpoints report incomplete.
- [ ] Backup/restore, restart/retry, privacy and parser bounds demonstrated.
- [ ] License, artifact checksums, dependency inventory, release notes and install/rollback instructions included.
- [ ] Named reviewer signs off the candidate commit; release owner publishes or records no-go in #17.
