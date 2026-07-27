# 0046. Upgrade bd from pinned 1.0.4 to 1.1.2

Date: 2026-07-27

## Status

Accepted

## Context

`docs/memo/0011-beads-dolt-write-thrash-root-cause.md` pinned `~/.local/bin/bd` to 1.0.4 for two
stacked reasons:

1. bd 1.0.4's server-mode auto-import had no emptiness guard (`gastownhall/beads#3849`), so almost
   every write command re-imported `.beads/issues.jsonl` and could clobber a just-made write with a
   stale value. Worked around by committing `export.auto: false` / `export.git-add: false` and
   removing the jsonl from `.beads/`.
2. Every bd release after 1.0.4 known at the time carried Dolt schema migrations (`0040`–`0053`)
   the memo assessed as dangerous to this database: `v1.0.5` (unpublished tag) paired the #3849 fix
   with migrations `0040`/`0047`, and `v1.1.0-rc.1` added `0050_dependencies_deterministic_id`, which
   the memo flagged as able to fork multi-clone histories. The memo explicitly rejected upgrading
   (Option B) on this basis and recommended staying on 1.0.4 indefinitely.

Re-investigating on 2026-07-27 (prompted by a cloud-agent setup script failure unrelated to the pin
itself):

- `#3849` closed 2026-05-10; the fix (`#4170`/`#4091`) merged to `main` 2026-05-26 — well before
  `v1.1.0` (2026-07-04). The auto-import clobber the pin was originally built around is fixed
  upstream as of 1.1.
- The migration-corruption concern was re-checked against upstream issues, not just changelog
  copy. Two open bugs matched this repo's exact configuration (Dolt **server mode** against an
  **external dolt sql-server**): `gastownhall/beads#4800` (migration `0040` throws `nothing to
  commit` and permanently blocks every `bd` command, reproduced on dolt 2.1.10 — this repo's exact
  dolt version) and `#4176` (a clone landing mid-migration crashes on `0047`'s `wisps` table
  reference). Both were still open at the time of this ADR.
- Given that, the upgrade was tested directly against a synced copy of the live database rather
  than decided from documentation alone: `bd dolt push` to flush local state, a manual `bd export
  --all` snapshot taken as a rollback artifact, the pre-migration Dolt commit hash recorded, then
  `BD_ALLOW_REMOTE_MIGRATE=1 bd migrate` run with bd 1.1.2 as the sole/designated migrator (single-
  developer project, so the multi-clone fork risk `#4259` warns about doesn't apply).
- The migration completed cleanly: 21 schema steps (`v32` → `v53`) applied without the `#4800`
  failure, `bd doctor` reported 0 errors, and `bd stats` showed the same issue count (1453) as the
  pre-migration export. Pushed to the DoltHub remote (`amyde/town-crier`) immediately after.

## Decision

Move the pin from bd 1.0.4 to **bd 1.1.2**, installed at `~/.local/bin/bd` from the official
`gastownhall/beads` GitHub release binary (not `brew install beads` — still avoid unpinned drift).
The `export.auto: false` / `export.git-add: false` config and the "never re-add
`.beads/issues.jsonl`" rule in `docs/memo/0011` and this file both stay in force: the upstream fix
removes the *need* for the workaround, not the workaround itself, and it costs nothing to keep as
defence-in-depth.

`gastownhall/beads#4800` and `#4176` remain open upstream as of this writing. They did not
reproduce on this database, but they are real, filed-and-confirmed bugs in the exact server-mode /
external-dolt-sql-server configuration this repo uses. Re-check their status before the next
version bump rather than assuming a clean jump.

## Consequences

- `bd` commands on this repo now run against schema `v53`, not `v32`. Any other machine or
  ephemeral environment (cloud-agent sandboxes, future worktrees) that clones this repo must run
  bd **1.1.2 or later** — an old 1.0.4 binary opening this database now fails, since the schema has
  moved out from under it. The cloud-agent setup script must build/install 1.1.2, not 1.0.4.
- The "designated migrator" model (`gastownhall/beads#4259`) now applies going forward: if this
  project ever gains a second contributor cloning the Dolt-backed DB, only one clone should ever
  run `bd migrate` against a future schema bump — others should `bd bootstrap` (re-clone) instead.
  Solo-developer status was a precondition for running this upgrade directly against the live DB
  rather than rehearsing it on a throwaway copy first.
- CLAUDE.md's "stay on 1.0.4" and "never upgrade off 1.0.4" language is stale and is updated
  alongside this ADR.
