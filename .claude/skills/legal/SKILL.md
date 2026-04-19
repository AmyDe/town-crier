---
name: legal
description: "Autonomous UK GDPR auditor for Town Crier — scans the codebase to inventory personal data collected, third-party processors, cross-border data flows, and cookies, then compares against the live Terms of Service and Privacy Policy (served from `GetLegalDocumentQueryHandler.cs`). Produces a gap report, files a bead, updates the copy, and files separate beads for any code-level compliance gaps found. MUST use this skill whenever the user says 'legal audit', 'privacy audit', 'privacy review', 'GDPR check', 'compliance check', 'update terms', 'update privacy policy', 'data audit', 'check data processors', 'privacy policy is stale', 'terms are stale', '/legal', or any variation of wanting to ensure the Terms of Service and Privacy Policy accurately reflect the codebase and meet UK GDPR transparency requirements. Also trigger proactively whenever a new third-party integration, new personal data field, or new data flow is added to the codebase."
---

# Legal Audit

You are a UK GDPR data protection auditor for Town Crier. Your job: scan the codebase, figure out what personal data is actually collected and where it actually flows, then compare that against what the live Terms of Service and Privacy Policy say. File a bead, update the copy, commit via PR. Separately, file beads for any compliance gaps that need code-level fixes (not just copy updates).

## Jurisdiction & Scope

Town Crier is a UK-only product: the dataset covers UK planning applications only, the governing law is England & Wales, and the target audience is UK residents. **However**, the product does not geo-block sign-ups, so overseas users may register. Treat UK GDPR as the primary framework. Don't draft for EU GDPR, CCPA, or LGPD — if a user outside the UK signs up, UK GDPR still governs what we do with their data.

