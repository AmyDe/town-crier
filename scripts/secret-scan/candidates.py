#!/usr/bin/env python3
"""List secret candidates in the repository for an LLM to judge.

Deliberately over-matches: provider token shapes, secret-named assignments,
credentialed URLs, high-entropy literals and sensitive file types. Precision
is the judge's job, recall is this script's.

Usage:
  candidates.py [--history-since DATE] [--include-allowlisted] [--json]

Output: one candidate per line as
  <fingerprint> <path>:<line> <rule> <masked-value>
or JSON lines with --json. The fingerprint is stable across line moves, so
scripts/secret-scan/allowlist.txt can suppress judged false positives.
Exit status is 0 whether or not candidates are found.
"""

import argparse
import hashlib
import json
import math
import os
import re
import subprocess
import sys

ROOT = subprocess.run(
    ["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True
).stdout.strip()
SELF_DIR = "scripts/secret-scan/"
ALLOWLIST = os.path.join(ROOT, SELF_DIR, "allowlist.txt")

SKIP_DIRS = (
    "node_modules/", "vendor/", ".beads/", "dist/", "build/", ".build/",
    "Pods/", ".gradle/", SELF_DIR,
)
SKIP_FILES = re.compile(
    r"(^|/)(go\.sum|go\.mod|package-lock\.json|yarn\.lock|pnpm-lock\.yaml|Package\.resolved"
    r"|Gemfile\.lock|gradle\.lockfile|project\.pbxproj)$"
)
SKIP_EXT = re.compile(
    r"\.(png|jpe?g|gif|webp|ico|icns|pdf|ttf|otf|woff2?|mp4|mov|zip|gz|car|xcassets"
    r"|svg|map)$",
    re.I,
)
SENSITIVE_FILE = re.compile(
    r"(^|/)(\.env(\..+)?|id_(rsa|dsa|ecdsa|ed25519)|credentials(\.json)?"
    r"|GoogleService-Info\.plist|google-services\.json|\.npmrc|\.netrc|\.pgpass"
    r"|[^/]+\.(pem|key|p8|p12|pfx|jks|keystore|mobileprovision|ppk|kdbx|tfstate))$",
    re.I,
)
ENV_EXAMPLE = re.compile(r"\.env\.(example|sample|template)$", re.I)

PROVIDER_RULES = [
    ("private-key-block", r"-----BEGIN (?:[A-Z]+ )*PRIVATE KEY(?: BLOCK)?-----"),
    ("aws-access-key", r"\b(?:AKIA|ASIA|AGPA|AIDA|AROA)[0-9A-Z]{16}\b"),
    ("github-token", r"\b(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,})\b"),
    ("slack-token", r"\bxox[abposr]-[A-Za-z0-9-]{10,}"),
    ("slack-webhook", r"hooks\.slack\.com/services/[A-Za-z0-9/]{20,}"),
    ("stripe-key", r"\b(?:sk|rk|pk)_(?:live|test)_[A-Za-z0-9]{16,}"),
    ("google-api-key", r"\bAIza[0-9A-Za-z_-]{35}\b"),
    ("google-oauth-secret", r"\bGOCSPX-[A-Za-z0-9_-]{20,}"),
    ("anthropic-key", r"\bsk-ant-[A-Za-z0-9_-]{20,}"),
    ("openai-key", r"\bsk-(?:proj-)?[A-Za-z0-9_-]{32,}"),
    ("sendgrid-key", r"\bSG\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}"),
    ("twilio-key", r"\b(?:AC|SK)[0-9a-f]{32}\b"),
    ("npm-token", r"\bnpm_[A-Za-z0-9]{36}\b"),
    ("pypi-token", r"\bpypi-[A-Za-z0-9_-]{50,}"),
    ("jwt", r"\beyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}"),
    ("azure-storage-key", r"AccountKey=[A-Za-z0-9+/=]{20,}"),
    ("azure-sas", r"[?&]sig=[A-Za-z0-9%+/=]{20,}"),
    ("azure-conn-secret", r"(?:SharedAccessKey|Password|Pwd)=[^;\"'\s]{6,}"),
    ("azure-client-secret", r"\b[A-Za-z0-9_~.-]{3}8Q~[A-Za-z0-9_~.-]{31,34}\b"),
    ("pulumi-token", r"\bpul-[a-f0-9]{40}\b"),
    ("cloudflare-token", r"\b(?:CF|cf)[_-]?(?:API)?[_-]?TOKEN\s*[:=]\s*[\"']?[A-Za-z0-9_-]{37,}"),
    ("doltHub-jwk", r"\"d\"\s*:\s*\"[A-Za-z0-9_-]{40,}\""),
    ("url-credentials", r"\b[a-z][a-z0-9+.-]{1,20}://[^/\s:@\"'<>]{1,64}:[^/\s@\"'<>]{3,128}@[^\s\"'<>]+"),
    ("basic-auth-header", r"\bBasic\s+[A-Za-z0-9+/]{16,}={0,2}"),
    ("bearer-literal", r"\bBearer\s+[A-Za-z0-9._~+/-]{20,}=*"),
]
PROVIDER_RULES = [(name, re.compile(rx)) for name, rx in PROVIDER_RULES]

SECRET_NAME = (
    r"(?:secret|passw(?:or)?d|passwd|pwd|token|api[_-]?key|apikey|access[_-]?key"
    r"|private[_-]?key|client[_-]?secret|signing[_-]?key|auth[_-]?key|credential"
    r"|conn(?:ection)?[_-]?str(?:ing)?|webhook[_-]?url|salt|seed|bearer)"
)
ASSIGNMENT = re.compile(
    r"(?i)[\"']?[\w.-]*" + SECRET_NAME + r"[\w.-]*[\"']?\s*(?::=|=>|[:=])\s*"
    r"(?:[\w.]+\()?\s*[\"'`]([^\"'`\s]{8,})[\"'`]"
)
YAML_OR_ENV_ASSIGNMENT = re.compile(
    r"(?i)^\s*(?:export\s+)?[\w.-]*" + SECRET_NAME + r"[\w.-]*\s*[:=]\s*([^\s#\"'`{}$<>]{8,})\s*$"
)
QUOTED = re.compile(r"[\"'`]([A-Za-z0-9+/=_\-.~]{20,})[\"'`]")

PLACEHOLDER = re.compile(
    r"(?i)^(?:x+|\*+|\.+|0+|changeme|example|placeholder|redacted|dummy|your[_-].*|<.*>)$"
)
IDENTIFIER_LIKE = re.compile(r"^[A-Za-z]+(?:[_.-][A-Za-z]+)*$")
UUID = re.compile(r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}")
WORDISH = re.compile(r"[A-Z]?[a-z]{3,}|[A-Z]{2,}(?![a-z])")
PULUMI_CIPHERTEXT = re.compile(r"^\s*secure:\s")
CONFIG_FILE = re.compile(
    r"(^|/)(\.env[^/]*|[^/]+\.(ya?ml|env|sh|bash|zsh|properties|ini|toml|conf|cfg|tfvars|xcconfig)"
    r"|Dockerfile[^/]*|Makefile|Fastfile|Appfile|Matchfile)$",
    re.I,
)


def shannon(s):
    if not s:
        return 0.0
    counts = {}
    for ch in s:
        counts[ch] = counts.get(ch, 0) + 1
    n = len(s)
    return -sum(c / n * math.log2(c / n) for c in counts.values())


def high_entropy(value):
    if len(value) < 20 or IDENTIFIER_LIKE.match(value) or UUID.search(value):
        return False
    if value.startswith("/") or "://" in value or value.count("/") >= 2:
        return False
    # Identifiers, import paths and slugs are mostly dictionary-like runs; random keys are not.
    if len(WORDISH.sub("", value)) < 0.4 * len(value):
        return False
    if re.fullmatch(r"[0-9a-fA-F]+", value):
        return len(value) >= 32 and shannon(value) >= 3.0
    if not (re.search(r"[0-9]", value) and re.search(r"[A-Za-z]", value)):
        return False
    return shannon(value) >= 4.0


def mask(value):
    value = value.strip()
    if len(value) <= 8:
        return "*" * len(value)
    return f"{value[:4]}…{value[-2:]} (len {len(value)})"


def fingerprint(path, rule, value):
    return hashlib.sha256(f"{path}\0{rule}\0{value}".encode()).hexdigest()[:16]


def load_allowlist():
    allowed = set()
    if os.path.exists(ALLOWLIST):
        with open(ALLOWLIST, encoding="utf-8") as fh:
            for line in fh:
                token = line.split("#", 1)[0].strip()
                if token:
                    allowed.add(token.split()[0])
    return allowed


def skip_path(path):
    return path.startswith(SKIP_DIRS) or any(f"/{d}" in path for d in SKIP_DIRS) or bool(
        SKIP_FILES.search(path) or SKIP_EXT.search(path)
    )


def scan_line(path, lineno, line):
    found = []
    seen_spans = []

    def add(rule, value, span):
        if PLACEHOLDER.match(value.strip("\"'` ")):
            return
        if any(s[0] <= span[0] and span[1] <= s[1] for s in seen_spans):
            return
        seen_spans.append(span)
        found.append((rule, value))

    for rule, rx in PROVIDER_RULES:
        for m in rx.finditer(line):
            add(rule, m.group(0), m.span())
    for m in ASSIGNMENT.finditer(line):
        add("secret-assignment", m.group(1), m.span(1))
    if CONFIG_FILE.search(path):
        for m in YAML_OR_ENV_ASSIGNMENT.finditer(line):
            add("secret-assignment", m.group(1), m.span(1))
    if PULUMI_CIPHERTEXT.match(line):
        return [(path, lineno, rule, value) for rule, value in found]
    for m in QUOTED.finditer(line):
        if high_entropy(m.group(1)):
            add("high-entropy-string", m.group(1), m.span(1))
    for m in re.finditer(r"(?<![A-Za-z0-9+/=_@-])([A-Za-z0-9+/_-]{32,}={0,2})(?![A-Za-z0-9+/=_-])", line):
        if high_entropy(m.group(1)):
            add("high-entropy-token", m.group(1), m.span(1))
    return [(path, lineno, rule, value) for rule, value in found]


def tracked_files():
    out = subprocess.run(
        ["git", "-C", ROOT, "ls-files", "-z"], capture_output=True, check=True
    ).stdout
    return [p for p in out.decode().split("\0") if p]


def read_text(path):
    full = os.path.join(ROOT, path)
    try:
        with open(full, "rb") as fh:
            data = fh.read(2_000_000)
    except OSError:
        return None
    if b"\0" in data[:8000]:
        return None
    return data.decode("utf-8", errors="replace")


def scan_tree():
    results = []
    for path in tracked_files():
        if SENSITIVE_FILE.search(path) and not ENV_EXAMPLE.search(path) and not path.startswith(SELF_DIR):
            results.append((path, 0, "sensitive-file", path))
        if skip_path(path):
            continue
        text = read_text(path)
        if text is None:
            continue
        for i, line in enumerate(text.splitlines(), 1):
            results.extend(scan_line(path, i, line[:4000]))
    return results


def scan_history(since):
    log = subprocess.run(
        ["git", "-C", ROOT, "log", f"--since={since}", "-p", "-U0", "--no-color",
         "--format=commit %H", "--diff-filter=AM", "--no-merges"],
        capture_output=True, check=True,
    ).stdout.decode("utf-8", errors="replace")
    results = []
    commit = path = None
    lineno = 0
    for line in log.splitlines():
        if line.startswith("commit "):
            commit = line.split()[1][:12]
        elif line.startswith("+++ b/"):
            path = line[6:]
        elif line.startswith("@@"):
            m = re.search(r"\+(\d+)", line)
            lineno = int(m.group(1)) if m else 0
        elif line.startswith("+") and not line.startswith("+++") and path and not skip_path(path):
            for _, ln, rule, value in scan_line(path, lineno, line[1:4000]):
                results.append((f"{path}@{commit}", ln, rule, value))
            lineno += 1
    return results


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--history-since", help="also scan lines added in commits since this git date, e.g. '8 days ago'")
    ap.add_argument("--include-allowlisted", action="store_true")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()

    allowed = set() if args.include_allowlisted else load_allowlist()
    rows = scan_tree()
    if args.history_since:
        rows += scan_history(args.history_since)

    emitted = set()
    for path, lineno, rule, value in rows:
        fp = fingerprint(path.split("@", 1)[0], rule, value)
        key = (fp, path)
        if fp in allowed or key in emitted:
            continue
        emitted.add(key)
        if args.json:
            print(json.dumps({"fingerprint": fp, "path": path, "line": lineno, "rule": rule, "masked": mask(value)}))
        else:
            print(f"{fp} {path}:{lineno} {rule} {mask(value)}")
    print(f"# {len(emitted)} candidate(s), {len(allowed)} allowlisted fingerprint(s)", file=sys.stderr)


if __name__ == "__main__":
    main()
