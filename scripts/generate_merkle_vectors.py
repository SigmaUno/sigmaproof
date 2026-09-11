#!/usr/bin/env python3
"""Reproduce RFC 6962 tree fixtures using an iterative Python implementation."""
import hashlib
import json
import sys
from pathlib import Path


def tree(entries, target):
    level = [(hashlib.sha256(b'\x00' + x).digest(), {i}) for i, x in enumerate(entries)]
    path = []
    if not level:
        return hashlib.sha256(b'').hexdigest(), path
    while len(level) > 1:
        next_level = []
        for i in range(0, len(level), 2):
            left, indices = level[i]
            if i + 1 == len(level):
                next_level.append((left, indices))
                continue
            right, others = level[i + 1]
            if target in indices:
                path.append(right.hex())
            elif target in others:
                path.append(left.hex())
            next_level.append((hashlib.sha256(b'\x01' + left + right).digest(), indices | others))
        level = next_level
    return level[0][0].hex(), path


vectors = []
for size in [0, 1, 2, 3, 5, 7, 8, 9, 16, 17]:
    entries = [bytes([i]) * i for i in range(size)]
    for index in range(max(1, size)):
        root, path = tree(entries, index)
        vectors.append(dict(entries=[x.hex() for x in entries], index=index, root=root, path=path))
target = Path(__file__).resolve().parents[1] / 'pkg/proof/merkle/testdata/rfc6962.json'
output = json.dumps(vectors, indent=2) + '\n'
if sys.argv[1:] == ['--check']:
    if not target.exists() or target.read_text() != output:
        raise SystemExit(f'Stale vectors: regenerate with {Path(__file__).name}')
elif not sys.argv[1:]:
    target.write_text(output)
else:
    raise SystemExit('Usage: ' + Path(__file__).name + ' [--check]')
