"""Build an Inno Setup installer from an explicit, clean distribution."""
import os
import pathlib
import re
import shutil
import subprocess

APP_ID = "com.ispace.quesora.native"


def find_iscc(explicit=None):
    candidates = [explicit, os.environ.get("ISCC_PATH"), shutil.which("ISCC.exe")]
    for variable in ("ProgramFiles(x86)", "ProgramFiles", "LOCALAPPDATA"):
        base = os.environ.get(variable)
        if base:
            for folder in ("Inno Setup 6", "Inno Setup 7", "Programs/Inno Setup 6", "Programs/Inno Setup 7"):
                candidates.append(str(pathlib.Path(base) / folder / "ISCC.exe"))
    for name in candidates:
        if name and pathlib.Path(name).is_file():
            return pathlib.Path(name).resolve()
    raise RuntimeError("Inno Setup compiler not found. Install Inno Setup 6.7.1+ or set ISCC_PATH.")


def numeric_version(version):
    match = re.fullmatch(r"(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?", version)
    if not match:
        raise ValueError("Invalid installer version")
    numbers = [int(match[i]) for i in (1, 2, 3)]
    suffix = match[4]
    revision = 65535 if not suffix else 1
    if suffix and suffix.rsplit(".", 1)[-1].isdigit():
        revision = int(suffix.rsplit(".", 1)[-1])
    numbers.append(revision)
    if any(n < 0 or n > 65535 for n in numbers):
        raise ValueError("Installer version component outside Windows version range")
    return ".".join(map(str, numbers))


def build_installer(root, distribution, version, icon, output, iscc=None):
    compiler = find_iscc(iscc)
    args = [str(compiler), "/Qp", "/DAppVersion=" + version,
            "/DNumericVersion=" + numeric_version(version),
            "/DDistDir=" + str(pathlib.Path(distribution).resolve()),
            "/DOutputDir=" + str(pathlib.Path(output).resolve()),
            "/DIconFile=" + str(pathlib.Path(icon).resolve()), str(root / "packaging" / "quesora.iss")]
    subprocess.run(args, cwd=root, check=True)
    setup = pathlib.Path(output) / ("QuesoraNative-" + version + "-setup-windows-x64.exe")
    if not setup.is_file():
        raise RuntimeError("Installer did not produce the expected output")
    return setup
