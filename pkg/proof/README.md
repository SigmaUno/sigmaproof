# Independent proof library

The [Merkle subpackage](merkle/README.md) implements tree roots and inclusion paths. The [commitment subpackage](commitment/README.md) implements the experimental SEP-2 exact-byte document commitment profile with private random nonces.

Full defensive envelope decoding and anchor authentication remain unimplemented. The library works without service/database dependencies. Cross-language fixtures cover both primitives; external review is still needed before interoperability is claimed.
