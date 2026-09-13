# Adversarial checks

Status: bounded release-gate evidence for the current experimental local proof profile; not a complete security audit.

Run:

```sh
make adversarial-check
```

This command exercises:

- package tampering cases that separate document, inclusion and anchor assertions;
- strict binary package bounds, malformed framing, invalid path lengths and unsupported profile fields;
- the `FuzzPackage` parser target for a short bounded fuzz window suitable for CI;
- CLI refusal to overwrite a destination symlink or leave a partial package after failed creation;
- CLI exit handling for malformed versus unsupported packages;
- the import-graph guard that keeps the offline verifier free of service, network and helper-process dependencies.

The current package profile is fixed-width binary, so JSON duplicate-key handling and path traversal inside package attachments are not runtime surfaces yet. If a future envelope adds structured metadata, attachments or archive extraction, this gate must grow before that feature can ship.

Passing this gate does not prove provider authenticity, regulatory qualification, Paperless event authenticity, tenant authentication or Celestia checkpoint trust. Those remain separate release blockers tracked in the MVP issues.
