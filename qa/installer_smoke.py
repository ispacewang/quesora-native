"""CI-only install/upgrade/uninstall regression; never launch the application UI."""
from contextlib import closing
import hashlib
import json
import os
import pathlib
import sqlite3
import subprocess
import tempfile
import time
import winreg
import zipfile
from installer import APP_ID, build_installer, find_iscc

ROOT = pathlib.Path(__file__).resolve().parents[1]
UNINSTALL_KEY = "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\" + APP_ID + "_is1"


def installed_values():
    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, UNINSTALL_KEY, 0, winreg.KEY_READ | winreg.KEY_WOW64_64KEY) as key:
            return {name: winreg.QueryValueEx(key, name)[0] for name in ("InstallLocation", "DisplayVersion")}
    except FileNotFoundError:
        return None


def run_setup(setup, *args, check=True):
    return subprocess.run([str(setup), "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/NOCLOSEAPPLICATIONS", *args], check=check, timeout=120)


def digest(filename):
    return hashlib.sha256(pathlib.Path(filename).read_bytes()).hexdigest()




def read_shell_link(filename):
    """Read Unicode Shell-link fields without WScript.Shell's ANSI path conversion."""
    import ctypes
    import uuid
    from ctypes import wintypes

    class GUID(ctypes.Structure):
        _fields_ = [("data", ctypes.c_ubyte * 16)]

    def guid(text):
        return GUID.from_buffer_copy(uuid.UUID(text).bytes_le)

    def invoke(pointer, index, argtypes, *args):
        table = ctypes.cast(pointer, ctypes.POINTER(ctypes.POINTER(ctypes.c_void_p))).contents
        method = ctypes.WINFUNCTYPE(ctypes.c_long, ctypes.c_void_p, *argtypes)(table[index])
        hr = method(pointer, *args)
        if hr < 0:
            raise OSError("Shell-link read failed: HRESULT 0x%08x" % (hr & 0xffffffff))

    ole = ctypes.OleDLL("ole32")
    ole.CoInitializeEx(None, 2)
    link, persist = ctypes.c_void_p(), ctypes.c_void_p()
    try:
        clsid = guid("00021401-0000-0000-c000-000000000046")
        iid = guid("000214f9-0000-0000-c000-000000000046")  # IShellLinkW
        ole.CoCreateInstance(ctypes.byref(clsid), None, 1, ctypes.byref(iid), ctypes.byref(link))
        persist_iid = guid("0000010b-0000-0000-c000-000000000046")
        invoke(link, 0, [ctypes.c_void_p, ctypes.c_void_p], ctypes.byref(persist_iid), ctypes.byref(persist))
        invoke(persist, 5, [wintypes.LPCWSTR, wintypes.DWORD], str(filename), 0)
        target, icon = ctypes.create_unicode_buffer(32768), ctypes.create_unicode_buffer(32768)
        index = ctypes.c_int()
        invoke(link, 3, [wintypes.LPWSTR, ctypes.c_int, ctypes.c_void_p, wintypes.DWORD], target, len(target), None, 4)
        invoke(link, 16, [wintypes.LPWSTR, ctypes.c_int, ctypes.POINTER(ctypes.c_int)], icon, len(icon), ctypes.byref(index))
        return {"target": target.value, "icon": icon.value + "," + str(index.value)}
    finally:
        for pointer in (persist, link):
            if pointer.value:
                table = ctypes.cast(pointer, ctypes.POINTER(ctypes.POINTER(ctypes.c_void_p))).contents
                ctypes.WINFUNCTYPE(ctypes.c_ulong, ctypes.c_void_p)(table[2])(pointer)
        ole.CoUninitialize()


def shell_folder(identifier):
    """Respect redirected Desktop/Programs folders instead of assuming USERPROFILE paths."""
    import ctypes
    folder = ctypes.create_unicode_buffer(260)
    result = ctypes.WinDLL("shell32").SHGetFolderPathW(None, identifier, None, 0, folder)
    if result != 0:
        raise OSError("Could not locate CI shortcut folder")
    return pathlib.Path(folder.value)


def verify_shortcut_icons(installed, version, icon):
    """Inspect real installed Shell links only on the ephemeral runner guarded by main()."""
    expected_icon = installed / "icons" / ("quesora-" + version + ".ico")
    if digest(expected_icon) != digest(icon):
        raise AssertionError("Installed standalone icon differs from the reviewed full artwork")
    for folder in (shell_folder(0x10), shell_folder(0x02)):
        filename = folder / "Quesora Native.lnk"
        if not filename.is_file():
            raise AssertionError("Installed shortcut missing: " + str(filename))
        link = read_shell_link(filename)
        icon_path, index = link["icon"].rsplit(",", 1)
        if pathlib.Path(link["target"]).resolve() != (installed / "QuesoraNativePreview.exe").resolve():
            raise AssertionError("Shortcut target mismatch: " + repr(link))
        if pathlib.Path(icon_path).resolve() != expected_icon.resolve() or index != "0":
            raise AssertionError("Shortcut icon mismatch: " + repr(link))

