"""Check the preparation artifacts using Python's standard library only.

This is a documentation consistency check, not a complete OpenAPI validator.
Run from any directory: python /path/to/repository/scripts/verify_specs.py.
"""

import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
errors = []


def check(condition, message):
    if not condition:
        errors.append(message)


markdown = list(ROOT.glob("*.md")) + list((ROOT / "docs").rglob("*.md")) + list((ROOT / "specs").rglob("*.md"))
for path in markdown:
    if ".git" in path.relative_to(ROOT).parts:
        continue
    source = path.read_text(encoding="utf-8-sig")
    for target in re.findall(r"\[[^\]]*\]\(([^)\s]+)\)", source):
        if re.match(r"^[a-zA-Z][\w+.-]*:", target) or target.startswith("#"):
            continue
        local = target.split("#", 1)[0]
        check((path.parent / local).exists(), f"Broken link in {path.relative_to(ROOT)}: {target}")



def resolve(pointer):
    node = contract
    for part in pointer[2:].split("/"):
        node = node[part.replace("~1", "/").replace("~0", "~")]
    return node


def walk(node):
    if isinstance(node, dict):
        if "$ref" in node:
            pointer = node["$ref"]
            check(pointer.startswith("#/"), f"Unexpected external reference: {pointer}")
            try:
                resolve(pointer)
            except (KeyError, TypeError):
                errors.append(f"Unresolved reference: {pointer}")
        if node.get("type") == "object":
            check(set(node.get("required", [])) <= set(node.get("properties", {})), "Required property missing from schema")
        for child in node.values():
            walk(child)
    elif isinstance(node, list):
        for child in node:
            walk(child)


operations = []
for feature, expected in [("001-foundation", 8), ("002-catalog", 8)]:
    contract = json.loads((ROOT / "specs" / feature / "openapi.json").read_text(encoding="utf-8"))
    check(contract.get("openapi") == "3.1.1", "Unexpected OpenAPI version")
    walk(contract)
    current = []
    for api_path, methods in contract["paths"].items():
        check(api_path.startswith("/api/v1/"), f"Unversioned path: {api_path}")
        for method, operation in methods.items():
            current.append(operation["operationId"])
            if method in {"post", "put", "patch", "delete"}:
                check(bool(operation.get("requestBody")), f"Mutation lacks body: {api_path}")
                headers = operation.get("parameters", [])
                check({"$ref": "#/components/parameters/MutationHeader"} in headers or any(p.get("name") == "X-Piratarr-Request" and p.get("required") for p in headers), f"Mutation lacks header: {api_path}")
                check({"400", "403", "413", "415", "422", "503"} <= set(operation["responses"]), f"Missing mutation failure contracts: {api_path}")
    check(len(current) == len(set(current)) == expected, f"Expected {expected} uniquely named operations in {feature}")
    operations.extend(current)
check(len(operations) == len(set(operations)), "Duplicate operation IDs across contracts")

tickets = sorted((ROOT / "specs").glob("*/issues/*.md"))
check(len(list((ROOT / "specs/001-foundation/issues").glob("*.md"))) == 4, "Expected four foundation tickets")
graph = {}
criteria = set(re.findall(r"\| (FND-\d{3}) \|", (ROOT / "specs/001-foundation/spec.md").read_text(encoding="utf-8")))
covered = set()
labels = {"needs-triage", "needs-info", "ready-for-agent", "ready-for-human", "wontfix"}
for path in tickets:
    content = path.read_text(encoding="utf-8")
    label = re.search(r"^Label: (.+)$", content, re.M)
    state = re.search(r"^State: (.+)$", content, re.M)
    blockers = re.search(r"^Blocked by: (.+)$", content, re.M)
    check(label is not None and label[1] in labels, f"Invalid label: {path.name}")
    check(state is not None and state[1] in {"open", "in-progress", "done"}, f"Invalid state: {path.name}")
    check(blockers is not None, f"Missing blockers: {path.name}")
    deps = [] if blockers is None or blockers[1] == "none" else [x.strip() for x in blockers[1].split(",")]
    key = path.relative_to(ROOT).as_posix()
    graph[key] = deps
    for dep in deps:
        check((ROOT / dep).is_file(), f"Missing blocker: {dep}")
    for criterion in re.findall(r"FND-\d{3}", content):
        check(criterion in criteria, f"Unknown criterion {criterion} in {path.name}")
        covered.add(criterion)
check(criteria == covered, f"Uncovered foundation criteria: {sorted(criteria - covered)}")


def visit(ticket, trail):
    if ticket in trail:
        errors.append(f"Dependency cycle: {' -> '.join(trail + [ticket])}")
        return
    for dependency in graph.get(ticket, []):
        visit(dependency, trail + [ticket])


for key in graph:
    visit(key, [])

product = (ROOT / "specs/000-product/spec.md").read_text(encoding="utf-8")
check("brolarr" not in product.lower(), "Removed scope remains in product specification")
check("C#" not in product and "undecided" not in product, "Superseded stack decision remains")
check(not (ROOT / "planejamento").exists(), "Removed planning folder was recreated")

if errors:
    raise SystemExit("Specification checks failed:\n" + "\n".join(f"- {e}" for e in errors))
print(f"PASS: {len(markdown)} Markdown files, {len(operations)} API operations, {len(tickets)} tickets, {len(criteria)} foundation criteria; local references and dependency graph valid.")
