"""Audit tracked publication files without printing any possible secret values."""
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
ROOT_FILES = {"go.mod", "go.sum", "mygo.json", "README.md", "LICENSE", "NOTICE", "CHANGELOG.md", ".gitignore", ".gitattributes", "build-preview.py"}
QA_FILES = {"qa/audit_repository.py", "qa/verify_executable.py", "qa/winres/main.go", "qa/winres/go.mod", "qa/winres/go.sum"}
OTHER_FILES = {".github/workflows/windows.yml", "docs/usage.md", "docs/images/preview-light.png", "testdata/legacy.xls"}
SECRET_PATTERNS = [
    re.compile(rb"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----"),
    re.compile(rb"(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,}|sk-[A-Za-z0-9_-]{30,})"),
    re.compile(rb"AKIA[0-9A-Z]{16}"),
]


def allowed(name):
    p = pathlib.PurePosixPath(name)
    if name in ROOT_FILES | QA_FILES | OTHER_FILES:
        return True
    if len(p.parts) == 1 and p.suffix == ".go":
        return True
    if name.startswith("resources/"):
        return p.suffix in {".png", ".json"} or name in {"resources/fonts/OFL.txt", "resources/fonts/LICENSE.txt"} or (name.startswith("resources/fonts/noto-") and name.endswith(".ttf.gz"))
    if name.startswith("third_party/xls/") and len(p.parts) == 3:
        return p.suffix in {".go", ".md"} or p.name in {"LICENSE", "go.mod", "go.sum"}
    return False


def main():
    data = subprocess.check_output(["git", "ls-files", "--stage", "-z"], cwd=ROOT)
    violations, count, total = [], 0, 0
    for entry in data.split(b"\0"):
        if not entry:
            continue
        metadata, raw_name = entry.split(b"\t", 1)
        name = raw_name.decode("utf-8")
        mode = metadata.split(b" ", 1)[0]
        count += 1
        if mode not in {b"100644", b"100755"} or not allowed(name):
            violations.append(name + ": outside the publication allowlist")
            continue
        content = (ROOT / name).read_bytes()
        total += len(content)
        if len(content) > 10 * 1024 * 1024:
            violations.append(name + ": file exceeds 10 MiB")
        if any(pattern.search(content) for pattern in SECRET_PATTERNS):
            violations.append(name + ": possible credential; inspect locally")
    if not count:
        violations.append("No tracked files found")
    if violations:
        print("Publication audit failed:\n" + "\n".join(violations))
        return 1
    print(f"Publication audit passed: {count} tracked files, {total / 1048576:.2f} MiB; no disallowed files or known credential patterns")
    return 0


if __name__ == "__main__":
    sys.exit(main())
