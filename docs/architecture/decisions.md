# Architecture decisions

| ID | Decision | Status | Consequence |
| --- | --- | --- | --- |
| ADR-001 | Start with one repository | Accepted for foundation | Protocol and implementation changes can be reviewed together; split only with ownership/release need |
| ADR-002 | Documents stay in source systems | Accepted | Agent must bind exact version/representation |
| ADR-003 | Portable independent evidence | Accepted | Export and verification are release gates |
| ADR-004 | Pluggable anchors and separate trust services | Accepted | Provider-specific assertions remain visible |
| ADR-005 | Preserve Celestia witness; no historical blob dependency | Accepted requirement | Proof capture/authentication spike precedes production work |
| ADR-006 | Go reference implementation | Accepted for skeleton | Go 1.27.1; confirm provider compatibility during #3 |
| ADR-007 | SHA-256 and RFC 6962-style tree initially | Proposed | Exact encoding and adversarial vectors precede protocol stabilization |
| ADR-008 | No regulatory certification claims in MVP | Accepted | Qualified profiles require specialist review |
| ADR-009 | Embedded bbolt for initial single-process private storage | Proposed; implemented for review | Atomic ingestion/batch/outbox transactions; exclusive local DB writer; no distributed DB or provider exactly-once claim |

New decisions should state context, alternatives, outcome, consequences, and superseded decisions. Accepted product requirements do not imply an implemented capability.


## ADR-009 context and tradeoffs

The initial service needs one atomic commit spanning ingestion, idempotency/source indexes, frozen batch membership and pending submission work. bbolt provides embedded transactions without introducing a database service or a C dependency. SQLite was considered for SQL queries/migrations; PostgreSQL for concurrent service instances. Those capabilities are not required for this first internal storage slice.

The implementation uses bbolt v1.5.0 with sync enabled and one process owning the database. Consequences: application-managed indexes/schema, serialized writes, trusted local storage and no high availability. Benchmark representative workloads before release. Choose a reviewed migration path if multi-process writes, SQL reporting or larger workloads require a different backend. This decision does not select a provider or weaken anchor trust requirements.
