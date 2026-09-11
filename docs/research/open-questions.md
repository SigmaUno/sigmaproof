# Open engineering questions

Resolve before protocol stabilization:

1. Which exact Celestia node/app versions and proof objects support the full manifest-to-authenticated-header verification path? How are expired trust periods handled?
2. Which independently provisioned historical checkpoints are acceptable, and how are their provenance and renewal preserved?
3. Which canonical serialization, framing, algorithm registry, extension rules, and package resource limits define SEP-1?
4. How are absent metadata, original/derived objects, tenant-scoped versions, actor assertions, and chain heads encoded?
5. How are concurrent events, forks, hidden suffixes, and known-latest checkpoints represented?
6. Which provider failure/finality states, independent-provider rules, and historical trust evidence are required by policy?
7. How are qualified-service status, certificate revocation evidence, and policy signatures preserved for historical verification?
8. Which artifacts must be renewed before digest or signature deprecation, and how is object-to-new-digest binding maintained?
9. What throughput, package-size, latency, and cost objectives follow from a representative workload?

Each answer should become a reviewed SEP/ADR with acceptance tests, not an implicit implementation convention.

Work in progress: [Celestia feasibility spike](celestia-spike.md).