def main():
    if os.environ.get("GITHUB_ACTIONS") != "true" or os.name != "nt":
        raise SystemExit("This test installs an application and is restricted to ephemeral GitHub Windows runners.")
    if installed_values() is not None:
        raise RuntimeError("Refusing to overwrite an existing Quesora Native installation")
    data = pathlib.Path(os.environ["LOCALAPPDATA"]) / "QuesoraNative" / "data"
    if data.exists():
        raise RuntimeError("Refusing to touch an existing Quesora Native data directory")
    report = json.loads((ROOT / "build/release-report.json").read_text(encoding="utf-8"))
    release = ROOT / "build/release"
    setup = release / next(a["name"] for a in report["artifacts"] if "-setup-" in a["name"])
    archive = release / next(a["name"] for a in report["artifacts"] if a["name"].endswith(".zip"))
    with tempfile.TemporaryDirectory(prefix="quesora-install-test-", dir=os.environ.get("RUNNER_TEMP"), ignore_cleanup_errors=True) as directory:
        temp = pathlib.Path(directory)
        distribution, baseline_output = temp / "distribution", temp / "baseline"
        baseline_output.mkdir()
        with zipfile.ZipFile(archive) as source:
            source.extractall(distribution)
        icon, resource = temp / "icon.ico", temp / "resources.syso"
        subprocess.run(["go", "run", ".", str(ROOT / "resources/icon-frames"), str(resource), str(icon)], cwd=ROOT / "qa/winres", check=True)
        baseline = build_installer(ROOT, distribution, "0.0.0-ci.1", icon, baseline_output, find_iscc())
        custom = temp / "自定义位置 with spaces" / "Quesora Native"
        db = data / "questions.db"
        try:
            run_setup(baseline, "/DIR=" + str(custom), "/TASKS=desktopicon")
            verify_shortcut_icons(custom, "0.0.0-ci.1", icon)
            state = installed_values()
            if not state or pathlib.Path(state["InstallLocation"]).resolve() != custom.resolve():
                raise AssertionError("Initial custom install path was not registered")
            executable = custom / "QuesoraNativePreview.exe"
            executable.write_bytes(b"old executable must be replaced during upgrade")
            data.mkdir(parents=True)
            with closing(sqlite3.connect(db)) as conn:
                conn.execute("CREATE TABLE upgrade_fixture(value TEXT)")
                conn.execute("INSERT INTO upgrade_fixture VALUES('preserve study records')")
                conn.commit()
            before = digest(db)
            # Deliberately omit /DIR: the upgrade must discover the previous path.
            run_setup(setup)
            state = installed_values()
            if not state or pathlib.Path(state["InstallLocation"]).resolve() != custom.resolve():
                raise AssertionError("Upgrade forgot the previous installation directory")
            if state["DisplayVersion"] != report["version"]:
                raise AssertionError("Upgrade did not update its registered version")
            if digest(executable) != digest(distribution / "QuesoraNativePreview.exe"):
                raise AssertionError("Old executable was not overwritten")
            if not (custom / ".quesora-native-installed").is_file():
                raise AssertionError("Installed-data marker missing")
            if digest(db) != before:
                raise AssertionError("Upgrade modified user data")
            verify_shortcut_icons(custom, report["version"], icon)
            # Reinstalling the same version must remain a single installation.
            run_setup(setup)
            verify_shortcut_icons(custom, report["version"], icon)
            if digest(db) != before or digest(executable) != digest(distribution / "QuesoraNativePreview.exe"):
                raise AssertionError("Same-version reinstall changed data or program")
            if run_setup(baseline, check=False).returncode == 0:
                raise AssertionError("Older installer was allowed to downgrade the installation")
            if installed_values()["DisplayVersion"] != report["version"]:
                raise AssertionError("Rejected downgrade changed the installed version")
            subprocess.run([str(custom / "unins000.exe"), "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART"], check=True, timeout=120)
            for _ in range(40):
                if not executable.exists():
                    break
                time.sleep(0.25)
            if executable.exists() or installed_values() is not None:
                raise AssertionError("Uninstaller left the application registered or installed")
            if digest(db) != before:
                raise AssertionError("Uninstaller removed or changed user data")
            print("Installer regression passed: custom path, remembered upgrade path, executable replacement, reinstall, downgrade guard, data retention, versioned desktop/start-menu icon paths")
        finally:
            if installed_values() is not None and (custom / "unins000.exe").is_file():
                subprocess.run([str(custom / "unins000.exe"), "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART"], timeout=120)
            # Only remove the fixture created above, after asserting uninstall retained it.
            if db.exists():
                db.unlink()
            for folder in (data, data.parent):
                try:
                    folder.rmdir()
                except OSError:
                    pass


if __name__ == "__main__":
    main()
