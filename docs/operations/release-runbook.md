# Release runbook

This runbook prepares the experimental v0.1.0 developer preview. It does not create a production service, regulatory certification, authenticated Celestia proof or customer-ready Paperless deployment.

## Preconditions

- Licensing decision in `LICENSING.md` is resolved before any public artifact distribution.
- Release owner records the go/no-go decision in [#17](https://github.com/SigmaUno/sigmaproof/issues/17).
- `make check` passes on the candidate commit.
- Open critical/high findings from [#15](https://github.com/SigmaUno/sigmaproof/issues/15) are closed or the release is no-go.
- Any release that claims authenticated anchoring must include the real Celestia evidence required by [#3](https://github.com/SigmaUno/sigmaproof/issues/3), [#4](https://github.com/SigmaUno/sigmaproof/issues/4) and [#7](https://github.com/SigmaUno/sigmaproof/issues/7). The current local package profile is explicitly unanchored.

## Local Candidate Build

Build the same archives as the tag workflow:

```sh
python3 scripts/build_release.py --version 0.1.0-preview.1 --dist-dir dist --clean --smoke-host
shasum -a 256 -c dist/SHA256SUMS
```

The builder emits four archives:

- `sigmaproof_VERSION_linux_amd64.tar.gz`
- `sigmaproof_VERSION_linux_arm64.tar.gz`
- `sigmaproof_VERSION_darwin_amd64.tar.gz`
- `sigmaproof_VERSION_darwin_arm64.tar.gz`

Each archive contains `sigmaproof`, `sigmaproofd`, `README.md`, `CHANGELOG.md`, `THIRD_PARTY.md`, `LICENSING.md` and `SECURITY.md`. The embedded command version is set from `--version`.

## Clean Machine Smoke Test

On a supported Linux or macOS host:

```sh
tar -xzf sigmaproof_0.1.0-preview.1_$(go env GOOS)_$(go env GOARCH).tar.gz
cd sigmaproof_0.1.0-preview.1_$(go env GOOS)_$(go env GOARCH)
./sigmaproof version
printf 'Synthetic SigmaProof document\n' > document.txt
./sigmaproof create --unanchored --representation original document.txt document.sigmaproof
./sigmaproof verify --offline document.txt document.sigmaproof
./sigmaproof verify document.txt document.sigmaproof; test "$?" -eq 3
```

Expected result: offline verification exits 0 for the matching document/package, and default verification exits 3 because authenticated anchors are unavailable.

## Tag Workflow

After the release owner approves the candidate commit:

```sh
git tag -a v0.1.0-preview.1 -m "SigmaProof v0.1.0 preview 1"
git push origin v0.1.0-preview.1
```

The `Preview release` workflow runs only on `v*` tags. It validates the repository, builds Linux/macOS amd64/arm64 archives, verifies `SHA256SUMS`, smoke-tests the runner-native CLI and creates a GitHub release using repository `contents: write` permission.

## Runtime Notes

- The CLI is offline only. It does not contact SigmaProof, Celestia, Paperless or any checkpoint service.
- Packages contain private witness material and should be stored with customer-controlled access.
- `sigmaproofd` without `-store` serves health/readiness only. Store-backed development ingestion is unauthenticated and must remain loopback-only.
- Provider credentials, Paperless tokens, checkpoint trust roots, backup locations and restore procedures must be documented by the deployment owner before a supported anchored preview.

## Rollback

If a tag workflow publishes incorrect artifacts, mark the GitHub release as a failed preview in the release notes and publish a new preview tag after fixing the candidate. Do not mutate checksums or replace artifacts under an already announced tag.
