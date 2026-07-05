# 0014. Long-term management of monorepo CI/CD and infrastructure

Date: 2026-07-05

## Status

Open

## Question

The monorepo now spans six deployable components (Go API + worker, Go CLI, iOS, Android, web, Pulumi infra) with conditional deployments across nine workflows. Purpose-built monorepo tooling exists — build orchestrators, affected-graph engines, programmable CI. Constrained to free (ideally open-source) tooling only: should we adopt any of it for the long term, or keep investing in the hand-rolled GitHub Actions + Pulumi stack? And either way, what needs hardening?

## Analysis

### Where we are

A full audit of the CI/CD surface (July 2026):

- **2,653 lines of CI YAML**: 2,016 across 9 workflows (38 jobs), plus 637 across 9 local composite actions.
- **~3,690 lines of Pulumi Go** in effectively two files (`environment.go` 926, `shared.go` 673), ~45 resource declarations, three stacks (shared/dev/prod).
- **Four independent change-detection mechanisms in five locations**, with no shared source of truth:
  1. `.github/actions/detect-changes` — a composite action mapping path prefixes to category booleans via a hardcoded `case` statement. Used only by pr-gate.
  2. `cd-dev.yml`'s trigger-level `paths-ignore` list (hand-maintained inverse logic, disagrees with #1: a `cli/`-only commit triggers a full dev deploy nothing needs).
  3. `cd-ios-testflight.yml`'s guard: raw `git diff --quiet <prev-tag>..<tag> -- mobile/ios/`.
  4. `legal-drift-check.yml`'s native `paths:` filter.
- **The dangerous failure mode is silent**: a new top-level directory falls through the `case` statement in #1, so pr-gate runs *zero* validation jobs for it (the aggregate `gate` job still passes, because skipped ≠ failed) — while cd-dev still runs a full unconditional deploy for it. Nothing fails loudly; nothing asserts the five lists agree.
- **Remaining copy-paste** (most mechanical duplication is already factored into composite actions): the iOS/Android release-note guards (~40 near-identical lines in pr-gate), `migrate-dev`/`migrate-prod` (byte-identical except the DB name), and the two ~35-line digest-resolution blocks in cd-prod.
- **Magic strings synced by tribal knowledge**: Container App/job names, image repo names, storage-account and SWA names each appear independently in workflows and `infra/*.go`; one composite action's header still points at the pre-Go-rewrite `EnvironmentStack.cs`.
- **Pulumi strain points**: dev/prod divergence expressed as ~8 scattered `if env == "prod"` conditionals rather than one table; a manually snapshotted 15-entry Cloudflare IP allowlist with no staleness check; the Cosmos-named identity frozen by URN stability.

