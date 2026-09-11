#!/usr/bin/env python3
"""Keep offline verification independent of services and network/helper clients.

This is an import-graph guard, not a runtime syscall audit. Caller-supplied Go
readers are outside this boundary; the CLI opens only explicitly supplied files.
"""
import subprocess

module = 'github.com/SigmaUno/sigmaproof'
for target in ['./pkg/proof/...', './cmd/sigmaproof']:
    dependencies = subprocess.check_output(['go', 'list', '-deps', target], text=True).splitlines()
    forbidden = [name for name in dependencies
                 if name == 'net' or name.startswith('net/') or name == 'os/exec'
                 or (target == './pkg/proof/...' and name.startswith(module + '/internal/'))]
    if forbidden:
        raise SystemExit(f'{target}: forbidden verifier dependencies: {forbidden}')
print('OK: proof library is service-independent; verifier has no network/helper imports')
