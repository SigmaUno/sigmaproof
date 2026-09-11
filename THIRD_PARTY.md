# Third-party notices

The storage implementation introduces these pinned runtime dependencies:

| Module | Version | License notice |
| --- | --- | --- |
| go.etcd.io/bbolt | v1.5.0 | [MIT](third_party/bbolt.LICENSE) |
| golang.org/x/sys | v0.45.0 | [BSD](third_party/x-sys.LICENSE) |

Notices are copied from the pinned module distributions. Dependencies used only by upstream tests may also appear in go.sum. Release packaging must still inventory the actual distributed artifacts and include their applicable notices under #16. SigmaProof's own [license selection](LICENSING.md) remains pending.
