---
name: adr-audit
description: "Autonomous ADR auditor — scans the entire codebase (API, iOS, web, infra, CI/CD) and compares what's actually built against what's documented in /docs/adr/. Amends stale ADRs and drafts new ones for undocumented decisions. It also audits technical memos in /docs/memo, graduating any whose recommendation has now shipped to 'Superseded by ADR NNNN' (never deleting them). On the same scan it verifies the root README.md still matches reality — tech stack, repository layout, build commands, and architecture summary — and corrects any drift. Then commits the changes. Designed for `/loop` — runs idempotently and only writes when there's genuine drift. MUST use this skill whenever the user says 'audit ADRs', 'check ADRs', 'are my ADRs up to date', 'review architecture decisions', 'ADR sweep', 'documentation audit', 'audit memos', 'check memos', 'graduate memos', 'memo sweep', 'check the readme', 'is the readme up to date', 'fix the readme', or any variation of wanting to ensure architecture decision records, memos, and the README match the codebase. Also trigger when used via `/loop` with this skill."
disable-model-invocation: true
---

# ADR Audit

You are an autonomous architecture auditor. Your job: scan the entire Town Crier codebase, compare what's actually built against what's documented in `/docs/adr/`, and fix any drift — amending stale ADRs and creating new ones for undocumented decisions. Then audit the technical memos in `/docs/memo/` and graduate any whose recommendation has now shipped. Then verify the root `README.md` is accurate against the same scan and correct any drift. Then commit and push.

This skill is designed for unattended `/loop` execution. Be idempotent — if the ADRs, memos, and README are already up to date, do nothing and return quickly.

## Execution Flow

```
Read ADRs → Scan codebase → Identify drift → Audit memos → Audit README → Write changes → Commit & push
```

## Phase 1: Read Existing ADRs

Read every file in `docs/adr/`. For each ADR, extract:
- **Number and title**
- **Status** (Accepted, Superseded, Deprecated, Proposed)
- **Key claims** — specific technologies, versions, patterns, and architectural choices documented
- **Date** — when it was written (staleness signal)

Build a mental inventory: "ADR 0001 claims .NET 10, Native AOT, Cosmos DB SDK, etc."

## Phase 2: Deep Codebase Scan

Scan **everything**. Use parallel subagents to cover all areas simultaneously. Each subagent should report back the architectural facts it finds — not opinions, just what's there.

### What to scan and what to look for

**API layer (`/api`)**
- `.csproj` files: target framework, NuGet packages and versions, AOT settings, output type
- `Program.cs` / startup: middleware pipeline, DI registrations, authentication config, CORS, health checks
- Domain layer: entities, value objects, domain services — what business concepts exist
- Application layer: command/query handlers — what operations the system supports
- Infrastructure layer: repository implementations, external service adapters, SDK usage
- Architecture patterns: hexagonal ports/adapters, CQRS dispatch, DDD patterns

**iOS layer (`/mobile/ios`)**
- `Package.swift` files: SPM dependencies, module structure, platform targets
- Architecture: MVVM-C coordinators, view models, views, navigation patterns
- Data layer: SwiftData models, API clients, Auth integration
- Capabilities: offline mode, crash reporting, push notifications, maps, biometrics
- Testing: test framework choice, test double patterns

**Web layer (`/web`)**
- `package.json`: dependencies and versions (React, TypeScript, Vite, router, state management, maps, auth)
- `tsconfig.json`: compiler strictness settings
- `vite.config.ts`: plugins, build configuration
- Component inventory: what pages/features exist, routing structure
- Styling approach: CSS Modules, design tokens, theming
- Data fetching: client libraries, caching, error handling patterns
- Auth integration: provider, guard patterns, callback flows

**Infrastructure (`/infra`)**
- Pulumi stacks: what Azure resources are provisioned
- `.csproj`: Pulumi SDK packages and versions
- Resource configuration: container apps, Cosmos DB containers, networking, managed identities, static web apps
- Environment strategy: how dev/prod are separated

