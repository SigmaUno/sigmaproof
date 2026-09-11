# CLI skeleton

Build with `make build`, then run `./bin/sigmaproof version` or `./bin/sigmaproof help`. Help/version exit 0; invalid commands exit 2; `verify` always exits 3 with an unsupported message, without reading files or accessing the network. Real verification belongs to #5 after reviewed format decisions. See the [verification model](../../docs/architecture/verification.md).
