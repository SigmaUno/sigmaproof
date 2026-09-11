#!/usr/bin/env python3
"""Generate synthetic fixed-width SEP-5 manifests without using Go code."""
import hashlib
import json
import struct
import sys
from pathlib import Path


def root(entries):
    nodes = [hashlib.sha256(b'\x00' + entry).digest() for entry in entries]
    while len(nodes) > 1:
        nodes = [hashlib.sha256(b'\x01' + nodes[i] + nodes[i + 1]).digest()
                 if i + 1 < len(nodes) else nodes[i]
                 for i in range(0, len(nodes), 2)]
    return nodes[0]


vectors = []
for size in [1, 2, 3, 5, 8, 9]:
    # Synthetic opaque commitment values: not real document evidence.
    entries = [hashlib.sha256(b'synthetic commitment:' + bytes([i])).digest()
               for i in range(size)]
    for name, ordered in [('ordered', entries), ('reversed', list(reversed(entries)))]:
        tree_root = root(ordered)
        domain = b'sigmaproof:batch\x00'
        assert len(domain) == 17
        encoded = struct.pack('>17sBBBBQ32s', domain, 1, 1, 1, 1, size, tree_root)
        assert len(encoded) == 61
        vectors.append(dict(name=f'{name}-{size}', commitments=[x.hex() for x in ordered],
                            root=tree_root.hex(), encoded=encoded.hex(),
                            digest=hashlib.sha256(encoded).hexdigest()))
target = Path(__file__).resolve().parents[1] / 'pkg/proof/batch/testdata/sep5-draft.json'
output = json.dumps(vectors, indent=2) + '\n'
if sys.argv[1:] == ['--check']:
    if not target.exists() or target.read_text() != output:
        raise SystemExit(f'Stale vectors: regenerate with {Path(__file__).name}')
elif not sys.argv[1:]:
    target.write_text(output)
else:
    raise SystemExit('Usage: ' + Path(__file__).name + ' [--check]')
