# 0011. Beads/Dolt write-thrash: root cause and durable fix

Date: 2026-06-26

## Status

Open. The recommended keystone fix (commit `export.auto: false` / `export.git-add: false` into
`.beads/config.yaml`, bead `tc-okn8`) ships in the same PR as this memo. The defence-in-depth guard
(bead `tc-53wo`) is a follow-up. May graduate to an ADR if the owner wants the "pin bd 1.0.4 + commit
anti-thrash config" decision formalised. Anchor issue: [#650](https://github.com/AmyDe/town-crier/issues/650).

## Question

bd writes (`bd close`, `bd update`) intermittently — and in this repo's configuration *deterministically* —
revert moments after they succeed. A just-closed bead reads back as `open`; a notes edit disappears. Some
commands print `auto-importing N issues into empty database` on every invocation, including read-only ones.
What is the exact mechanism, every way it gets re-armed, and what is the one durable fix?

This memo records the end-to-end mechanism, a deterministic repro proven on a throwaway database, every
observed trigger path, the upstream bug status, and a recommendation. It supersedes the scattered notes in
the auto-memory `project_bd_thrash_and_105_breakage` with a single verified account.

## Analysis

### Environment

- bd **1.0.4** (`~/.local/bin/bd`, `ce242a879`), **server mode**, Dolt backend, DoltHub remote `amyde/town-crier`.
- `.beads/issues.jsonl` is **gitignored** in this repo (`.gitignore:107`, untracked since 2026-05-17 to stop
  merge conflicts) and currently archived out of `.beads/` entirely.
- The thrash was reproduced on a **throwaway** server-mode workspace (`bd init --server --prefix rp`) outside
  the repo. The live database was never touched.

### Mechanism, end to end

