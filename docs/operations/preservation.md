# Preservation and recovery

Export each complete evidence revision to customer-controlled storage alongside, or durably linked to, the exact document representation. Maintain a secondary backup with suitable access and retention controls. A SigmaProof database copy alone is not the preservation strategy.

Capture provider material immediately after publication becomes verifiable. Monitor ingestion backlog, pending submissions, proof-capture age, missing witnesses, retry exhaustion, package export failures, and backup/restore results. Set capture objectives well inside the shortest supported provider retrieval window; determine actual thresholds in deployment testing.

Reconciliation must distinguish submission failure from unknown outcome so retries do not create uncontrolled duplicate fees. Failed required anchors keep the evidence visibly incomplete. Disaster recovery must rebuild service state from durable records while preserving original package revisions.

Run restore drills on a clean verifier installation using independently provisioned trust context. Archive protocol specifications, verification source/releases, algorithm identifiers, policies, trust evidence, and test vectors with the packages. Optional archival nodes support reconstruction, not ordinary proof verification.

Renewal appends a new commitment over prior evidence and, when needed, fresh object digests while old algorithms remain trustworthy. Rehashing an already broken digest does not recover the lost binding. Renewal intervals depend on risk and trust lifetimes; illustrative five-year intervals and ten-year retention are not legal defaults.