App Store and Play Store policy compliance is in scope only where it directly affects the privacy policy (e.g., Apple's subscription disclosures).

## Output Contract

This skill produces **three things**:
1. A gap report in the conversation (what drifts, what's missing)
2. Updates to `api/src/town-crier.application/Legal/GetLegalDocumentQueryHandler.cs` and the associated tests, committed via PR through a single bead that is created and closed in one pass
3. **Separate** beads for any finding that requires a code change beyond the legal copy (e.g., "add a cookie banner", "implement account deletion endpoint", "add data export endpoint"). Leave these beads open for later work.

If no drift is found, report `Legal audit: no drift detected` and exit. Do nothing.

## Where the Legal Copy Lives

The Terms of Service and Privacy Policy are **not** static files. They are served from a .NET handler:

- **File:** `api/src/town-crier.application/Legal/GetLegalDocumentQueryHandler.cs`
- **Served by:** `api/src/town-crier.web/Endpoints/LegalEndpoints.cs` at `GET /legal/{documentType}` where documentType is `privacy` or `terms`
- **Rendered by:** iOS (`mobile/ios/packages/town-crier-presentation/Sources/Features/Legal/`) and web (`web/src/features/legal/`)
- **Tests:** `api/tests/town-crier.application.tests/Legal/GetLegalDocumentQueryHandlerTests.cs`

All edits go in the handler file. Remember to:
- Update the `LastUpdated` date to today (the current session date, not hardcoded).
- Update tests that assert on section content — don't leave them stale.

## Execution Flow

```
Read legal docs → Parallel scan (data, processors, flows, cookies) → Gap analysis → Present findings → Create bead → Enter worktree → Edit copy → Run tests → Close bead → Ship → File code-gap beads
```

## Phase 1: Read Existing Legal Copy

Read `api/src/town-crier.application/Legal/GetLegalDocumentQueryHandler.cs` and extract:
- Every section heading and the verbatim body text for both Privacy Policy and Terms of Service
- The `LastUpdated` date (staleness signal)
- Which named third parties are already disclosed (e.g., "Microsoft Azure", "PlanIt", "Apple Push Notification Service")

Build a mental inventory: "The policy currently names X, Y, Z as processors; claims retention is N months; lists M user rights."

## Phase 2: Scan the Codebase

Run these scans in parallel using subagents. Each subagent reports facts, not opinions.

### 2a. Personal Data Inventory — `/api`

What personal data does the system collect, store, or process? Look at:

- **Domain entities** (`api/src/town-crier.domain/`) — user profiles, saved locations, notification preferences, devices. Record every field that could identify a person (name, email, device ID, IP, coordinates, postcode, phone).
- **DTOs and request models** in `api/src/town-crier.web/` — what do the API endpoints accept from clients?
- **Cosmos documents** in `api/src/town-crier.infrastructure/` — what shape is the data actually persisted in?
- **Observability** — are request bodies, user IDs, IP addresses, or user-agents logged? (App Insights, OpenTelemetry spans). If PII is in logs, that's a processing activity.
- **Auth0 claims** — what does the identity token carry? (email, sub, name, maybe more)

For each data category, record:
- **What**: the field name
- **Where it comes from**: user input, device sensor, third party, derived
- **Where it's stored**: Cosmos container + external system
- **Why**: the feature it supports (so you can articulate lawful basis)

### 2b. iOS Personal Data — `/mobile/ios`

- **SwiftData models** — what's stored on device?
- **Permissions requested** — location, notifications, camera? Check `Info.plist` and permission-request code.
- **Device identifiers** — APNs token, IDFA, IDFV?
- **Analytics / crash reporting** — is there any SDK that phones home with user data? (Crashlytics, Sentry, Firebase, Amplitude, PostHog, etc.)

### 2c. Web Personal Data — `/web`

- **Forms** — what do they collect?
- **Cookies / localStorage / sessionStorage** — grep for `document.cookie`, `localStorage.setItem`, `sessionStorage.setItem`. Cookies that aren't strictly necessary require consent under PECR.
- **Analytics** — Google Analytics, Plausible, PostHog, Hotjar? Grep for known SDK imports in `package.json` and `index.html`.
- **Third-party scripts** — anything loaded from a non-first-party domain.

### 2d. Third-Party Processor Inventory

A processor is any third party that handles personal data on our behalf. Scan for:

- **`package.json` / `Package.swift` / `.csproj`** — identify SDKs that integrate with remote services (auth, email, push, analytics, payments, maps, storage)
- **Environment variables and Pulumi config** (`infra/`) — API keys, connection strings, endpoints reveal which services are actually wired up in prod
- **Infra stack** (`infra/EnvironmentStack.cs` and siblings) — what Azure resources are provisioned? What region?
- **`Program.cs`** in `api/src/town-crier.web/` and `api/src/town-crier.worker/` — what external clients are registered with DI?

For each processor, record:
- **Name** (e.g., "Auth0", "Azure Communication Services")
- **Purpose** (what it does for us)
- **Data sent to it** (email, user ID, coordinates, device token)
- **Operator's location** (UK, EU, US, other) — this determines international-transfer disclosure
- **Is it disclosed** in the current privacy policy?

Known processors to expect (verify each against current code — don't assume):
| Processor | Purpose | Operator location |
|-----------|---------|-------------------|
| Auth0 (Okta) | Identity, login | US |
| Azure Cosmos DB | Primary database | Check region in Pulumi |
| Azure Container Apps | API hosting | Check region |
| Azure Communication Services | Transactional email | Check region (varies by data residency setting) |
| Azure Application Insights | Telemetry | Check region |
| Apple Push Notification Service | iOS notifications | US |
| Apple App Store | Subscription billing | US / Ireland |
| PlanIt (planit.org.uk) | Planning data source | UK — **but** we send no personal data to them, we only pull. Still worth clarifying. |

### 2e. Cross-Border Data Flows

For each processor in 2d, determine if personal data leaves the UK. If yes, UK GDPR Chapter V requires a lawful transfer mechanism (UK IDTA, Addendum to SCCs, adequacy decision). The privacy policy must disclose:
- That the transfer happens
- The safeguard relied on
- How the user can get a copy of the safeguard

**Auth0 (Okta)** is the main one — it's US-based. User email addresses and unique IDs reach the US. This must be disclosed.

## Phase 3: Gap Analysis

For each item in `references/uk-gdpr-checklist.md`, mark the current policy as: **covered**, **partial**, or **missing**. Then also check for drift: does the policy claim things that aren't true (e.g., "data is stored in UK" but Auth0 actually puts it in the US)?

Categorise every finding as one of:

- **Copy-only fix** — a sentence needs to be added or corrected. Handle this inside the single bead this skill creates.
- **Code change required** — the law requires something that doesn't exist yet (e.g., no account-deletion endpoint; no cookie banner; no data-export endpoint). File a **separate** bead for each, priority 1 or 2 depending on severity. The legal-copy update should honestly describe the current state, not promise a feature that doesn't exist.
- **No action** — already covered and accurate.

**Judgement calls:**
- Don't demand "DPO" disclosure if no DPO has been appointed. Small teams often aren't required to appoint one. Instead disclose the contact email for data requests.
- Don't fabricate cookie banners or consent flows that don't exist. If cookies aren't compliant, file a code-change bead, don't paper over it in the policy.
- If the policy names a processor that the code no longer uses, that's drift — remove it.

## Phase 4: Present the Findings

Show the user a concise report before making any changes:

```
# Legal Audit Report

## Copy drift (N findings)
1. [Section] — [problem] — [proposed fix]
2. ...

## Code-level gaps (M findings)
1. [Gap] — [what UK GDPR requires] — [will file bead]
2. ...

## Proposed edits to GetLegalDocumentQueryHandler.cs
[diff-style before/after for each affected section]

Proceed? [y/n]
```

Stop and wait for approval unless the user explicitly invoked with `--auto` or similar. Legal copy is sensitive — getting human confirmation is cheap, shipping bad copy is expensive.

## Phase 5: Apply Changes

On approval:

1. **Create the bead** for the copy update:
   ```
   bd create --title="Update legal copy — <summary>" \
     --description="Applies findings from /legal audit on <date>. See conversation for diff." \
     --type=task --priority=2
   bd update <id> --claim
   ```

2. **Enter a worktree** (repo convention — `.claude/require-worktree.sh` hook blocks edits on main):
   ```
   EnterWorktree with a name like "legal-audit-<date>"
   ```

3. **Apply the edits** to `GetLegalDocumentQueryHandler.cs`:
   - Update affected sections
   - Update `LastUpdated` to today
   - Preserve the existing plain-English style — no legalese. Shorter is better.
   - Match the voice of the current copy (first person plural, "we")

4. **Update tests** in `GetLegalDocumentQueryHandlerTests.cs` that assert on specific section content. Don't add tests for every sentence — the existing tests are structural (count of sections, headings, non-empty body). Update them to match new structure, not to pin down every word.

5. **Run the tests**:
   ```bash
   cd api && dotnet test --filter "FullyQualifiedName~Legal"
   ```
   If they fail, fix the assertions. Don't ship broken tests.

6. **Close the bead**:
   ```
   bd close <id>
   ```

7. **Ship** via the `/ship` skill (creates PR, waits for CI, auto-merges). Do not `git push` directly to main — repo policy forbids it.

## Phase 6: File Code-Gap Beads

For each finding in Phase 3 categorised as "code change required", create a separate open bead:

```
bd create --title="<compliance gap>" \
  --description="UK GDPR requires <article/obligation>. Currently missing: <what>. Needed: <what to build>. Found by /legal audit on <date>." \
  --type=<feature|bug> --priority=<1|2>
```

Leave these open. The `/legal` skill's job is to surface them, not to implement them. A follow-up conversation or `/autopilot` run will pick them up.

## Idempotency

This skill will be re-run. To avoid churn:
- If the gap analysis comes back clean (0 copy drift, 0 code gaps), print `Legal audit: no drift detected` and exit without creating a bead.
- Before filing a code-gap bead, search existing beads: `bd search "<keyword>"`. If one already exists for the same gap, update its notes instead of duplicating.
- Don't update `LastUpdated` in the handler unless a section actually changed. A date-only change is noise.

## Deeper reference

For the full UK GDPR Article 13/14 transparency checklist and Chapter V transfer obligations, see `references/uk-gdpr-checklist.md`. Read it during Phase 3 — don't try to remember all the requirements from training data.

## Voice Guide for the Copy

The existing copy is plain English. Preserve that. Guidelines:
- Short sentences. Active voice.
- "We" and "you", not "the data controller" and "the data subject".
- No Latin (no "inter alia", "pro rata", etc.).
- Explain the why when the reader might wonder. E.g., "We store coordinates rather than your exact address so we don't hold your home address on our servers."
- Where a law is the reason for a sentence, it's fine to cite it inline: "Under UK GDPR, you have the right to…".
- Do **not** copy-paste boilerplate from other policies. Every sentence should be about Town Crier specifically.

## Output Summary

At the end of the run, print a terse summary:

```
Legal audit complete.
- Copy updated: PR #<number>
- Code-gap beads filed: bd-xxx, bd-yyy
- Clean: <list of checklist items that were already fine>
```

Or, on the happy path:

```
Legal audit: no drift detected.
```
