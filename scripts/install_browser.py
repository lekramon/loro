"""Install Lôro's optional browser runtime outside the synchronized checkout."""
import argparse
import os
from pathlib import Path
import subprocess
import sys
import venv


def main():
    default = Path(os.environ.get("LOCALAPPDATA", Path.home() / ".local")) / "LouroBrowser"
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, default=default)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    target = args.directory.resolve()
    if target == root or root in target.parents:
        parser.error("Keep browser runtime outside the repository.")
    if sys.version_info[:2] != (3, 13):
        parser.error("Use Python 3.13 for this pinned runtime.")
    venv.EnvBuilder(with_pip=True).create(target)
    python = target / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
    subprocess.run([str(python), "-m", "pip", "install", "--disable-pip-version-check", "-r", str(root / "browser-requirements.txt")], check=True)
    subprocess.run([str(python), "-m", "invisible_playwright", "fetch"], check=True)
    subprocess.run([str(python), "-m", "pip", "check"], check=True)
    print(f"Browser runtime installed. LOURO_BROWSER_PYTHON={python}")


if __name__ == "__main__":
    main()
