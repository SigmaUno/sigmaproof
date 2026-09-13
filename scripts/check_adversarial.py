#!/usr/bin/env python3
"""Run bounded adversarial checks for the current local proof profile."""

import subprocess


COMMANDS = (
    [
        "go",
        "test",
        "./pkg/proof",
        "-run",
        "TestTamperingSeparatesAssertions|TestBoundedStrictParser|TestReaderBoundsAndFailures|TestMaximumPathFraming",
    ],
    ["go", "test", "./pkg/proof", "-run", "^$", "-fuzz", "FuzzPackage", "-fuzztime=2s"],
    [
        "go",
        "test",
        "./internal/cli",
        "-run",
        "TestCreationRefusesSymlinkAndLeavesNoPartialFile|TestUnsupportedAndMalformedPackageExits",
    ],
    ["python3", "scripts/check_verifier_boundary.py"],
)


def main() -> int:
    for command in COMMANDS:
        subprocess.run(command, check=True)
    print("OK: adversarial package, CLI and verifier-boundary checks passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
