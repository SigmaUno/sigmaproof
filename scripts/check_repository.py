#!/usr/bin/env python3
"""Dependency-free checks for repository documentation and JSON examples."""
import json
import re
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[1]
errors = []
markdown = sorted(ROOT.rglob('*.md'))
for path in markdown:
    if '.git' in path.parts:
        continue
    content = path.read_text(encoding='utf-8')
    if not content.strip():
        errors.append(f'{path.relative_to(ROOT)}: empty document')
    for target in re.findall(r'\[[^\]]*\]\(([^)]+)\)', content):
        target = target.split(' "', 1)[0].strip('<>')
        parsed = urlsplit(target)
        if parsed.scheme or target.startswith('#'):
            continue
        resolved = (path.parent / unquote(parsed.path)).resolve()
        if not resolved.exists():
            errors.append(f'{path.relative_to(ROOT)}: broken local link {target}')
for path in sorted((ROOT / 'examples').rglob('*.json')):
    try:
        json.loads(path.read_text(encoding='utf-8'))
    except (ValueError, UnicodeError) as error:
        errors.append(f'{path.relative_to(ROOT)}: {error}')
for required in ['README.md', 'CONTRIBUTING.md', 'SECURITY.md', 'GOVERNANCE.md', 'ROADMAP.md']:
    if not (ROOT / required).is_file():
        errors.append(f'Missing {required}')
if errors:
    print('\n'.join(errors), file=sys.stderr)
    sys.exit(1)
print(f'OK: local links in {len(markdown)} Markdown documents and JSON examples')
