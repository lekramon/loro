"""Ephemeral browser fetch and a bounded, site-specific HTML parser.

Original Lôro implementation. No HTTP service or persistent browser profile.
"""
import asyncio
import contextlib
import io
import json
import re
import sys
import time
from html.parser import HTMLParser
from urllib.parse import urlparse


_trace_stream = sys.stderr
_trace_start = time.monotonic()


def trace(phase, **fields):
    event = {"phase": phase, "elapsedMs": int((time.monotonic() - _trace_start) * 1000)}
    event.update({key: value for key, value in fields.items() if key in ("status", "count", "code", "kind")})
    with contextlib.suppress(Exception):
        _trace_stream.write(json.dumps(event, ensure_ascii=True) + "\n")
        _trace_stream.flush()


class IndexPage(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.rows = []
        self.row = None
        self.field = None
        self.link = False
        self.table = False
        self.magnets = set()
        self.hrefs = []

    def handle_starttag(self, tag, attributes):
        attrs = dict(attributes)
        if tag == "table" and "table-list" in attrs.get("class", "").split():
            self.table = True
        if tag == "tr":
            self.row = {"title": "", "path": "", "category": "", "size": "", "seeders": ""}
        if tag == "td":
            classes = attrs.get("class", "").split()
            self.field = next((c for c in classes if c in ("coll-1", "coll-2", "coll-4")), None)
        if tag == "a":
            href = attrs.get("href", "")
            self.hrefs.append(href)
            if href.startswith("magnet:"):
                self.magnets.add(href)
            if self.row is not None and self.field == "coll-1":
                if re.fullmatch(r"/torrent/[0-9]+/[^?#]*", href):
                    self.row["path"] = href
                    self.link = True
                elif re.fullmatch(r"/sub/[0-9]+/(?:[0-9]+/)?", href):
                    self.row["category"] = href.strip("/").split("/")[1]

    def handle_data(self, text):
        if self.row is None:
            return
        if self.link:
            self.row["title"] += text
        elif self.field == "coll-2":
            self.row["seeders"] += text
        elif self.field == "coll-4":
            self.row["size"] += text

    def handle_endtag(self, tag):
        if tag == "a":
            self.link = False
        if tag == "td":
            self.field = None
        if tag == "tr" and self.row is not None:
            if self.row["path"]:
                self.rows.append({key: value.strip() for key, value in self.row.items()})
            self.row = None


def parse_page(content, mode, address):
    if len(content.encode("utf-8")) > 2 * 1024 * 1024:
        return {"error": "source-invalid-response"}
    parser = IndexPage()
    parser.feed(content)
    if mode == "detail":
        if len(parser.magnets) != 1:
            return {"error": "ambiguous-reference" if parser.magnets else "missing-reference"}
        magnet = next(iter(parser.magnets))
        return {"magnet": magnet} if len(magnet) <= 8192 else {"error": "unsupported-reference"}
    explicit_empty = "no results were returned" in content.lower()
    if not parser.table and not explicit_empty:
        return {"error": "source-invalid-response"}
    if parser.table and not parser.rows and not explicit_empty:
        return {"error": "source-invalid-response"}
    if len(parser.rows) > 40 or len(parser.hrefs) > 10000:
        return {"error": "source-invalid-response"}
    page_number = int(urlparse(address).path.rstrip("/").split("/")[-1])
    next_path = urlparse(address).path.rstrip("/").rsplit("/", 1)[0] + f"/{page_number + 1}/"
    more = any(urlparse(href).path == next_path for href in parser.hrefs)
    movie_categories = {"1", "2", "3", "4", "42", "54", "55", "66", "70", "73", "76"}
    return {"rows": [dict(r, position=i) for i, r in enumerate(parser.rows) if r["category"] in movie_categories],
            "rawCount": len(parser.rows), "hasMore": more or len(parser.rows) == 40}


def challenge_html(content):
    content = content.lower()
    return "/cdn-cgi/challenge-platform/" in content or "cf-chl-" in content or (
        "just a moment" in content and "cloudflare" in content)


async def fetch(request):
    from invisible_playwright.async_api import InvisiblePlaywright
    from invisible_core.download import cache_dir_for_seal
    from invisible_core.seal import active_seal
    import platform
    seal = active_seal()
    entry = cache_dir_for_seal(seal) / seal.asset_for(sys.platform, platform.machine()).entry_rel
    if not entry.is_file():
        return {"error": "source-browser-unavailable"}
    deadline = asyncio.get_running_loop().time() + 50
    address = request["url"]
    origin = urlparse(address)
    async with InvisiblePlaywright(binary_path=str(entry), headless=True, humanize=True, timezone="America/Sao_Paulo", locale="en-US") as browser:
        context = await browser.new_context(accept_downloads=False)
        page = await context.new_page()
        trace("browser.started")
        page.on("request", lambda request: trace("request.started", kind=request.resource_type if request.resource_type in ("document", "script", "stylesheet", "image", "xhr", "fetch", "font", "media") else "other"))
        blocked_navigation = False
        redirect_response = False
        challenge_seen = False
        def inspect_response(response):
            nonlocal redirect_response, challenge_seen
            kind = response.request.resource_type
            if kind == "document" and response.headers.get("cf-mitigated", "") == "challenge" and not challenge_seen:
                trace("browser.challenge.detected")
                challenge_seen = True
            trace("browser.response", status=response.status, kind=kind if kind in ("document", "script", "stylesheet", "image", "xhr", "fetch", "font", "media") else "other")
            if response.request.is_navigation_request() and 300 <= response.status < 400:
                redirect_response = True
        page.on("response", inspect_response)
        # Top-level navigation must not turn a site-controlled link into a new origin.
        async def route_request(route):
            nonlocal blocked_navigation
            target = urlparse(route.request.url)
            if route.request.is_navigation_request() and route.request.frame == page.main_frame:
                if (target.scheme, target.netloc) != (origin.scheme, origin.netloc):
                    blocked_navigation = True
                    await route.abort()
                    return
            if target.scheme not in ("http", "https", "data", "blob", "about"):
                await route.abort()
                return
            await route.continue_()
        await context.route("**/*", route_request)
        page.on("popup", lambda popup: asyncio.create_task(popup.close()))
        trace("browser.navigation")
        try:
            response = await page.goto(address, wait_until="domcontentloaded", timeout=45000)
        except Exception as exc:
            if blocked_navigation or redirect_response:
                return {"error": "source-invalid-response"}
            return {"error": "source-timeout" if type(exc).__name__ == "TimeoutError" else "source-unavailable"}
        await asyncio.sleep(0.5)
        challenged = response and response.headers.get("cf-mitigated", "") == "challenge"
        last_click = 0
        previous_invalid = None
        last_widget_count = None
        while True:
            if (urlparse(page.url).scheme, urlparse(page.url).netloc) != (origin.scheme, origin.netloc):
                return {"error": "source-invalid-response"}
            content = await page.content()
            title = (await page.title()).lower()
            result = parse_page(content, request["mode"], address)
            if "error" not in result and not challenge_html(content) and "just a moment" not in title:
                if challenge_seen:
                    trace("browser.challenge.cleared")
                trace("browser.parsed", count=result.get("rawCount", 1 if result.get("magnet") else 0))
                break
            if not challenge_html(content) and "just a moment" not in title and (not challenged or title):
                # A challenge can navigate to the target while its response is
                # still streaming. Do not classify a partial <head> as bad HTML.
                invalid = (hash(content), result["error"])
                if previous_invalid == invalid and await page.evaluate("document.readyState") != "loading":
                    break
                previous_invalid = invalid
                if asyncio.get_running_loop().time() >= deadline:
                    return {"error": "source-timeout"}
                await asyncio.sleep(0.5)
                continue
            previous_invalid = None
            if not challenge_seen:
                trace("browser.challenge.detected")
                challenge_seen = True
            if asyncio.get_running_loop().time() >= deadline:
                return {"error": "source-challenge-required"}
            # Some interstitials finish automatically. A checkbox may be exposed
            # by the challenge frame; leave unresolved challenges explicit.
            for frame in page.frames if asyncio.get_running_loop().time() - last_click > 5 else []:
                if urlparse(frame.url).hostname == "challenges.cloudflare.com":
                    checkbox = frame.locator('input[type="checkbox"]')
                    if await checkbox.count() and await checkbox.first.is_visible():
                        with contextlib.suppress(Exception):
                            trace("browser.interaction", count=1)
                            last_click = asyncio.get_running_loop().time()
                            await checkbox.first.click(timeout=500)
                    else:
                        label = frame.locator("label")
                        if await label.count() and await label.first.is_visible():
                            with contextlib.suppress(Exception):
                                trace("browser.interaction", count=1)
                                last_click = asyncio.get_running_loop().time()
                                await label.first.click(timeout=500)
            if asyncio.get_running_loop().time() - last_click > 5:
                widget = page.locator('input[name="cf-turnstile-response"]')
                widget_count = await widget.count()
                if widget_count != last_widget_count:
                    trace("browser.widget.found", count=widget_count)
                    last_widget_count = widget_count
                if widget_count:
                    point = await widget.first.evaluate("""node => {
                        for (let parent = node.parentElement; parent; parent = parent.parentElement) {
                            const box = parent.getBoundingClientRect();
                            if (box.width >= 200 && box.height >= 40 && box.height <= 100)
                                return {x: box.x + 21, y: box.y + box.height / 2};
                        }
                        return null;
                    }""")
                    if point:
                        trace("browser.interaction", count=1)
                        await page.mouse.click(point["x"], point["y"])
                        last_click = asyncio.get_running_loop().time()
            await asyncio.sleep(0.5)
        if (urlparse(page.url).scheme, urlparse(page.url).netloc) != (origin.scheme, origin.netloc):
            return {"error": "source-invalid-response"}
        result = parse_page(content, request["mode"], address)
        if "error" in result and response and response.status == 429:
            return {"error": "source-rate-limited"}
        return result


async def main():
    request = json.loads(sys.stdin.buffer.read(8193))
    # Library diagnostics must never corrupt the private JSON protocol or logs.
    with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
        try:
            result = await asyncio.wait_for(fetch(request), timeout=55)
        except ImportError:
            result = {"error": "source-browser-unavailable"}
        except (asyncio.TimeoutError, TimeoutError):
            result = {"error": "source-timeout"}
        except Exception as exc:
            result = {"error": "source-timeout" if type(exc).__name__ == "TimeoutError" else "source-browser-unavailable"}
    if result.get("error"):
        trace("browser.failed", code=result["error"])
    sys.stdout.write(json.dumps(result, ensure_ascii=True))


if __name__ == "__main__":
    asyncio.run(main())
