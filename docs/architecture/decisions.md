# Architecture decisions

| ID | Decision | Status | Consequence |
| --- | --- | --- | --- |
| ADR-001 | Start with one repository | Accepted for foundation | Protocol and implementation changes can be reviewed together; split only with ownership/release need |
| ADR-002 | Documents stay in source systems | Accepted | Agent must bind exact version/representation |
| ADR-003 | Portable independent evidence | Accepted | Export and verification are release gates |
| ADR-004 | Pluggable anchors and separate trust services | Accepted | Provider-specific assertions remain visible |
| ADR-005 | Preserve Celestia witness; no historical blob dependency | Accepted requirement | Proof capture/authentication spike precedes production work |
| ADR-006 | Go reference implementation | Proposed | Pin toolchain after provider compatibility research |
| ADR-007 | SHA-256 and RFC 6962-style tree initially | Proposed | Exact encoding and adversarial vectors precede protocol stabilization |
| ADR-008 | No regulatory certification claims in MVP | Accepted | Qualified profiles require specialist review |

New decisions should state context, alternatives, outcome, consequences, and superseded decisions. Accepted product requirements do not imply an implemented capability.
