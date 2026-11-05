"""Verify the Linux distribution through Compose, HTTP and disposable bind mounts.

Build the image first: docker compose build (or docker build -t louro:local .).
Requires a running Linux Docker engine and an unused loopback port (--port).
Only fresh temporary directories are initialized; existing user media is untouched.
"""
import argparse
import hashlib
import json
import os
import re
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[1]
BASE = "http://127.0.0.1:8080"
HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def command(args, *, env=None, timeout=120, allow_failure=False):
    result = subprocess.run(args, cwd=ROOT, env=env, capture_output=True, text=True, timeout=timeout)
    if result.returncode and not allow_failure:
        raise RuntimeError(f"Command failed: {' '.join(args)}\n{result.stdout}\n{result.stderr}")
    return result


def request(path, body=None, expected=200):
    headers = {}
    if body is not None:
        headers = {"Content-Type": "application/json", "X-Piratarr-Request": "1"}
    req = urllib.request.Request(BASE + path, data=None if body is None else json.dumps(body).encode(), headers=headers)
    try:
        response = HTTP.open(req, timeout=5)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        data = response.read()
        assert response.status == expected, (path, response.status, data[:300])
    return data


def ready():
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        try:
            if json.loads(request("/api/v1/health/ready"))["status"] == "ready":
                return
        except (OSError, AssertionError, ValueError):
            pass
        time.sleep(0.5)
    raise RuntimeError("Container readiness did not succeed within 60 seconds")