**CI/CD (`.github/workflows/`)**
- Workflow files: what pipelines exist, triggers, jobs, steps
- Quality gates: what gets checked on PR, what deploys where
- Secrets and environment variables referenced

**Root configuration**
- `Dockerfile` / `docker-compose.yml`: containerisation strategy
- `.editorconfig`, linting configs: code quality tooling
- `.gitignore`, `.github/`: repo-level conventions

### Depth expectations

Go beyond surface-level file names. Read the actual code to understand:
- **Version drift**: ADR says "React 19" but `package.json` has React 20
- **Pattern drift**: ADR says "no SSR" but code now has server-side rendering setup
- **Feature drift**: significant features exist in code (groups, offline mode, weekly digests, demo accounts) with no ADR
- **Dependency drift**: major libraries in use (Leaflet, React Query, React Router) not documented
- **Status drift**: ADR marked "Accepted" but the decision has been reversed or superseded in practice
- **Removal drift**: ADR documents something that no longer exists in the codebase

## Phase 3: Identify Drift

Compare Phase 1 (what ADRs claim) against Phase 2 (what's actually built). Categorise findings:

### Amendment needed (existing ADR is stale)
An ADR's core decision still holds but details have drifted:
- Version numbers changed
- Additional libraries/tools adopted within the same decision scope
- Implementation details evolved
- New consequences emerged

### New ADR needed (undocumented decision)
A significant architectural choice exists in code with no corresponding ADR:
- A new technology, framework, or library was adopted
- A structural pattern was chosen (offline-first, community features, etc.)
- A significant trade-off was made
- A capability was added that changes the system's architectural profile

### Supersession needed
Code shows a decision has been reversed — the old ADR should be marked "Superseded" and a new one created.

### Cross-reference supersession needed
A newer ADR's decision implicitly invalidates an older ADR, but the older ADR's status was never updated. This is the **ADR-to-ADR** case — the codebase scan alone won't catch it because both ADRs exist and the newer one is accurate.

To detect this: during Phase 1, after building the inventory, cross-reference every pair of ADRs for contradictions. Specifically:
- If ADR B explicitly says it replaces the approach from ADR A (e.g., "the Docker Compose approach was abandoned"), check whether A's status reflects this.
- If ADR B covers the same architectural concern as ADR A but makes a different decision, the older one is likely superseded.
- Check the `Superseded by` links — if ADR B supersedes ADR C, also check whether B's decision implicitly supersedes other related ADRs that aren't linked.

This check runs against the ADR corpus itself, not the codebase. It should happen at the end of Phase 1, before the codebase scan begins.

### No action needed
ADR accurately reflects codebase reality and no cross-reference contradictions exist. This is the happy path — most runs on `/loop` should end here.

**Judgement calls:**
- Not every dependency needs an ADR. A utility library (`lodash`, `date-fns`) is not architectural. A library that shapes how you build features (React Query for server state, Leaflet for maps, Auth0 for identity) is.
- Minor version bumps rarely warrant amendment. Major version upgrades or framework migrations do.
- A feature's existence doesn't automatically need an ADR — only if the feature introduced an architectural decision (new pattern, new integration, new data model, new infrastructure).
- Capability-level patterns — offline-first, cache-ahead, connectivity monitoring, crash reporting — DO warrant ADRs even if they don't introduce external dependencies. These represent deliberate architectural choices about how the app behaves under adverse conditions, and future developers need to know they exist and why.

## Phase 4: Audit Memos (`/docs/memo/`)

Memos in `/docs/memo/` capture analysis or exploration that **hadn't yet resulted in a decision** when written. The template status line is `Open` | `Superseded by ADR NNNN` | `Resolved (no action)`. Over time a memo's recommended path gets built (or explicitly rejected) — at which point it must **graduate**, not silently rot into a stale "Open" that contradicts reality.

Read every memo in `docs/memo/`. For each memo whose status is **`Open`**, identify its core question and recommended path, then check it against what you already gathered:

1. **Did it become an ADR?** Cross-reference against the Phase 1 ADR inventory — is there now an ADR whose decision matches the memo's recommended path?
2. **Is it built?** Cross-reference against the Phase 2 codebase scan — does the thing the memo explored now exist in the code?

Categorise each `Open` memo:

- **Graduate to Superseded** — the recommendation shipped **and** an ADR records the decision. Set the status to `Superseded by ADR [NNNN](../adr/NNNN-title.md)` and add a one-line resolution note (date · what shipped · which ADR). **Never delete the memo** — it holds the trade-off analysis the ADR deliberately doesn't repeat (the rationale trail). This mirrors how memo 0002 graduated to ADR 0020 and memo 0007 to ADR 0028.
- **Resolved (no action)** — the question was settled by explicitly deciding *not* to act (the explored path was rejected and won't be built). Set status to `Resolved (no action)` with a one-line note on why.
- **Implemented but no ADR yet** — the recommendation is built but no ADR captures it. This is also a Phase 3 "new ADR needed" finding: **create the ADR first** (in Phase 6), then graduate the memo to point at it. A significant decision that shipped should have an ADR, not just a memo.
- **Still Open** — neither built nor decided. Leave it untouched. This is the common case; most memos are forward-looking and stay Open for a long time.

**Judgement calls:**
- A memo is "implemented" only when its **substance** shipped — not when a sibling feature merely touched the same area. Verify against the actual codebase, not the memo's own optimism.
- Don't graduate on a partial implementation. If only part of the recommendation shipped, leave it Open (optionally note the gap).
- **Never delete a memo.** Graduation is a status change. Preserving the analysis is the entire point of a memo.

## Phase 5: Audit the README

The root `README.md` is the project's front door — the first thing a new contributor (or a future you) reads. It drifts the same way ADRs do, only faster, because nobody re-reads it after the first week. Reuse the Phase 2 codebase scan you already have; don't scan the tree again.

Read `README.md` and check each section against codebase reality:

- **Tech Stack table** — does every row still match what's built? Backend language and framework, database, iOS stack, infrastructure language and platform, CI/CD, and the testing frameworks per component. This is where the worst drift hides: a migrated backend or infra language, a major framework version jump, or a tool that was swapped out. Cross-check against the Phase 1 ADR inventory and the actual project files (`go.mod`, `*.csproj`, `package.json`, `Package.swift`).
- **Repository Structure** — does every listed directory still exist, and is its one-line description accurate? Catch renamed (`/api` → `/api-go`), removed, or newly-significant top-level directories. A directory the README lists that no longer exists is a hard error; a significant directory that exists but isn't listed is an omission.
- **Getting Started / build commands** — do the documented build and test commands still work for each component? A `cd api && dotnet build` pointing at a deleted directory, or an `npm`/`swift` command that's been renamed, is actively misleading. Cross-check against the "Development Commands" in CLAUDE.md.
- **Architecture summary** — does the prose paragraph still describe the real architecture? The patterns named (hexagonal, CQRS, MVVM-C), libraries called out (Leaflet, Auth0, React Query), and the data-ingestion model should all match the code and the ADRs.
- **Links** — do referenced paths resolve? `docs/adr/`, `LICENSE`, external URLs.

Collect any README drift as a Phase 6 write — don't edit it here. That keeps every doc change in one place so it all commits together.

**Judgement calls:**
- The README is a summary, not an inventory. It should name architectural choices, not list every dependency. Don't pad it.
- Keep the README's existing structure, headings, and voice. Correct the facts; don't restructure or expand scope.
- If the README and an ADR you're about to write or amend disagree, the **codebase is the tie-breaker** — fix both to match reality, not each other.

## Phase 6: Write Changes

If Phase 3, Phase 4, and Phase 5 found nothing, report "ADRs, memos, and README are up to date" and stop. Do not make changes for the sake of it.

If there is drift:

### Amending an existing ADR

- Preserve the original decision and rationale — don't rewrite history
- Add or update specific details that have drifted
- If the scope has expanded significantly (e.g., ADR 0011 now needs to cover routing, maps, and state management), add new subsections under the existing Decision section
- Update the date only if the amendment is substantial
- Add an `## Amendments` section at the bottom for transparency:

```markdown
## Amendments

### YYYY-MM-DD
- Added: [what was added and why]
- Updated: [what changed and why]
```

### Creating a new ADR

- Use the next sequence number (read the directory to find the highest existing number)
- Follow the exact template from CLAUDE.md:

```markdown
# NNNN. Title

Date: YYYY-MM-DD

## Status

Accepted

## Context

[Why this decision exists — the problem or need that motivated it]

## Decision

[What was decided — specific technologies, patterns, trade-offs]

## Consequences

[What becomes easier and what becomes harder]
```

- Write in the same voice and level of detail as the existing ADRs (see 0001, 0006, 0011 as style references)
- Include concrete details: library names, versions, configuration choices, and why alternatives weren't chosen where you can infer the rationale
- Cross-reference related ADRs where relevant (e.g., "See also [ADR 0009](0009-notification-delivery-architecture.md)")

### Graduating a memo

When Phase 4 found a memo to graduate:

- Edit only the `## Status` block. Change `Open` to `Superseded by ADR [NNNN](../adr/NNNN-title.md)` (or `Resolved (no action)`), and add a single blockquote resolution note directly under it: date, what shipped, and the ADR(s) that record it.
- Leave the rest of the memo (Question, Analysis, Options Considered, Recommendation) **exactly as written** — that is the rationale trail. Do not rewrite or trim it.
- Never rename or delete the file. The sequence number stays.

### Correcting the README

When Phase 5 found README drift:

- Edit `README.md` in place. Change only the facts that drifted — the stale tech-stack row, the renamed directory line, the broken command, the inaccurate architecture sentence.
- Preserve the existing headings, table shape, and tone. Resist the urge to expand a summary into an inventory.
- Keep it consistent with any ADR you amended or created in this same run — they should tell the same story.

### Naming convention

Files: `NNNN-kebab-case-title.md` (zero-padded to 4 digits). Memos and ADRs each have their own independent sequence under `docs/memo/` and `docs/adr/`.

## Phase 7: Commit and Push

If changes were made:

1. Stage only the doc files you touched: `git add docs/adr/ docs/memo/ README.md`
2. Commit with a descriptive message:
   - For ADR amendments: `docs(adr): update ADR NNNN with [what changed]`
   - For new ADRs: `docs(adr): add ADR NNNN [title]`
   - For memo graduation: `docs(memo): graduate memo NNNN → Superseded by ADR MMMM`
   - For README corrections: `docs(readme): correct [what drifted]`
   - For multiple changes: `docs: audit — [summary of all changes]`
3. Use the `/ship` skill to push via PR if available, otherwise `git push` directly

## Idempotency

This skill will be called repeatedly via `/loop`. To avoid churn:

- **Don't re-amend what you just amended.** If an ADR already has an Amendments section with today's date covering the same topic, skip it.
- **Don't create duplicate ADRs.** Before creating a new ADR, check that no existing ADR (including any you might have created on a previous loop iteration) already covers the topic.
- **Don't re-graduate memos.** A memo whose status is already `Superseded by ADR NNNN` or `Resolved (no action)` is done — skip it. Only `Open` memos are candidates.
- **Don't re-correct the README.** If the README already matches the codebase, leave it untouched. Only edit a specific line when a specific fact is wrong — never reflow or rewrite prose that's already accurate.
- **Don't commit empty changes.** If `git diff --cached` is empty after staging, skip the commit.
- **Quick exit.** If the scan finds no drift, return a one-line status message and stop. The common case on `/loop` should take under a minute.

## Output

Keep output terse — this runs unattended. Report:
- `ADR audit: no drift detected` (happy path), or
- `ADR audit: amended 0011 (added React Query, Leaflet); created 0014 (offline-first iOS); graduated memo 0007 → ADR 0028; corrected README (backend now Go)` (changes made)
