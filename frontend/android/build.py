"""Build the ARM64 compressed Release APK with Alisten's Kotlin playback host.

Requires a JDK, Android SDK 35+, Kotlin (KOTLIN_HOME or PATH), and the NSH CLI.
Only generated files under .nsh/android-shell and .nsh/out are written.
"""
from pathlib import Path
import argparse
import json
import os
import shutil
import subprocess
import tempfile
import zipfile

FRONTEND = Path(__file__).resolve().parent.parent
PROJECT = FRONTEND.parent
ANDROID = FRONTEND / "android"


def run(*args, env=None):
    subprocess.run([str(arg) for arg in args], cwd=FRONTEND, env=env, check=True)


def executable(name, fallback=None):
    found = shutil.which(name)
    if found:
        return Path(found).resolve()
    if fallback and fallback.is_file():
        return fallback.resolve()
    raise RuntimeError(f"Missing {name}; configure PATH or the documented environment variable")


def main():
    argparse.ArgumentParser(description=__doc__).parse_args()
    nsh = executable("nsh", PROJECT / ".tmp/nsh-toolchain/nsh/bin/nsh.exe")
    runtime = Path(os.environ.get("NSH_RUNTIME_HOME", str(nsh.parent.parent)))
    sdk = Path(os.environ.get("ANDROID_HOME") or os.environ.get("ANDROID_SDK_ROOT") or PROJECT / ".tmp/android-sdk")
    tools = sorted((sdk / "build-tools").glob("*"), key=lambda p: tuple(int(x) for x in p.name.split(".") if x.isdigit()))[-1]
    platforms = sorted((sdk / "platforms").glob("android-*"), key=lambda p: int(p.name.split("-")[-1]))
    android_jar = platforms[-1] / "android.jar"
    kotlin_home = os.environ.get("KOTLIN_HOME")
    kotlinc = executable("kotlinc", Path(kotlin_home) / "bin/kotlinc.bat" if kotlin_home else None)
    kotlin_lib = (Path(kotlin_home) if kotlin_home else kotlinc.parent.parent) / "lib"
    stdlib = kotlin_lib / "kotlin-stdlib.jar"
    java_home = os.environ.get("JAVA_HOME")
    java = executable("java", Path(java_home) / "bin/java.exe" if java_home else None)
    suffix = ".exe" if os.name == "nt" else ""
    batch = ".bat" if os.name == "nt" else ""
    aapt = tools / f"aapt2{suffix}"
    d8 = tools / f"d8{batch}"
    # Android Studio carries an R8/D8 matched to its bundled Kotlin compiler.
    studio_r8 = kotlin_lib.parent.parent.parent / "android/lib/r8.jar"
    r8 = Path(os.environ.get("R8_JAR", str(studio_r8)))
    zipalign = tools / f"zipalign{suffix}"
    signer = tools / f"apksigner{batch}"
    shell_root = (FRONTEND / ".nsh/android-shell").resolve()
    shell_root.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="build-", dir=shell_root)).resolve()
    # All recursive cleanup is confined to this verified build workspace.
    if not work.is_relative_to(shell_root):
        raise RuntimeError("Android build workspace escaped its root")
    env = dict(os.environ)
    env["ANDROID_HOME"] = str(sdk)
    env["ANDROID_SDK_ROOT"] = str(sdk)
    env["TEMP"] = env["TMP"] = str(work)
    env["JAVA_TOOL_OPTIONS"] = env.get("JAVA_TOOL_OPTIONS", "") + f' -Djava.io.tmpdir="{work}"'
    try:
        run(nsh, "package", "App.tsx", "--target", "android", "--name", "Alisten",
            "--out", work / "base", "--abi", "arm64-v8a", env=env)
        base = work / "base/Alisten.apk"
        config = json.loads((FRONTEND / "nsh.json").read_text(encoding="utf-8"))
        manifest = (ANDROID / "AndroidManifest.xml").read_text(encoding="utf-8")
        manifest = manifest.replace("@BUILD@", str(config["build"])).replace("@VERSION@", config["version"])
        (work / "AndroidManifest.xml").write_text(manifest, encoding="utf-8")
        icon_dir = work / "res/mipmap-xxxhdpi"
        icon_dir.mkdir(parents=True)
        shutil.copyfile(FRONTEND / "icon.png", icon_dir / "ic_launcher.png")
        with zipfile.ZipFile(runtime / "embed/android/nsh.aar") as aar:
            (work / "nsh.jar").write_bytes(aar.read("classes.jar"))
        classpath = os.pathsep.join(map(str, (android_jar, work / "nsh.jar", stdlib)))
        run(java, "-Xmx512m", "--add-opens", "java.base/java.util=ALL-UNNAMED",
            "-cp", str(kotlin_lib / "*"), "org.jetbrains.kotlin.cli.jvm.K2JVMCompiler",
            *sorted((ANDROID / "src").rglob("*.kt")), "-jvm-target", "1.8",
            "-no-reflect", "-no-stdlib", "-classpath", classpath, "-d", work / "host.jar", env=env)
        dex = work / "dex"
        dex.mkdir()
        dex_command = [java, "-cp", r8, "com.android.tools.r8.D8"] if r8.is_file() else [d8]
        run(*dex_command, "--release", "--min-api", "28", "--lib", android_jar, "--classpath", work / "nsh.jar",
            "--output", dex, work / "host.jar", stdlib, env=env)
        run(aapt, "compile", "--dir", work / "res", "-o", work / "res.zip", env=env)
        run(aapt, "link", "--manifest", work / "AndroidManifest.xml", "-I", android_jar,
            "--auto-add-overlay", "-o", work / "shell.ap_", work / "res.zip", env=env)
        unsigned = work / "unsigned.apk"
        with zipfile.ZipFile(unsigned, "w") as output:
            with zipfile.ZipFile(base) as source:
                for name in source.namelist():
                    if name == "AndroidManifest.xml" or name == "resources.arsc" or name.startswith(("META-INF/", "res/")):
                        continue
                    output.writestr(name, source.read(name), compress_type=zipfile.ZIP_DEFLATED, compresslevel=9)
            with zipfile.ZipFile(work / "shell.ap_") as shell:
                for name in shell.namelist():
                    output.writestr(name, shell.read(name), compress_type=zipfile.ZIP_STORED)
            output.write(dex / "classes.dex", "classes2.dex", compress_type=zipfile.ZIP_DEFLATED)
        destination = FRONTEND / f".nsh/out/alisten-android-release/Alisten-{config['version']}-arm64-v8a-compressed-release.apk"
        destination.parent.mkdir(parents=True, exist_ok=True)
        run(zipalign, "-P", "16", "-f", "4", unsigned, destination, env=env)
        keystore = Path(os.environ.get("NSH_KEYSTORE_PATH", str(Path.home() / ".android/debug.keystore")))
        env["ALISTEN_SIGNING_PASSWORD"] = os.environ.get("NSH_KEYSTORE_PASSWORD", "android")
        run(signer, "sign", "--ks", keystore, "--ks-key-alias", os.environ.get("NSH_KEY_ALIAS", "androiddebugkey"),
            "--ks-pass", "env:ALISTEN_SIGNING_PASSWORD", "--key-pass", "env:ALISTEN_SIGNING_PASSWORD", destination, env=env)
        run(signer, "verify", destination, env=env)
        print(f"Android APK with Kotlin playback service: {destination}")
        if "NSH_KEYSTORE_PATH" not in os.environ:
            print("Signing: existing local Android debug key (compatible with development installs)")
    finally:
        if work.is_relative_to(shell_root):
            shutil.rmtree(work)


if __name__ == "__main__":
    main()