What the audit also confirmed is that the *architecture* is already the industry-standard one. The detect-changes-job + `if:` gating + always-run aggregator `gate` job is literally GitHub's own documented recommendation for required checks in path-filtered monorepos ([GitHub docs: handling skipped but required checks](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/troubleshooting-required-status-checks#handling-skipped-but-required-checks)), and the community's "bucket job" pattern ([discussion #44490](https://github.com/orgs/community/discussions/44490), still the standing answer — GitHub has shipped no first-party fix since acknowledging it in 2024). OIDC-federated logins, digest-pinned prod deploys with post-deploy verification, and durable release tags are all ahead of typical practice at this scale. The problem is not the pattern; it is that the pattern's inputs (path lists, resource names) are duplicated without enforcement.

### What the market offers (surveyed July 2026)

Every candidate was assessed against the constraint that actually bites: this repo is **polyglot across Go, Swift, Kotlin, and TypeScript**, with the iOS lane requiring Xcode + fastlane on macOS runners.

| Tool | Licence | Polyglot fit for this repo | Verdict |
|------|---------|---------------------------|---------|
| Bazel | Apache 2.0 | rules_go mature; rules_swift/Xcode integration is where small teams bleed | Fixed cost measured in person-months ("~1 person-decade" is the community's number for full rollout); an [ICSE 2024 study](https://dl.acm.org/doi/10.1145/3597503.3639169) found 11.2% of 542 adopting projects later abandoned it, median 638 days in; [Anki ripped it out](https://github.com/ankitects/anki/commit/5e0a761b875fff4c9e4b202c08bd740c7bb37763) after ~2 years of "build system upkeep" eating development time. **No.** |
| Nx | MIT core; Nx Cloud SaaS | Only first-party non-JS plugin is Gradle, explicitly experimental; Go is a community plugin; [no native-Android support](https://nx.dev/blog/android-and-nx) ("isn't technically fully supported"); no Swift/iOS anything | 2024–26 licence whiplash: free self-hosted caching removed (v20), restored under pressure (Mar 2025), [deprecated again May 2026](https://emilyxiong.medium.com/exploring-of-nx-self-hosted-cache-5bc39bd2ed7f) steering users to paid Nx Cloud. Plus the Aug 2025 [s1ngularity supply-chain attack](https://nx.dev/blog/s1ngularity-postmortem). For our stack it degrades to a task-runner wrapping shell commands. **No.** |
| Turborepo | MIT (Vercel) | JS-only by design; non-JS code must be wrapped in `package.json` scripts inside a JS workspace; the [language-agnostic RFC](https://github.com/vercel/turborepo/issues/683) has sat open since 2022 | Would accelerate `/web` only. **No.** |
| moonrepo (moon) | MIT | The interesting one: Go is genuinely first-class ([tier 3 since v1.38](https://moonrepo.dev/blog/moon-v1.38), reads go.mod/go.work); but Swift/iOS is not supported *at all* (not even named in docs), Kotlin/Android is generic-shell-tier only | Company is a wound-down 2-person YC startup (status "Inactive", paid product discontinued 2025); OSS ships weekly but bus factor ≈ 2 and ~50k weekly downloads vs Nx's ~5M. **No** — covers exactly the half of the repo that least needs help. |
| Pants / Buck2 / Please | Apache 2.0 | Pants' documented small-team successes are all Python-only shops; Buck2 has no small-team adoption story in public existence; Please is niche | **No.** |
| Dagger | Apache 2.0 | Healthy (16k stars, active, runs free on standard GitHub runners with no paid Cloud dependency) — but the engine is container-based, and **Xcode/fastlane/signing cannot run in Linux containers**, so the iOS lane stays in plain Actions regardless, splitting CI across two paradigms | Also mid-pivot toward AI-agent infrastructure; HN consensus flags scope drift. **No** — would replace half the YAML while adding an SDK, an engine dependency, and a second mental model. |
| Earthly | — | — | **Dead.** The company [stopped maintaining the OSS project in April 2025](https://earthly.dev/blog/shutting-down-earthfiles-cloud/) after killing its SaaS twice; the community fork (EarthBuild) is 167 stars vs the original's 12k. The definitive cautionary tale for betting CI on a VC-funded build-tool startup. |

Cross-cutting evidence from the same survey:

- The threshold heuristic in current (2025–26) practitioner writing: affected-graph tooling starts paying at roughly **5–15 packages with shared internal libraries**. This repo has six components with almost no cross-component build coupling (the API/CLI don't even share a Go module) — below the line on both axes.
- Where orchestrator adoption succeeded at small scale, it was single-language repos or teams that budgeted person-months with a designated build-system owner. Neither describes a solo maintainer spanning four language ecosystems.
- The AI-agent angle cuts *toward* plain Actions, not away: agents author either format with equal ease, but when something breaks, GitHub Actions YAML is among the best-represented formats in public training data, while Starlark/bzlmod/Nx-plugin failure modes are niche. The debugging tax, not the authoring tax, is what sank the documented regret cases.
- For the change-detection problem specifically, the standalone tooling landscape is thin and settled: [`dorny/paths-filter`](https://github.com/dorny/paths-filter) (MIT, 3.2k stars, release shipped this week) is the industry-standard directory-boolean tool, and [`digitalocean/gta`](https://github.com/digitalocean/gta) (Apache 2.0, alive, release Feb 2026) does transitive affected-package selection for Go only. Nothing credible does cross-language affected graphs without full framework buy-in. Our home-grown `detect-changes` action already does what paths-filter does, minus its supply-chain exposure — the gap is coverage and enforcement, not capability.
- Supporting tools verified healthy and MIT/free: [`actionlint`](https://github.com/rhysd/actionlint) (workflow correctness + shellcheck of `run:` blocks) and [`zizmor`](https://github.com/zizmorcore/zizmor) (Actions security audit: template injection, unpinned actions, excessive permissions; Trail of Bits-hardened 2026). Complementary, not overlapping.
- Merge queue: unavailable to us (org-owned repos only; this is a personal-account repo) and pointless for a solo committer anyway — it exists to serialise concurrent PRs from multiple authors.
- The March 2025 [`tj-actions/changed-files` compromise](https://www.cve.org/CVERecord?id=CVE-2025-30066) (release tags rewritten via a stolen PAT to exfiltrate CI secrets, 23k+ repos exposed) is the standing argument for pinning third-party actions to full commit SHAs and preferring first-party/home-grown where trivial.

### Infrastructure (Pulumi) side

- **Pulumi Cloud's Individual tier is free with unlimited projects/stacks/resources for one user** ([pricing](https://www.pulumi.com/pricing/)). The cost trigger is adding a second collaborator (Team tier, $40/mo), not growth. The CLI/SDK remain Apache 2.0 — no Terraform-style licence event has occurred.
- **Escape hatch exists and is mature**: DIY backend (`azblob://` + Azure Key Vault secrets provider) with built-in per-stack locking. Migration is a supported export/import, mildly fiddly. No reason to move today; worth knowing the door is open.
- **Drift detection**: Pulumi Cloud's native drift UI is Enterprise-gated, but a scheduled `pulumi preview --refresh --expect-no-changes` in Actions is the [documented pattern](https://www.pulumi.com/blog/patterns-drift-detection/), free, and effectively the only option (no OSS drift tool supports Pulumi state; driftctl is Terraform-only and in maintenance mode). This repo is public, so Actions minutes are free.
- **Program structure**: 1,600 lines in two files is below Pulumi's own [stated triggers for splitting into micro-stacks](https://www.pulumi.com/docs/iac/guides/basics/organizing-projects-stacks/) (hundreds of resources, per-concern files, divergent deploy cadences). When splitting does become warranted, component resources first, then separate projects with stack references; `aliases` make refactors non-destructive if each move is previewed.
- **CrossGuard policy packs are free OSS** (`--policy-pack` on preview/up, no Cloud dependency) — unused upside for encoding invariants like "prod min-replicas ≥ 1" and "no public blob access".
- **The real lock-in is Go, not Pulumi**: mature conversion tooling exists only *into* Pulumi, not out, and no other IaC tool offers Go as a first-class authoring language (OpenTofu means an HCL rewrite). Accept this: it is the price of infra code in the same language as the backend, and the escape hatch (OpenTofu, now CNCF-hosted and genuinely mature) exists if the ecosystem ever forces it.

## Options Considered

1. **Adopt a build orchestrator (Bazel, Nx, moon, Pants, Buck2).** Rejected. Every candidate fails the polyglot test for this specific stack — the tools that understand Go don't understand Swift, and none understand Xcode/fastlane. The repo would carry an orchestrator's fixed maintenance cost to accelerate, at best, one or two of six components, with documented odds (11.2% for Bazel) of an expensive later retreat.
2. **Adopt Dagger (CI as typed code).** Rejected for now. The one candidate that is genuinely healthy, genuinely free, and philosophically appealing for a Go shop — but it cannot touch the macOS/Xcode lane, so it would *add* a paradigm rather than replace one, and its scope is visibly still moving. Worth re-reading in a year if the YAML surface doubles.
3. **Generate workflow YAML from a program (cue/jsonnet/ytt, checked-in output).** Rejected. It solves duplication by adding an indirection layer only one person understands, and the duplication that remains after the fixes below is small. actionlint covers the correctness gap more cheaply.
4. **Split the monorepo.** Rejected without much agonising: conditional CI is a solved problem here, and a monorepo is the right shape for agent-driven cross-cutting work (atomic renames, one dependency graph, no PR trains).
5. **Keep the hand-rolled GitHub Actions + Pulumi stack; close the specific gaps the audit found.** Recommended.

## Recommendation

Stay on hand-rolled GitHub Actions + Pulumi Cloud (free tier). The current architecture is the documented best practice for exactly this shape of repo, and every alternative surveyed either can't cover the stack, isn't free in the ways that matter, or carries fixed costs sized for teams of fifty. The long-term management strategy is not a tool migration; it is closing the enforcement gaps that make the current system fragile, in roughly this order:

1. **Make `detect-changes` the single source of truth, and make it fail closed.** Add a final `case` arm that treats any changed path *not* matching a known prefix (or an explicit allowlist: `docs/`, `.beads/`, `.claude/`, `scripts/`, `*.md`, …) as "unknown → force all categories true". A new top-level directory then over-builds instead of silently skipping validation. This is the single highest-value change.
2. **Consume it everywhere.** cd-dev: drop the trigger-level `paths-ignore` and gate jobs (or short-circuit the run) with the same action in push mode, keeping an explicit `workflow_dispatch` deploy-everything override. cd-ios-testflight: extend the action with a tag-range mode and replace the hand-rolled guard. legal-drift-check's four-line `paths:` filter can stay — it is not a required check and duplicates nothing structural.
3. **Assert coverage in CI.** A trivial pr-gate step: list top-level directories, fail if any is neither mapped to a category nor on the explicit ignore list. This turns "someone must remember five files" into "the build tells you".
4. **Lint the CI itself.** Add `actionlint` and `zizmor` to pr-gate (run only when `.github/**` changes, via the same detection). Pin the few third-party actions to full commit SHAs while doing it.
5. **Factor the last copy-paste blocks** into composite actions: release-note guard (platform parameter), pgmigrate job (database parameter), prod digest-resolve-and-deploy. Fix the stale `EnvironmentStack.cs` comment in `deploy-worker-jobs`.
6. **One source for resource names.** Either read them from `pulumi stack output` wherever a workflow already logs into Azure, or commit a single `names` file consumed by workflows with a Go test in `/infra` asserting it matches the `Sprintf` conventions. Kills the three-way hardcoding noted in seo-refresh's own apologetic comment.
7. **Add the free drift-detection cron**: scheduled workflow running `pulumi preview --refresh --expect-no-changes` per stack (daily is enough at this scale; minutes are free on a public repo). Fold in a weekly staleness check of the Cloudflare IP snapshot against `api.cloudflare.com/client/v4/ips` — the one hand-maintained list that can silently rot into an outage.
8. **Tidy `environment.go` opportunistically, don't restructure.** Gather the scattered dev/prod conditionals into one per-env config table read at the top of `runEnvironmentStack` (pure code motion, no URN changes). Extract component resources only when a subsystem next changes anyway. Do not split into micro-stacks yet; revisit when the resource count or deploy cadence actually diverges.
9. **Later / optional**: a small CrossGuard policy pack for the two or three invariants that matter; `digitalocean/gta` for affected-package Go testing if `api-go` CI time ever becomes the bottleneck; re-evaluate orchestrators only if the repo passes ~10+ components with genuinely shared internal libraries, which is not the current trajectory.

Standing watch-items, no action needed: Pulumi free tier survives until a second human collaborator appears; Nx/Dagger are the two tools whose trajectory could change this analysis; the `subscription-sweep` and `pg-purge` worker jobs appear in Pulumi but in neither CD deploy list — flagged separately for verification as a possible deploy gap.
