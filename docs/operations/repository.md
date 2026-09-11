# Repository administration

The foundation uses one repository, `SigmaUno/sigmaproof`, for product documentation, SEPs, implementation boundaries, and integration plans. Split repositories only when independent releases or ownership justify the overhead.

Initial visibility is private pending the owner's public-launch decision. Licensing is undecided; see [licensing status](../../LICENSING.md). The open-protocol vision does not itself grant a software license.

Repository setup includes issues, phase milestones, starter engineering tasks, PR/issue templates, documentation CI, dependency updates for Actions, and code ownership. The wiki is disabled so documentation remains version-controlled. No organization-wide settings or unrelated repositories should be changed.

Use `main` as the default branch. Prefer squash merges and branch cleanup. Apply branch protection requiring documentation checks and resolved review conversations where the organization plan supports it. Record any unavailable controls rather than claiming they are enforced. Maintainer review remains required by contribution policy even when GitHub cannot enforce it.

Before public launch: choose a license, confirm visibility, establish private reporting availability and named maintainer contacts, and review the repository for sensitive material. Before software release: add implementation CI, supported versions, signed release artifacts, dependency/security scanning appropriate to the actual code, and reproducible verification fixtures.
