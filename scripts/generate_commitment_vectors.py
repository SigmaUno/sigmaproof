#!/usr/bin/env python3
"""Generate synthetic vectors for the proposed SEP-2 fixed-width profile.

Fixed nonces are for reproducible tests only. Production uses a CSPRNG.
"""
import hashlib
import json
import sys
import struct
from pathlib import Path

vectors = []
for name, document, representation, nonce in [
    ('empty-original', b'', 1, bytes(range(32))),
    ('original', b'SigmaProof\n', 1, bytes(range(32))),
    ('changed-byte', b'SigmaProof!', 1, bytes(range(32))),
    ('changed-representation', b'SigmaProof\n', 2, bytes(range(32))),
    ('changed-nonce', b'SigmaProof\n', 1, bytes(reversed(range(32)))),
    ('binary-original', bytes(range(256)), 1, b'\xa5' * 32),
]:
    digest = hashlib.sha256(document).digest()
    # Struct layout specifies fixed widths independently of Go's append encoder.
    preimage = struct.pack('>22sBBBB32s32s', b'sigmaproof:commitment\x00',
                           1, 1, representation, 0, digest, nonce)
    assert len(preimage) == 90
    vectors.append(dict(name=name, document=document.hex(), representation=representation,
                        nonce=nonce.hex(), document_digest=digest.hex(),
                        preimage=preimage.hex(), commitment=hashlib.sha256(preimage).hexdigest()))
target = Path(__file__).resolve().parents[1] / 'pkg/proof/commitment/testdata/sep2-draft.json'
output = json.dumps(vectors, indent=2) + '\n'
if sys.argv[1:] == ['--check']:
    if not target.exists() or target.read_text() != output:
        raise SystemExit(f'Stale vectors: regenerate with {Path(__file__).name}')
elif not sys.argv[1:]:
    target.write_text(output)
else:
    raise SystemExit('Usage: ' + Path(__file__).name + ' [--check]')
