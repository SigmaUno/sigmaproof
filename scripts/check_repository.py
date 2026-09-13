#!/usr/bin/env python3
"""Repository documentation, example and optional OpenAPI checks."""
import json
import re
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit

try:
    import yaml
except ImportError:
    yaml = None

ROOT = Path(__file__).resolve().parents[1]
errors = []
openapi_checked = 0
markdown = sorted(ROOT.rglob('*.md'))

def walk(value):
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from walk(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk(child)

def has_json_pointer(document, pointer):
    if not pointer.startswith('#/'):
        return False
    current = document
    for part in pointer[2:].split('/'):
        part = part.replace('~1', '/').replace('~0', '~')
        if not isinstance(current, dict) or part not in current:
            return False
        current = current[part]
    return True

def documented_operations(document):
    result = set()
    for route, operations in document.get('paths', {}).items():
        if not isinstance(operations, dict):
            continue
        for method in operations:
            if method.lower() in {'get', 'post', 'put', 'patch', 'delete', 'head', 'options'}:
                result.add((method.upper(), route))
    return result

def implemented_dev_routes():
    server = (ROOT / 'internal' / 'server' / 'server.go').read_text(encoding='utf-8')
    allowed = set()
    for route, method in re.findall(r'mux\.HandleFunc\("([^"]+)", requireMethod\(http\.Method([A-Za-z]+),', server):
        allowed.add((method.upper(), route))
    return allowed

def implemented_error_codes():
    server = (ROOT / 'internal' / 'server' / 'server.go').read_text(encoding='utf-8')
    return set(re.findall(r'errorJSON\([^,]+,\s*[^,]+,\s*"([^"]+)"', server))

def documented_error_codes(document):
    try:
        codes = document['components']['schemas']['Error']['properties']['error']['enum']
    except (KeyError, TypeError):
        return set()
    return set(codes) if isinstance(codes, list) else set()
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
if yaml is not None:
    implemented_routes = implemented_dev_routes()
    implemented_errors = implemented_error_codes()
    for path in sorted(ROOT.rglob('*.openapi.yaml')):
        openapi_checked += 1
        try:
            document = yaml.safe_load(path.read_text(encoding='utf-8'))
        except (yaml.YAMLError, UnicodeError) as error:
            errors.append(f'{path.relative_to(ROOT)}: {error}')
            continue
        if not isinstance(document, dict) or not document.get('openapi') or not isinstance(document.get('paths'), dict):
            errors.append(f'{path.relative_to(ROOT)}: invalid OpenAPI document')
            continue
        for node in walk(document):
            ref = node.get('$ref')
            if isinstance(ref, str) and not has_json_pointer(document, ref):
                errors.append(f'{path.relative_to(ROOT)}: unresolved OpenAPI ref {ref}')
        if path.name == 'dev-api.openapi.yaml':
            documented_routes = documented_operations(document)
            if documented_routes != implemented_routes:
                missing = sorted(implemented_routes - documented_routes)
                extra = sorted(documented_routes - implemented_routes)
                if missing:
                    errors.append(f'{path.relative_to(ROOT)}: missing implemented routes {missing}')
                if extra:
                    errors.append(f'{path.relative_to(ROOT)}: documents unimplemented routes {extra}')
            documented_errors = documented_error_codes(document)
            if documented_errors != implemented_errors:
                missing = sorted(implemented_errors - documented_errors)
                extra = sorted(documented_errors - implemented_errors)
                if missing:
                    errors.append(f'{path.relative_to(ROOT)}: missing implemented error codes {missing}')
                if extra:
                    errors.append(f'{path.relative_to(ROOT)}: documents unused error codes {extra}')
for required in ['README.md', 'CONTRIBUTING.md', 'SECURITY.md', 'GOVERNANCE.md', 'ROADMAP.md']:
    if not (ROOT / required).is_file():
        errors.append(f'Missing {required}')
if errors:
    print('\n'.join(errors), file=sys.stderr)
    sys.exit(1)
suffix = ''
if openapi_checked:
    noun = 'document' if openapi_checked == 1 else 'documents'
    suffix = f' and {openapi_checked} OpenAPI {noun}'
elif list(ROOT.rglob('*.openapi.yaml')):
    suffix = ' (OpenAPI validation skipped; PyYAML unavailable)'
print(f'OK: local links in {len(markdown)} Markdown documents, JSON examples{suffix}')
