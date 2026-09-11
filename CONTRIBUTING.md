# Contributing

Start with the [roadmap](ROADMAP.md) and open an issue for a protocol or architectural change. Small documentation fixes can go directly to a pull request.

1. Create a branch from `main`.
2. Keep the change focused and update linked specifications/examples.
3. Install Go 1.27.1 and Python 3.9+, then run `make check build` (or `make docs-check` for documentation only).
4. Describe the problem, resulting behavior, verification, and remaining limits in the PR.

Protocol changes must document encoding, compatibility, privacy implications, and positive/negative vectors. Do not present pseudocode as a finalized wire format or unimplemented adapters as working features. Later implementation PRs must add relevant tests and pin supported toolchains/dependencies.

Use synthetic data only. Never include private documents, production proofs, credentials, or customer identifiers. Report vulnerabilities through [the security policy](SECURITY.md).

Maintainers review contributions; no response-time commitment is currently offered. License terms are stated in the repository licensing notice.
