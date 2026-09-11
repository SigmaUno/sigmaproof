# Celestia feasibility spike: source baseline

Status: source investigation started September 11, 2026; no real-network experiment has passed. Tracks [#3](https://github.com/SigmaUno/sigmaproof/issues/3). comet-archive remains excluded.

## Candidate implementation path

Investigate celestia-node **v0.33.0** (release target `9e954e900951ba06c0e7b8502a7361ffdbd6f91f`). Confirm the resolved tag commit, deployed testnet protocol and matching celestia-app dependencies before freezing runtime pins. The release target alone is not a tested deployment combination.

The released [CommitmentProof implementation](https://github.com/celestiaorg/celestia-node/blob/v0.33.0/blob/commitment_proof.go) carries subtree roots, subtree-to-row proofs, row-to-data-root proofs and namespace fields. Its `Verify(dataRoot, commitment)` method is a candidate for local inclusion verification. SigmaProof must additionally bind the exact manifest bytes, namespace/version, network and independently trusted header. Do not treat the method's success as complete package authentication.

The [service implementation](https://github.com/celestiaorg/celestia-node/blob/main/blob/service.go) has a `GetCommitmentProof` capture path. In the source inspected, `Included` retrieves historical shares again; it must not be the offline verifier. Recheck these behaviors against the pinned release before implementing the adapter. Capture can use online data; exported verification must not.

## Experiment sequence

1. Record exact node/app binaries and hashes, testnet chain ID, configured namespace/share version, RPC capabilities and independently obtained checkpoint provenance. Use synthetic public manifest data only.
2. Submit a synthetic batch manifest through a funded testnet node. Record exact bytes, commitment and receipt; do not mark a submission receipt as captured evidence.
3. Capture the commitment proof, relevant extended header and required trust material while data is available. Record sizes and durations. Repeat with a blob spanning multiple rows.
4. Recompute the blob commitment from the exact bytes using pinned share/commitment rules. Check namespace fields explicitly against the expected namespace and the authenticated data path.
5. Verify the captured inclusion proof against the authenticated header's data root. Establish header authentication from recipient-provisioned trust inputs, not merely from the supplying RPC or a self-consistent carried validator set.
6. Export fixtures and run a standalone verifier with all outbound networking denied, source/daemon/archival services unavailable. Run again in a clean environment.
7. Mutate manifest, commitment, namespace/version, network, height, subtree/row proof and header independently. Missing/untrusted checkpoint must yield incomplete, not authenticated. Exercise nil/empty proof members and oversized inputs before calling upstream verification code.
8. Document checkpoint lifetime, validator changes, expired trusting periods and long-range assumptions. Require reviewer sign-off before marking #3 complete or freezing SEP-6.

## Inputs still needed

A usable testnet node endpoint, funded submission capability, and an independently authenticated checkpoint source. Endpoint/setup details may be shared in the issue; tokens and keys must remain in local secret configuration. No endpoint or checkpoint has yet been selected in this workspace. Until these inputs and the real experiment exist, M0 remains open regardless of unit-test results.
