# Independent proof library

The [Merkle subpackage](merkle/README.md) implements tree roots and inclusion paths. The [commitment subpackage](commitment/README.md) implements the experimental SEP-2 exact-byte document commitment profile with private random nonces.

The [batch subpackage](batch/README.md) constructs and strictly decodes the experimental public manifest that binds count, algorithm versions and root.

Full defensive envelope decoding and anchor authentication remain unimplemented. The library works without service/database dependencies. Cross-language fixtures cover these primitives; external review is still needed before interoperability is claimed.
