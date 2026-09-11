#!/usr/bin/env python3
"""Generate synthetic unanchored package fixtures using Python only."""
import hashlib
import json
import struct
import sys
from pathlib import Path


def tree(entries, target):
    level = [(hashlib.sha256(b'\x00' + c).digest(), {i}) for i, c in enumerate(entries)]
    path = []
    while len(level) > 1:
        following = []
        for i in range(0, len(level), 2):
            left, ids = level[i]
            if i + 1 == len(level):
                following.append((left, ids))
                continue
            right, others = level[i + 1]
            if target in ids:
                path.append(right)
            elif target in others:
                path.append(left)
            following.append((hashlib.sha256(b'\x01' + left + right).digest(), ids | others))
        level = following
    return level[0][0], path


vectors = []
for size in [1, 3, 5, 8]:
    docs = [b'' if i == 0 else b'SigmaProof synthetic document ' + str(i).encode() for i in range(size)]
    witnesses = [struct.pack('>22sBBBB32s32s', b'sigmaproof:commitment\x00', 1, 1, 1 + i % 2, 0,
                            hashlib.sha256(doc).digest(), hashlib.sha256(b'test nonce ' + bytes([i])).digest())
                 for i, doc in enumerate(docs)]
    commitments = [hashlib.sha256(w).digest() for w in witnesses]
    for index in range(size):
        root, path = tree(commitments, index)
        manifest = struct.pack('>17sBBBBQ32s', b'sigmaproof:batch\x00', 1, 1, 1, 1, size, root)
        encoded = b'SIGMAPRF\x01' + witnesses[index] + manifest + struct.pack('>QB', index, len(path)) + b''.join(path) + b'\x00'
        assert len(encoded) == 170 + 32 * len(path)
        vectors.append(dict(name=f'batch-{size}-index-{index}', document=docs[index].hex(), package=encoded.hex()))
target = Path(__file__).resolve().parents[1] / 'pkg/proof/testdata/unanchored-v1.json'
output = json.dumps(vectors, indent=2) + '\n'
if sys.argv[1:] == ['--check']:
    if not target.exists() or target.read_text() != output:
        raise SystemExit(f'Stale vectors: regenerate with {Path(__file__).name}')
elif not sys.argv[1:]:
    target.write_text(output)
else:
    raise SystemExit('Usage: ' + Path(__file__).name + ' [--check]')
