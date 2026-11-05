"""Opt-in TMDB smoke test against an isolated native Piratarr process.

Uses a private token FILE, never a token command argument. Creates and removes
only a fresh temporary catalog; the user's configured installation is untouched.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def verify(executable, token_file):
    if not token_file.is_file():
        raise RuntimeError("Private application credential file is unavailable")
    parent = Path(tempfile.gettempdir()).resolve()
    fixture = Path(tempfile.mkdtemp(prefix="piratarr-live-catalog-", dir=parent)).resolve()
    assert fixture.parent == parent and fixture.name.startswith("piratarr-live-catalog-")
    downloads, movies = fixture / "downloads", fixture / "movies"
    downloads.mkdir()
    movies.mkdir()
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        port = probe.getsockname()[1]
    base = f"http://127.0.0.1:{port}"
    process = None
    def request(path, body=None):
        headers = {} if body is None else {"Content-Type": "application/json", "X-Piratarr-Request": "1"}
        req = urllib.request.Request(base + path, data=None if body is None else json.dumps(body).encode(), headers=headers)
        with HTTP.open(req, timeout=15) as response:
            return response.status, response.headers.get("Content-Type", ""), response.read()
    def start(credential):
        child = subprocess.Popen([str(executable), "--data-dir", str(fixture / "config"), "--listen", f"127.0.0.1:{port}", "--tmdb-token-file", str(credential)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, creationflags=subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0)
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            if child.poll() is not None:
                raise RuntimeError("Isolated catalog process exited during startup")
            try:
                if request("/api/v1/health/ready")[0] == 200:
                    return child
            except OSError:
                pass
            time.sleep(0.1)
        child.terminate()
        child.wait(timeout=10)
        raise RuntimeError("Isolated catalog process did not become ready")
    try:
        process = start(token_file)
        status = json.loads(request("/api/v1/metadata/status")[2])
        assert status["configured"], "Application credential did not load"
        search = json.loads(request("/api/v1/metadata/search?query=Inception&language=pt-BR")[2])
        assert any(movie["tmdbId"] == 27205 for movie in search["results"]), "Expected public movie not found"
        request("/api/v1/setup", {"directories": {"downloadDirectory": str(downloads), "movieDirectory": str(movies)}, "profile": {"acceptedResolutions": ["1080p"], "preferredResolution": "1080p", "audioPreferences": ["original"]}})
        code, _, body = request("/api/v1/catalog/movies", {"tmdbId": 27205, "language": "pt-BR"})
        registered = json.loads(body)["movie"]
        assert code == 201 and registered["originalLanguage"] == "en" and registered["year"] == 2010
        assert registered["posterUrl"].startswith("/api/v1/metadata/images/")
        _, mime, artwork = request(registered["posterUrl"])
        assert mime in ("image/jpeg", "image/png") and artwork
        process.terminate()
        process.wait(timeout=10)
        process = start(fixture / "missing-credential")
        assert not json.loads(request("/api/v1/metadata/status")[2])["configured"]
        assert json.loads(request(f"/api/v1/catalog/movies/{registered['id']}")[2]) == registered
        assert request(registered["posterUrl"])[2] == artwork
        code, _, repeated = request("/api/v1/catalog/movies", {"tmdbId": 27205})
        assert code == 200 and json.loads(repeated)["movie"]["id"] == registered["id"]
        print("PASS: registered application credential, live TMDB search/details, local poster cache, and credential-free restart/offline catalog; no existing installation modified")
    finally:
        if process is not None and process.poll() is None:
            process.terminate()
            process.wait(timeout=10)
        assert fixture.parent == parent and fixture.name.startswith("piratarr-live-catalog-")
        shutil.rmtree(fixture)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--token-file", required=True, type=Path)
    parser.add_argument("--executable", type=Path, default=ROOT / "bin" / ("piratarr-catalog.exe" if os.name == "nt" else "piratarr-catalog"))
    args = parser.parse_args()
    verify(args.executable.resolve(), args.token_file.resolve())
