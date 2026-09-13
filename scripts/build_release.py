#!/usr/bin/env python3
"""Build SigmaProof preview release archives."""

from __future__ import annotations

import argparse
import hashlib
import os
import platform
import shutil
import subprocess
import tarfile
import tempfile
from pathlib import Path


TARGETS = (
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("darwin", "amd64"),
    ("darwin", "arm64"),
)

COMMANDS = ("sigmaproof", "sigmaproofd")
EXTRA_FILES = ("README.md", "CHANGELOG.md", "THIRD_PARTY.md", "LICENSING.md", "SECURITY.md")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True, help="version string to embed and use in archive names")
    parser.add_argument("--dist-dir", default="dist", help="output directory")
    parser.add_argument("--clean", action="store_true", help="remove the output directory before building")
    parser.add_argument("--smoke-host", action="store_true", help="run a create/verify smoke test for the host target")
    args = parser.parse_args()

    root = Path(__file__).resolve().parents[1]
    dist = (root / args.dist_dir).resolve()
    if root not in dist.parents and dist != root:
        raise SystemExit("dist-dir must be inside the repository")
    if dist.exists():
        if not args.clean:
            raise SystemExit(f"{dist} already exists; pass --clean to replace it")
        shutil.rmtree(dist)
    dist.mkdir(parents=True)

    checksums: list[tuple[str, str]] = []
    for goos, goarch in TARGETS:
        archive = build_target(root, dist, args.version, goos, goarch)
        checksums.append((sha256(archive), archive.name))

    checksum_file = dist / "SHA256SUMS"
    checksum_file.write_text("".join(f"{digest}  {name}\n" for digest, name in checksums), encoding="utf-8")
    if args.smoke_host:
        smoke_host(dist, args.version)
    return 0


def build_target(root: Path, dist: Path, version: str, goos: str, goarch: str) -> Path:
    package_name = f"sigmaproof_{version}_{goos}_{goarch}"
    archive = dist / f"{package_name}.tar.gz"
    with tempfile.TemporaryDirectory(prefix=f"{package_name}-", dir=dist) as tmp:
        staging = Path(tmp) / package_name
        staging.mkdir()
        env = os.environ.copy()
        env.update({"GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0"})
        ldflags = f"-s -w -X github.com/SigmaUno/sigmaproof/internal/buildinfo.Version={version}"
        for command in COMMANDS:
            subprocess.run(
                [
                    "go",
                    "build",
                    "-trimpath",
                    "-ldflags",
                    ldflags,
                    "-o",
                    str(staging / command),
                    f"./cmd/{command}",
                ],
                cwd=root,
                env=env,
                check=True,
            )
        for name in EXTRA_FILES:
            shutil.copy2(root / name, staging / name)
        with tarfile.open(archive, "w:gz") as tar:
            tar.add(staging, arcname=package_name)
    return archive


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def smoke_host(dist: Path, version: str) -> None:
    goos = {"Darwin": "darwin", "Linux": "linux"}.get(platform.system())
    arch = {"x86_64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine())
    if goos is None or arch is None:
        raise SystemExit(f"unsupported smoke-test host: {platform.system()} {platform.machine()}")
    archive = dist / f"sigmaproof_{version}_{goos}_{arch}.tar.gz"
    if not archive.exists():
        raise SystemExit(f"missing host archive: {archive}")
    with tempfile.TemporaryDirectory(prefix="sigmaproof-smoke-") as tmp:
        tmp_path = Path(tmp)
        with tarfile.open(archive, "r:gz") as tar:
            safe_extract(tar, tmp_path)
        package_dir = tmp_path / f"sigmaproof_{version}_{goos}_{arch}"
        sigmaproof = package_dir / "sigmaproof"
        document = tmp_path / "document.txt"
        evidence = tmp_path / "document.sigmaproof"
        document.write_text("Synthetic SigmaProof release smoke document\n", encoding="utf-8")
        version_out = subprocess.run([str(sigmaproof), "version"], check=True, text=True, capture_output=True)
        if version_out.stdout.strip() != version:
            raise SystemExit(f"unexpected version output: {version_out.stdout!r}")
        subprocess.run(
            [str(sigmaproof), "create", "--unanchored", "--representation", "original", str(document), str(evidence)],
            check=True,
        )
        subprocess.run([str(sigmaproof), "verify", "--offline", str(document), str(evidence)], check=True)


def safe_extract(tar: tarfile.TarFile, destination: Path) -> None:
    root = destination.resolve()
    for member in tar.getmembers():
        target = (root / member.name).resolve()
        if root not in target.parents and target != root:
            raise SystemExit(f"unsafe archive path: {member.name}")
        if member.isdir():
            target.mkdir(parents=True, exist_ok=True)
            continue
        if not member.isfile():
            raise SystemExit(f"unsupported archive member: {member.name}")
        target.parent.mkdir(parents=True, exist_ok=True)
        source = tar.extractfile(member)
        if source is None:
            raise SystemExit(f"cannot extract archive member: {member.name}")
        with source, target.open("wb") as out:
            shutil.copyfileobj(source, out)
        os.chmod(target, member.mode & 0o777)


if __name__ == "__main__":
    raise SystemExit(main())
