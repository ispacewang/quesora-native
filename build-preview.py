"""Build and verify the Windows x64 native preview without local runtime data."""
import argparse
import hashlib
import json
import os
import pathlib
import platform
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

ROOT = pathlib.Path(__file__).resolve().parent
VERSION_RE = r"[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?"
SCENES = ["history", "empty", "unselected", "multi", "judge", "fill", "short", "selected", "correct", "wrong", "important", "exam", "exam-result", "exam-setup", "exam-setup-empty", "many-banks"]


def build_version(value):
    if not value:
        value = re.search(r'appVersion\s*=\s*"([^"]+)"', (ROOT / "version.go").read_text(encoding="utf-8")).group(1)
    value = value.removeprefix("v")
    if not re.fullmatch(VERSION_RE, value):
        raise ValueError("Expected a version such as 0.1.0 or 0.1.0-preview.3")
    return value


def run(args, cwd, env, capture=False):
    print(" ".join(map(str, args)), flush=True)
    return subprocess.run(list(map(str, args)), cwd=cwd, env=env, check=True,
                          stdout=subprocess.PIPE if capture else None, text=capture, encoding="utf-8" if capture else None)


def module_licenses(module_root, target, env):
    data = run(["go", "list", "-m", "-json", "all"], module_root, env, capture=True).stdout
    decoder = json.JSONDecoder()
    while data.strip():
        data = data.lstrip()
        module, pos = decoder.raw_decode(data)
        data = data[pos:]
        actual = module.get("Replace", module)
        directory = actual.get("Dir")
        if not directory or module.get("Main"):
            continue
        dest = target / module["Path"].replace("/", "_")
        for item in pathlib.Path(directory).iterdir():
            if item.is_file() and item.name.upper().startswith(("LICENSE", "COPYING", "NOTICE")):
                dest.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(item, dest / item.name)


def build(stage, distribution, version, env):
    for source in ROOT.glob("*.go"):
        shutil.copy2(source, stage / source.name)
    for name in ("go.mod", "go.sum"):
        shutil.copy2(ROOT / name, stage / name)
    for name in ("testdata", "third_party", "resources"):
        shutil.copytree(ROOT / name, stage / name)
    gofiles = sorted(stage.glob("*.go"))
    unformatted = run(["gofmt", "-l", *gofiles], stage, env, capture=True).stdout.strip()
    if unformatted:
        raise RuntimeError("Run gofmt on:\n" + unformatted)
    run(["go", "mod", "download"], stage, env)
    run(["go", "mod", "verify"], stage, env)
    run(["go", "test", "-count=1", "./..."], stage, env)
    run(["go", "tool", "mygo", "vet", "."], stage, env)
    resource_module = ROOT / "qa" / "winres"
    run(["go", "mod", "download"], resource_module, env)
    run(["go", "mod", "verify"], resource_module, env)
    run(["go", "run", ".", ROOT / "resources" / "icon.png", stage / "quesora_windows_amd64.syso"], resource_module, env)
    out = ROOT / "build"
    out.mkdir(exist_ok=True)
    executable = out / "QuesoraNativePreview.exe"
    flags = "-s -w -H=windowsgui -X main.appVersion=" + version
    run(["go", "build", "-trimpath", "-ldflags", flags, "-o", executable, "."], stage, env)
    run([sys.executable, ROOT / "qa" / "verify_executable.py", executable], ROOT, env)
    scenes = [("light", []), ("dark", ["-dark"]), ("upload", ["-show-upload"]),
              ("exam-setup-dark", ["-scene", "exam-setup", "-dark"]),
              ("many-banks-dark", ["-scene", "many-banks", "-dark"]),
              ("history-dark", ["-scene", "history", "-dark"])]
    scenes += [(name, ["-scene", name]) for name in SCENES]
    for name, args in scenes:
        run([executable, "-snapshot", out / ("preview-" + name + ".png"), *args], stage, env)
    shutil.copy2(executable, distribution / executable.name)
    for name in ("README.md", "LICENSE", "NOTICE", "CHANGELOG.md"):
        shutil.copyfile(ROOT / name, distribution / name)
    shutil.copytree(ROOT / "docs", distribution / "docs")
    licenses = distribution / "licenses"
    licenses.mkdir()
    for module_root in (stage, resource_module):
        module_licenses(module_root, licenses, env)
    font_license = licenses / "Noto-Sans-SC"
    font_license.mkdir()
    for name in ("LICENSE.txt", "OFL.txt"):
        shutil.copyfile(ROOT / "resources" / "fonts" / name, font_license / name)
    release = out / "release"
    release.mkdir(exist_ok=True)
    prefix = "QuesoraNative-" + version
    release_exe = release / (prefix + "-windows-x64.exe")
    release_zip = release / (prefix + "-windows-x64.zip")
    shutil.copy2(executable, release_exe)
    with zipfile.ZipFile(release_zip, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for item in sorted(distribution.rglob("*")):
            if item.is_file():
                archive.write(item, item.relative_to(distribution))
    # Only the freshly assembled distribution is archived; no local DB/caches.
    with zipfile.ZipFile(release_zip) as archive:
        for name in archive.namelist():
            if name.lower().endswith((".db", ".sqlite", ".log", ".pem", ".key")) or name.startswith(("data/", ".")):
                raise RuntimeError("Unexpected archive member: " + name)
    shutil.copy2(release_zip, out / "QuesoraNativePreview-windows-x64.zip")
    checksum = release / (prefix + "-SHA256SUMS.txt")
    checksum.write_text("".join(hashlib.sha256(p.read_bytes()).hexdigest() + "  " + p.name + "\n" for p in (release_exe, release_zip)), encoding="utf-8", newline="\n")
    report = {"version": version, "artifacts": [{"name": p.name, "bytes": p.stat().st_size} for p in (release_exe, release_zip, checksum)]}
    (out / "release-report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
            output.write("version=" + version + "\n")
    for p in (release_exe, release_zip):
        print(p.name, round(p.stat().st_size / 1048576, 2), "MiB", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", help="Override the source version (v prefix accepted)")
    args = parser.parse_args()
    if platform.system() != "Windows":
        parser.error("Build on Windows x64; this script also runs the produced EXE for snapshot checks")
    tag = os.environ.get("GITHUB_REF_NAME") if os.environ.get("GITHUB_REF_TYPE") == "tag" else None
    version = build_version(args.version or tag)
    env = os.environ.copy()
    env.update({"CGO_ENABLED": "0", "GOOS": "windows", "GOARCH": "amd64"})
    cache = ROOT / ".preview-build"
    cache.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="release-", dir=cache) as directory:
        workspace = pathlib.Path(directory)
        stage, distribution = workspace / "source", workspace / "distribution"
        stage.mkdir()
        distribution.mkdir()
        build(stage, distribution, version, env)


if __name__ == "__main__":
    main()
