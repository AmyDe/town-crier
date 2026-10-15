#!/usr/bin/env python3
"""List secret candidates in recent GitHub Actions run logs for an LLM to judge.

Reuses the rules in candidates.py and adds log-only rules. Deliberately over-matches:
structural noise filters apply only to the high-entropy rules, so a provider-shaped
token, private key or credentialed URL is always reported. There is no allowlist.

Usage:
  log_candidates.py [--hours 48] [--repo OWNER/NAME] [--zip FILE ...]

Output: JSON lines on stdout. Each candidate is
  {"type": "candidate", "fingerprint", "rules", "masked", "occurrences", "runs", "examples": [...]}
and the last line is
  {"type": "summary", "runs", "attempts", "downloaded", "unavailable": [...], "candidates"}.
A fingerprint is sha256(value)[:16], so the same leaked value has the same
fingerprint in every run, every day and under every rule that matched it. Values are only ever printed masked.
Exit status is 0 whether or not candidates are found, and 2 if the run list cannot be read.
"""

import argparse
import collections
import datetime
import hashlib
import io
import json
import os
import re
import subprocess
import sys
import zipfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import candidates as c  # noqa: E402

TIMESTAMP = re.compile(r"^﻿?\d{4}-\d\d-\d\dT[\d:.]+Z ")

LOG_RULES = [
    ("json-secret-field", re.compile(
        r'(?i)"(?:access_?token|refresh_?token|id_?token|client_?secret|password|secret|api_?key'
        r'|connection_?string|primary_?key|secondary_?key|primary_?connection_?string'
        r'|secondary_?connection_?string|account_?key|sas_?token)"\s*:\s*"([^"]{8,})"')),
    ("connection-string", re.compile(
        r"(?i)(?:endpoint=(?:https|sb)://[^;\s\"']+;[^\s\"']*(?:accesskey|sharedaccesskey)=[^;\s\"']+"
        r"|DefaultEndpointsProtocol=[^\s\"']+|InstrumentationKey=[^\s\"']+)")),
    ("pulumi-plain-output", re.compile(
        r"(?i)^\s*[~+-]?\s*\w*(?:secret|password|key|token|connectionstring)\w*\s*:\s*\"([^\"\[]{16,})\"")),
]

# Public by design: App Insights ingestion key (shipped in the web bundle) and Auth0 SPA client IDs.
PUBLIC_VALUE = re.compile(r"(?i)^InstrumentationKey=[0-9a-f-]{36};IngestionEndpoint=")
PUBLIC_CONTEXT = re.compile(r"(?i)auth0-client-id|AUTH0_CLIENT_ID|secretRef\"?\s*:")

NOISE_CONTEXT = re.compile(
    r"(?i)cache|##\[command\]/usr/bin/docker|github_network_|content_?md5|(?:Print service|Stop and remove) container|SHA256:|sha(?:256|384|512)[-:]"
    r"|\[new branch\]|\[new tag\]|->\s+origin/|\"version\"\s*:|add-mask::\$|Expires=\d+&.*Signature="
    r"|latest(?:Ready)?Revision|Deactivating ca-|digest:|integrity"
)
NOISE_VALUE = re.compile(
    r"(?i)^ca-town-crier-[a-z0-9-]+$|\.azurecontainerapps\.io|^(?:node|ios|spm|setup-go|go|golangci|npm|gradle|pip)[-.]"
)
UUID_ONLY = re.compile(r"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")
HEX_ONLY = re.compile(r"^[0-9a-fA-F]+$")
WINDOW = 4000
WINDOW_OVERLAP = 512
CONTEXT_BEFORE = 120
CONTEXT_WIDTH = 300


def fingerprint(value):
    return hashlib.sha256(value.encode()).hexdigest()[:16]


def keep(rule, value, line):
    if "***" in value or not value.strip():
        return False
    if PUBLIC_VALUE.match(value) or PUBLIC_CONTEXT.search(line):
        return False
    if UUID_ONLY.match(value):
        return False
    if rule.startswith("high-entropy"):
        if HEX_ONLY.match(value) or NOISE_VALUE.search(value) or NOISE_CONTEXT.search(line):
            return False
    return True


def windows(line):
    # Overlapping windows keep a secret near a window edge whole without one huge regex input.
    if len(line) <= WINDOW:
        return [line]
    step = WINDOW - WINDOW_OVERLAP
    return [line[i:i + WINDOW] for i in range(0, len(line) - WINDOW_OVERLAP, step)]