def verify(image, port):
    global BASE
    BASE = f"http://127.0.0.1:{port}"
    info = command(["docker", "info", "--format", "{{.OSType}}"], timeout=20).stdout.strip()
    assert info == "linux", "A Linux Docker engine is required"
    # Fail before starting Compose if another local application owns this port.
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", port))
    temporary_parent = Path(tempfile.gettempdir()).resolve()
    fixture = Path(tempfile.mkdtemp(prefix="piratarr-container-", dir=temporary_parent)).resolve()
    assert fixture.parent == temporary_parent and fixture.name.startswith("piratarr-container-")
    config = fixture / "config"
    data = fixture / "data"
    config.mkdir()
    data.mkdir()
    (data / "downloads" / "nested").mkdir(parents=True)
    (data / "media" / "movies").mkdir(parents=True)
    (data / "readonly").mkdir()
    sentinel = data / "media" / "movies" / "existing.mkv"
    sentinel.write_bytes(b"existing media must remain untouched")
    fingerprint = hashlib.sha256(sentinel.read_bytes()).hexdigest()
    project = "piratarr-check-" + uuid.uuid4().hex[:10]
    environment = os.environ.copy()
    environment.update(PIRATARR_CONFIG_DIR=str(config), PIRATARR_STORAGE_DIR=str(data), PIRATARR_UID="10001", PIRATARR_GID="10001", PIRATARR_WORKERS="4", PIRATARR_PORT=str(port))
    override = fixture / "override.json"
    override.write_text(json.dumps({"services": {"piratarr": {"image": image, "restart": "no"}}}), encoding="utf-8")
    compose = ["docker", "compose", "-p", project, "-f", str(ROOT / "compose.yaml"), "-f", str(override)]
    started = False
    try:
        # Root is used only to prepare this newly created, empty test fixture.
        command(["docker", "run", "--rm", "--user", "0:0", "--entrypoint", "sh", "--mount", f"type=bind,source={fixture},target=/fixture", image, "-ec", "mkdir -p /fixture/data/downloads /fixture/data/media/movies /fixture/data/readonly; chown 10001:10001 /fixture/config /fixture/data /fixture/data/downloads /fixture/data/media /fixture/data/media/movies /fixture/data/readonly; chmod 700 /fixture/config; chmod 755 /fixture/data /fixture/data/downloads /fixture/data/media /fixture/data/media/movies; chmod 555 /fixture/data/readonly"])
        resolved = json.loads(command(compose + ["config", "--format", "json"], env=environment).stdout)
        service = resolved["services"]["piratarr"]
        assert service["user"] == "10001:10001"
        assert all(port.get("host_ip") == "127.0.0.1" for port in service["ports"])
        assert all(item["target"] == port and int(item["published"]) == port for item in service["ports"])
        assert len(service["volumes"]) == 2
        started = True
        command(compose + ["up", "--detach", "--no-build", "--wait", "--wait-timeout", "60"], env=environment)
        ready()
        container = command(compose + ["ps", "--quiet", "piratarr"], env=environment).stdout.strip()
        def execute(*args, success=True):
            result = command(["docker", "exec", container, *args], allow_failure=not success)
            if not success:
                assert result.returncode != 0, "Expected failure did not occur"
            return result.stdout.strip()
        assert execute("id", "-u") == "10001"
        assert execute("stat", "-c", "%a", "/config") == "700"
        mode_before = execute("stat", "-c", "%a:%u:%g", "/data/media/movies/existing.mkv")
        execute("ffprobe", "-version")
        execute("sh", "-ec", "! command -v node; ! command -v npm; test -s /etc/ssl/certs/ca-certificates.crt")
        execute("/usr/local/bin/louro", "healthcheck")
        execute("/usr/local/bin/piratarr", "healthcheck")
        assert json.loads(request("/api/v1/setup"))["completed"] is False
        html = request("/")
        assert b"<html" in html.lower()
        assert b"<title>Louro</title>" in html
        assert json.loads(request("/api/v1/about"))["name"] == "Louro"
        assets = re.findall(rb'(?:src|href)="(/assets/[^" ]+)"', html)
        assert assets, "Embedded page references no production assets"
        for asset in assets:
            assert len(request(asset.decode())) > 0
        assert b"<html" in request("/preview").lower()
        assert b"<svg" in request("/mock-art/orbit.svg")
        payload = {"directories": {"downloadDirectory": "/data/downloads", "movieDirectory": "/data/media/movies"}, "profile": {"acceptedResolutions": ["720p", "1080p"], "preferredResolution": "1080p", "audioPreferences": ["pt", "original"], "acceptedFormats": ["remux", "web-dl"], "acceptedContainers": ["mkv"]}}
        for invalid in ["/data/downloads", "/data/downloads/nested", "/config", "/data/readonly"]:
            attempt = json.loads(json.dumps(payload))
            attempt["directories"]["movieDirectory"] = invalid
            request("/api/v1/setup", attempt, expected=422)
        execute("ln", "-s", "/data/downloads", "/data/download-alias")
        attempt = json.loads(json.dumps(payload))
        attempt["directories"]["movieDirectory"] = "/data/download-alias"
        request("/api/v1/setup", attempt, expected=422)
        saved = json.loads(request("/api/v1/setup", payload, expected=201))
        assert saved["revision"] == 1
        execute("/usr/local/bin/piratarr", "healthcheck")
        exclusion = command(["docker", "exec", container, "/usr/local/bin/piratarr", "--data-dir", "/config", "--listen", "0.0.0.0:8082"], allow_failure=True)
        assert exclusion.returncode != 0 and "another instance" in exclusion.stderr, "Second instance was not refused by the data-directory lock"
        assert json.loads(request("/api/v1/settings")) == saved
        command(compose + ["restart", "piratarr"], env=environment)
        ready()
        assert json.loads(request("/api/v1/settings")) == saved
        command(compose + ["up", "--detach", "--no-build", "--force-recreate", "--wait", "--wait-timeout", "60"], env=environment)
        ready()
        assert json.loads(request("/api/v1/settings")) == saved
        container = command(compose + ["ps", "--quiet", "piratarr"], env=environment).stdout.strip()
        assert execute("stat", "-c", "%a:%u:%g", "/data/media/movies/existing.mkv") == mode_before
        assert hashlib.sha256(sentinel.read_bytes()).hexdigest() == fingerprint
        assert not list(data.rglob(".piratarr-probe-*"))
    finally:
        if started:
            cleanup = command(compose + ["down", "--timeout", "20"], env=environment, allow_failure=True)
            if cleanup.returncode:
                print(f"Container cleanup failed; preserve fixture for inspection: {fixture}")
                raise RuntimeError("Container cleanup failed; fixture preserved")
        # Verified absolute disposable root; never delete supplied user data.
        assert fixture.parent == temporary_parent and fixture.name.startswith("piratarr-container-")
        if os.name == "posix":
            # Reclaim only this disposable fixture, without following symlinks.
            command(["docker", "run", "--rm", "--user", "0:0", "--entrypoint", "sh", "--mount", f"type=bind,source={fixture},target=/fixture", image, "-ec", f"chown -hR {os.getuid()}:{os.getgid()} /fixture"])
        shutil.rmtree(fixture)
    print("PASS: Linux non-root Compose, embedded UI, ffprobe, health before/after setup, path validation, exclusion, restart/recreation persistence, unchanged media and fixture cleanup")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", default="louro:local")
    parser.add_argument("--port", type=int, default=8080)
    options = parser.parse_args()
    if not 1 <= options.port <= 65535:
        parser.error("--port must be between 1 and 65535")
    verify(options.image, options.port)
