"""Check a source-only or dummy-provisioned metadata image without provider calls."""
import argparse
import json
import re
import socket
import subprocess
import time
import urllib.request
import uuid


def verify(image, configured):
    name = "piratarr-metadata-check-" + uuid.uuid4().hex[:10]
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        port = probe.getsockname()[1]
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    base = f"http://127.0.0.1:{port}"
    marker = b"dummy-release-read-token"
    started = False
    def get(path):
        with opener.open(base + path, timeout=3) as response:
            return response.read()
    try:
        subprocess.run(["docker", "run", "--detach", "--rm", "--name", name, "--env", f"PIRATARR_LISTEN=0.0.0.0:{port}", "--publish", f"127.0.0.1:{port}:{port}", image], capture_output=True, text=True, check=True, timeout=30)
        started = True
        deadline = time.monotonic() + 30
        while True:
            try:
                if json.loads(get("/api/v1/health/ready"))["status"] == "ready":
                    break
            except OSError:
                pass
            if time.monotonic() >= deadline:
                raise RuntimeError("Metadata package did not become ready")
            time.sleep(0.2)
        status = get("/api/v1/metadata/status")
        assert json.loads(status) == {"provider": "tmdb", "configured": configured}
        responses = [status, get("/api/v1/catalog/movies"), get("/api/v1/about"), get("/about"), get("/tmdb.svg")]
        assert json.loads(responses[1])["items"] == []
        assert b"<svg" in responses[4]
        for path in re.findall(rb'(?:src|href)="(/assets/[^" ]+)"', get("/")):
            responses.append(get(path.decode()))
        assert all(marker not in payload for payload in responses), "Embedded credential leaked through public response/assets"
        embedded = subprocess.run(["docker", "exec", name, "sh", "-ec", "grep -a -q dummy-release-read-token /usr/local/bin/piratarr"], capture_output=True, timeout=10)
        assert (embedded.returncode == 0) == configured, "Dummy provisioning did not match the built executable"
        print(f"PASS: metadata package configured={configured}; readiness/catalog/credits work; no credential in API or frontend; no provider calls")
    finally:
        if started:
            subprocess.run(["docker", "stop", "--time", "20", name], capture_output=True, check=True, timeout=30)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", required=True)
    parser.add_argument("--dummy-provisioned", action="store_true", help="Image intentionally embeds only the known dummy test credential")
    options = parser.parse_args()
    verify(options.image, options.dummy_provisioned)