1. **bd runs the JSONL auto-import on (almost) every command.** In server mode bd 1.0.4 calls the auto-import
   path (`maybeAutoImportJSONL`, `cmd/bd/main.go:1025`) for any command that takes the non-read-only store
   path. The server-mode guard (`shouldRunAutoImportJSONL`) that exists on `main` was **not** in the 1.0.4
   tag, and `GetStatistics` misreads the populated server DB as empty (`TotalIssues == 0`), so the
   "empty database" guard never fires. The `auto-importing N issues into empty database` line is a hard-coded
   printf, **not** a real emptiness check (upstream #4245, #3849).

2. **The import is an additive upsert, not a replace.** It runs `INSERT … ON DUPLICATE KEY UPDATE` over the
   jsonl. Consequences, all confirmed empirically:
   - A row **in the DB but absent from the jsonl** is **not** deleted — it survives. (A bead created *after*
     the jsonl froze was never lost.)
   - A row **in both** whose field value **diverges** → the jsonl's stale value **overwrites the DB**. This is
     the clobber.
   - An **internally inconsistent** stale record is **rejected** with a non-fatal warning (e.g. a hand-edited
     `status:"open"` that still carries `closed_at` →
     `validation failed … non-closed issues cannot have closed_at timestamp`), so that clobber does not land.
     A *real* stale export is internally consistent, so it does land.

3. **The on-disk jsonl goes stale and stays stale — this is what makes the revert permanent.** Because
   `.beads/issues.jsonl` is gitignored, bd's auto-export writes a temp file and then `git add`s the real
   path, which fails:
   `Warning: auto-export: git add failed: … The following paths are ignored by one of your .gitignore files:
   .beads/issues.jsonl … Use -f`. The on-disk jsonl therefore never catches up to a new write. (Separately, in
   server mode the write-time export was observed *not* refreshing the file at all across many writes — it
   froze at an old snapshot.) A frozen, internally-consistent, divergent jsonl + an import-triggering command
   = a deterministic, repeatable revert.

4. **Which commands trigger the import.** Measured on the throwaway DB:

   | Command | auto-import fires? |
   |---|---|
   | 1st `bd create` (DB genuinely empty, no jsonl yet) | no |
   | 2nd+ `bd create` (jsonl now present) | **yes** |
   | `bd show` / `bd list` / `bd ready` (pure reads) | no |
   | `bd stats` (read-only *semantically*, but not flagged read-only) | **yes** |
   | `bd update` / `bd close` (writes) | **yes** |

   The trigger is "the command takes the non-read-only store path **and** `.beads/issues.jsonl` exists." The
   important subtlety: **a read-only command alone can clobber** — `bd stats` reverts a divergent field even
   though it writes nothing. Pure reads (`show`/`list`/`ready`) cannot.

### Deterministic repro (throwaway DB — never run against the live repo)

```bash
mkdir -p /tmp/bd-repro && cd /tmp/bd-repro && git init
~/.local/bin/bd init --server --prefix rp --skip-agents

~/.local/bin/bd create --title "First"     # -> rp-50t  (no auto-import: DB was empty)
~/.local/bin/bd create --title "Second"    # -> rp-xbm  ("auto-importing … into empty database" fires)
~/.local/bin/bd close rp-50t               # DB: rp-50t CLOSED

# Freeze a divergent BUT internally-valid jsonl: set rp-50t back to status "open" and drop closed_at.
# (Edit the single rp-50t line of .beads/issues.jsonl with python/jq.)

~/.local/bin/bd show  rp-50t   # -> CLOSED   (pure read; no import)
~/.local/bin/bd stats          # -> "auto-importing … into empty database / auto-imported N issues"
~/.local/bin/bd show  rp-50t   # -> OPEN     ← reverted. The clobber. Repeatable.
```

Re-closing and re-staling the jsonl reverts it again on the next `bd stats`. The same reverts a numeric
field (DB priority P0, stale jsonl P4 → `bd stats` → DB reads back P4).

### Every observed trigger / re-arm path

1. **Default config regenerates the jsonl.** With `export.auto: true` (the bd default), writes create and
   maintain `.beads/issues.jsonl`. From the 2nd command on, every non-read-only command re-imports it.
2. **The on-disk jsonl freezes** (gitignored → `git add` fails silently; and server-mode export not
   refreshing the file) → divergence becomes permanent rather than transient.
3. **`git reset --hard origin/main` re-arms it.** The anti-thrash config has lived only as an *uncommitted*
   local edit to `.beads/config.yaml`. The committed file in `origin/main` has no `export.auto`/`export.git-add`
   keys, so bd defaults them to `true`. Every mandated post-merge `git fetch && git reset --hard origin/main`
   discards the override → auto-export re-enables → the jsonl regenerates in the hot path → thrash returns.
   The archive directory shows the jsonl being re-removed **7 times on 2026-06-25 alone**; it bit again on
   2026-06-26.
4. **A stray bd on `PATH`** (#3948): a different bd version running a command clobbers, and a newer bd (1.0.5/
   1.1.0-rc.1) would also apply a hostile schema migration on first run.
5. **Worktree ops** were historically associated with thrash. They could not be re-reproduced in the
   throwaway sandbox (`bd worktree create` refuses from `/private/tmp` with
   `BEADS_DIR points to unsafe location`), but the mechanism is the same: any worktree command that runs while
   a divergent jsonl is present imports it. With `export.auto: false` and no jsonl, there is nothing to
   import, so the worktree path cannot clobber via this mechanism.

### Fix validation (decision-critical)

Tested on the throwaway DB in both directions:

| Configuration | Outcome |
|---|---|
| `export.auto: false` + `export.git-add: false`, **jsonl still present (stale)** | **Still clobbers.** Auto-*import* is independent of export config: a write stuck momentarily, then `bd stats` re-imported the stale jsonl and reverted it. **Config alone is insufficient.** |
| `export.auto: false` + **jsonl removed** | **Stable.** Every valid write stuck; no `auto-importing` line ever; the jsonl was **never** regenerated. |
| `export.auto: true` + jsonl removed | **Re-arms.** One write regenerated `.beads/issues.jsonl` → trap reloaded. **jsonl-removal alone is insufficient.** |

So a durable fix needs **both** halves, exactly as the live mitigation already does it: (a) `export.auto: false`
(+ `export.git-add: false`) to stop bd recreating/refreshing the jsonl, and (b) the jsonl physically absent
from `.beads/` to disarm any currently-loaded clobber. The jsonl is already archived out; the missing piece is
making half (a) survive `git reset` by committing it.

These are the correct keys for 1.0.4: `bd config show` reports `export.auto`/`export.git-add` with provenance
`(config.yaml)`. The `backup.*` block (`backup.enabled`/`interval`/`git-push`) is a separate off-machine
git-backup feature (defaults disabled) and is **not** the thrash lever — the thrash reads `export.path`
(`.beads/issues.jsonl`). Nothing under `backup:` needs touching, and the `BD_IMPORT_AUTO`/`BEADS_NO_AUTO_IMPORT`
env toggles do not work on the write path in 1.0.4 (#4304).

### Upstream bug status (gastownhall/beads)

| Issue | Topic | State |
|---|---|---|
| #3849 | server-mode auto-import has no emptiness guard (canonical) | Closed 2026-05-10 (before the fix actually merged) |
| #4170 | gate auto-import on server mode at the call site (**the fix**, PR) | **Merged to `main` 2026-05-26** |
| #4304 | `bd update` auto-imports despite `import.auto=false` env toggle | Open |
| #3948 | auto-import fires every command despite a non-empty DB | Open |
| #4128 | 1.0.4 write-path re-imports JSONL per call → OOM/lock-fight | Open (reopened) |
| #4245 | missing `serverMode` guard at `cmd/bd/main.go:1025` | Open |
| #4239 | shared-server: import overwrites live data every command | Open |
| #3880 | repeated "auto-import … empty database" on every update | Open |
| #4331 | concurrent-mutate ephemeral-import race clobbers field edits | Open |
| #4038 | prune/purge no-op when jsonl git-excluded; import reverts | Closed 2026-05-22 |
| #3421 | `bd worktree create` "database not found" regression | Closed 2026-04-29 |
| #3593 | worktree `.beads/` left at 0755 (chmod warning) | Open |

**Is the fix in a safe release? No.** The #4170 merge commit (`4990c83`) is 223 commits ahead of `v1.0.4`, so
it is confirmed not in 1.0.4. The only refs after 1.0.4 both carry it together with a migration this DB cannot
safely take:

- **v1.0.5** — a bare tag, no published GitHub release. Carries #4170 **and** migrations `0040`–`0049`
  (the `wisps`-lineage migration that corrupted this DB: `0040_ignored_tables_also_nonlocal_tables`,
  `0047_recompute_mixed_is_blocked`). Not safe.
- **v1.1.0-rc.1** — published prerelease. Carries #4170 **and** `0050_dependencies_deterministic_id` (+0051–0053)
  — the migration that can make multi-clone histories un-mergeable. Not safe.

There is no released bd `> 1.0.4` that carries the fix without also carrying a hostile migration. **Upgrading is
not a viable path today.**

## Options Considered

- **A — Commit `export.auto: false` + `export.git-add: false` into `.beads/config.yaml`** (bead `tc-okn8`).
  Stops bd regenerating/refreshing the jsonl, and crucially survives `git reset --hard origin/main` and fresh
  clones (a clone has the committed config and, because the jsonl is gitignored, no jsonl to import). Removes
  the single biggest residual re-arm path (trigger path 3). Trade-off: relies on bd honouring committed config
  keys in server mode — verified it does (`(config.yaml)` provenance). Cost: one config edit.

- **B — Upgrade to a bd carrying #4170.** Would fix the root cause in bd itself. **Rejected:** no safe release
  carries #4170 without a migration that corrupts this DB (v1.0.5) or breaks multi-clone merges (v1.1.0-rc.1).
  Revisit only when a clean release ships the fix.

- **C — A guard (pre-commit / CI / `bd doctor`-style) that re-asserts config and deletes a stray jsonl**
  (bead `tc-53wo`). Defence-in-depth against the residual paths A doesn't cover: a jsonl reappearing (a clone
  that carried one, a future bd op) or the config keys being edited back. Cheap; complements A rather than
  replacing it. Trade-off: a hook is one more moving part to maintain.

- **D — Disable jsonl generation more deeply** (env/path tricks). Subsumed by A: `export.auto: false` already
  stops generation, and `BD_IMPORT_AUTO`/`BEADS_NO_AUTO_IMPORT` are no-ops on the write path in 1.0.4. No extra
  lever worth pulling.

## Recommendation

**Adopt A as the durable keystone fix, with C as defence-in-depth, and keep bd 1.0.4 pinned (B rejected).**

A is decisive on its own for normal operation: with `export.auto: false` committed, bd never regenerates the
jsonl, the hot path stays empty, there is nothing to import, and `git reset` can no longer flip the keys back —
the re-arm loop that has bitten repeatedly is closed. Combined with the already-archived jsonl, the trap is
fully disarmed. C guards the long tail (a stray jsonl or an edited-back config). Upgrading (B) stays off the
table until a bd release ships #4170 without a hostile migration; until then the pin on `~/.local/bin/bd`
1.0.4 and the "never `brew install beads`" rule remain load-bearing.

This PR ships A (the committed config) alongside this memo. C is filed as `tc-53wo`.

### Guardrails (unchanged, restated)

Never `brew install beads`; never upgrade off 1.0.4; never re-create `.beads/issues.jsonl`, re-track it, or set
`export.auto`/`export.git-add` back to `true`; never `bd export` to the default path. Sync is Dolt only
(`bd dolt push`/`pull`). Treat the bd DB as fragile — experiment on a copy.