def scan_log_line(line):
    """Return (masked line, [(value, rules)]); every raw hit is masked, kept or not."""
    line = TIMESTAMP.sub("", line)
    hits = []
    for chunk in windows(line):
        hits += [(rule, value) for _, _, rule, value in c.scan_line("log.txt", 0, chunk)]
        for rule, rx in LOG_RULES:
            for m in rx.finditer(chunk):
                hits.append((rule, m.group(m.lastindex or 0)))
    kept = {}
    for rule, value in hits:
        value = value.strip()
        if keep(rule, value, line):
            kept.setdefault(value, set()).add(rule)
    maximal = [v for v in kept if not any(v != o and v in o for o in kept)]
    merged = {v: set(kept[v]) for v in maximal}
    for value, found in kept.items():
        for container in maximal:
            if value != container and value in container:
                merged[container] |= found
    masked = line
    for value in sorted({v.strip() for _, v in hits if v.strip()}, key=len, reverse=True):
        masked = masked.replace(value, c.mask(value))
    return masked, list(merged.items())


def context(masked_line, value):
    at = max(masked_line.find(c.mask(value)), 0)
    start = max(at - CONTEXT_BEFORE, 0)
    return masked_line[start:start + CONTEXT_WIDTH]


def scan_zip(data, meta, agg):
    with zipfile.ZipFile(io.BytesIO(data)) as z:
        for name in z.namelist():
            if name.endswith("/"):
                continue
            text = z.read(name).decode("utf-8", errors="replace")
            for lineno, raw in enumerate(text.splitlines(), 1):
                line, hits = scan_log_line(raw)
                for value, found in hits:
                    entry = agg[value]
                    entry["rules"] |= found
                    entry["occurrences"] += 1
                    entry["runs"].add(meta["run_id"])
                    step = (meta["workflow"], name)
                    if step not in entry["steps"] and len(entry["examples"]) < 3:
                        entry["steps"].add(step)
                        entry["examples"].append({
                            **meta,
                            "job_file": name,
                            "line": lineno,
                            "context": context(line, value),
                        })


def gh_json(path):
    out = subprocess.run(["gh", "api", path], capture_output=True, check=True).stdout
    return json.loads(out)


def gh_bytes(path):
    proc = subprocess.run(["gh", "api", path], capture_output=True)
    return proc.stdout if proc.returncode == 0 else None


def list_runs(repo, since):
    runs = []
    page = 1
    while True:
        data = gh_json(f"repos/{repo}/actions/runs?per_page=100&page={page}&status=completed&created=>={since}")
        batch = data.get("workflow_runs", [])
        runs.extend(batch)
        if len(batch) < 100:
            return runs
        page += 1


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--hours", type=int, default=48)
    ap.add_argument("--repo", default="AmyDe/town-crier")
    ap.add_argument("--zip", nargs="+", help="scan these log archives instead of downloading")
    args = ap.parse_args()

    agg = collections.defaultdict(lambda: {"rules": set(), "occurrences": 0, "runs": set(), "steps": set(), "examples": []})
    summary = {"type": "summary", "runs": 0, "attempts": 0, "downloaded": 0, "unavailable": []}

    if args.zip:
        for path in args.zip:
            with open(path, "rb") as fh:
                scan_zip(fh.read(), {"workflow": "local", "run_id": os.path.basename(path), "attempt": 1, "url": ""}, agg)
            summary["attempts"] += 1
            summary["downloaded"] += 1
    else:
        since = (datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(hours=args.hours)).strftime("%Y-%m-%dT%H:%M:%SZ")
        try:
            runs = list_runs(args.repo, since)
        except (subprocess.CalledProcessError, json.JSONDecodeError) as err:
            print(f"cannot list runs: {err}", file=sys.stderr)
            return 2
        summary["runs"] = len(runs)
        for run in runs:
            for attempt in range(1, (run.get("run_attempt") or 1) + 1):
                summary["attempts"] += 1
                data = gh_bytes(f"repos/{args.repo}/actions/runs/{run['id']}/attempts/{attempt}/logs")
                if not data or not data.startswith(b"PK"):
                    summary["unavailable"].append(f"{run['id']}/{attempt}")
                    continue
                summary["downloaded"] += 1
                meta = {"workflow": run.get("name", ""), "run_id": run["id"], "attempt": attempt,
                        "created_at": run.get("created_at", ""), "head_sha": run.get("head_sha", ""),
                        "url": run.get("html_url", "")}
                scan_zip(data, meta, agg)

    for value, entry in sorted(agg.items(), key=lambda kv: -kv[1]["occurrences"]):
        print(json.dumps({
            "type": "candidate",
            "fingerprint": fingerprint(value),
            "rules": sorted(entry["rules"]),
            "masked": c.mask(value),
            "occurrences": entry["occurrences"],
            "runs": len(entry["runs"]),
            "examples": entry["examples"],
        }))
    summary["candidates"] = len(agg)
    print(json.dumps(summary))
    return 0


if __name__ == "__main__":
    sys.exit(main())
