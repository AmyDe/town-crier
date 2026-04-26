# Decision State Vocabulary Alignment

Date: 2026-04-26

## Context

PlanIt's `app_state` field uses a vocabulary that does **not** match what Town Crier's iOS, web, demo data, and tests assume. Investigation on 2026-04-26 sampled 20,000 of 153,666 production applications in `town-crier-prod`:

| Actual `app_state` | Share of sample |
|---|---|
| Undecided | 45% |
| Permitted | 30% |
| Conditions | 13% |
| Rejected | 6% |
| Withdrawn | 4% |
| (null) | 3% |
| Unresolved | <0.1% |
| Referred | <0.1% |

The iOS, web, and demo-data layers only recognise `Approved`, `Refused`, and `Appealed` — none of which ever appear in PlanIt data. Decided applications fall through to `.unknown` (iOS) and are silently dropped from any future status filtering. There are no users yet, so we can rename UX labels to match PlanIt directly without breaking expectations.

A second, independent gap surfaced during the same investigation: `DispatchDecisionAlertCommandHandler` exists at `api/src/town-crier.application/DecisionAlerts/` with full tests, but the handler is **not registered in DI** and is **not dispatched** from `PollPlanItCommandHandler` or anywhere else in production. Even with correct vocabulary, no decision push notifications would fire today.

This spec covers four independent, parallelisable work items.

References:
- Polling model: ADR-0006
- Earlier (incorrect) vocabulary assumption: `docs/specs/ios-api-alignment.md` §1.1 (now superseded)
- iOS coding standards: `.claude/skills/ios-coding-standards/`
- React coding standards: `.claude/skills/react-coding-standards/`
- .NET coding standards: `.claude/skills/dotnet-coding-standards/`

## Canonical Vocabulary

These are the strings PlanIt sends and what the entire stack should use end-to-end:

| String | Meaning | Decision? |
|---|---|---|
| `Undecided` | Not yet decided | No |
| `Permitted` | Granted | **Yes** |
| `Conditions` | Granted with conditions | **Yes** |
| `Rejected` | Refused | **Yes** |
| `Withdrawn` | Applicant withdrew | Terminal (not a decision alert) |
| `Appealed` | Refusal under appeal | Terminal (not a decision alert) |
| `Unresolved` | PlanIt couldn't determine state | No |
| `Referred` | Escalated (e.g. to Secretary of State) | No |
| `Not Available` | PlanIt does not have state info | No |
| `null` | Field missing on source record | No |

The four states triggering decision alerts (Task D below) are: **Permitted, Conditions, Rejected, Appealed**. Withdrawn does not trigger an alert.

Identifier casing: enums and union members are PascalCase matching the wire string exactly (`Permitted`, not `permitted`) on web; lowerCamelCase on Swift (`permitted`) with `rawValue = "Permitted"` so the Decodable mapping is direct. This avoids string-matching divergence.

## Scope

In:
- iOS, web, API demo data, tests align to the vocabulary above
- Decision-alert dispatch wired into the polling pipeline

Out:
- Push notification copy variants (handled later — initial version uses generic "Decision: <state>")
- Email digest changes (separate spec already)
- Renaming `appState` field on the wire (would break Cosmos docs — kept as-is)

## Steps

### iOS

Update `mobile/ios/packages/town-crier-domain/Sources/Entities/ApplicationStatus.swift`:

```swift
public enum ApplicationStatus: String, Equatable, Hashable, Sendable {
  case undecided      = "Undecided"
  case permitted      = "Permitted"
  case conditions     = "Conditions"
  case rejected       = "Rejected"
  case withdrawn      = "Withdrawn"
  case appealed       = "Appealed"
  case unresolved     = "Unresolved"
  case referred       = "Referred"
  case notAvailable   = "Not Available"
  case unknown
}
```

Then:
- `town-crier-data/Sources/Repositories/APIPlanningApplicationRepository.swift`: replace the `mapAppState` switch with `ApplicationStatus(rawValue: state) ?? .unknown` so the enum's raw values are the source of truth. Update `synthesizeStatusHistory` so the "decided" event fires on `.permitted`, `.conditions`, `.rejected` (not `.approved`/`.refused`).
- `town-crier-domain/Sources/Entities/PlanningApplication.swift`: update `Decision` enum and the recordDecision logic — there's a line `decision == .approved ? .approved : .refused` that needs to map to the new vocabulary. `Decision` should become `permitted | conditions | rejected`.
- `town-crier-presentation/Sources/DesignSystem/Components/ApplicationStatus+Display.swift`: update display labels and color cases. Suggested labels: "Granted" for `.permitted`, "Granted with conditions" for `.conditions`, "Refused" for `.rejected` — these are user-facing. (We keep "Refused" in user-facing copy because that's how UK residents speak about planning outcomes, even though the wire value is `Rejected`.)
- `town-crier-presentation/Sources/DesignSystem/Colors/Color+TownCrier.swift`: rename `tcStatusApproved` → `tcStatusPermitted`, `tcStatusRefused` → `tcStatusRejected`. Add `tcStatusConditions` (suggested: amber/orange — granted but constrained).
- All views referencing the renamed colors (search for `tcStatusApproved`/`tcStatusRefused`).
- Tests: `ApplicationStatusTests`, fixture builders, any spy/fake that hard-codes the old strings.

### Web

Update `web/src/domain/types.ts`:

```ts
export type ApplicationStatus =
  | "Undecided"
  | "Permitted"
  | "Conditions"
  | "Rejected"
  | "Withdrawn"
  | "Appealed"
  | "Unresolved"
  | "Referred"
  | "Not Available";
```

Update `APPLICATION_STATUSES` array and `isApplicationStatus` accordingly. Then:
- All fixtures: `web/src/features/SavedApplications/__tests__/fixtures/saved-application.fixtures.ts` (rename `savedApprovedApplication` → `savedPermittedApplication`, change `appState` to `'Permitted'`), and equivalents in `ApplicationDetail`, `Search`, `Dashboard`, `Map`, `Applications`, `ApplicationCard` fixtures.
- CSS tokens: rename `--tc-status-refused` → `--tc-status-rejected`, add `--tc-status-permitted` / `--tc-status-conditions` (current `tc-status-approved` likely needs renaming too — search globally). User-facing copy on web can keep "Granted"/"Refused" labels via a display helper.
- Tests in `domain/__tests__/types.test.ts`, `useSavedApplications.test.ts`, `SavedApplicationsPage.test.tsx`, etc. — update assertions to the new vocabulary.

### API

The wire field stays `appState` and the persisted Cosmos field stays `appState` — no domain or persistence rename. Changes are limited to:

- `api/src/town-crier.application/DemoAccount/DemoSeedData.cs`: change `appState: "Approved"` → `"Permitted"`, `"Refused"` → `"Rejected"`. Add a `Conditions` example so the demo account exercises that branch.
- `api/tests/town-crier.domain.tests/PlanningApplications/PlanningApplicationBuilder.cs`: default `appState` stays `"Undecided"`.
- `api/tests/town-crier.application.tests/DecisionAlerts/DispatchDecisionAlertCommandHandlerTests.cs`: parameterise tests over the four decision states (`Permitted`, `Conditions`, `Rejected`, `Appealed`).
- `api/tests/town-crier.infrastructure.tests/DecisionAlerts/DecisionAlertDocumentTests.cs`: update fixture decision strings.

### Dispatch

Wire `DispatchDecisionAlertCommand` into the polling pipeline.

DI registration:
- `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`: add `services.AddTransient<DispatchDecisionAlertCommandHandler>();` and register `IDecisionAlertPushSender` (an APNS-backed implementation already exists for other push types — reuse the abstraction).
- `api/src/town-crier.worker/Program.cs`: same registration in the worker DI graph (the polling loop runs there, not in the web host).

Trigger location:
- In `PollPlanItCommandHandler`, when an upserted application's `appState` transitions from non-decision to one of `Permitted | Conditions | Rejected | Appealed`, dispatch `DispatchDecisionAlertCommand`.
- The transition check requires comparing the persisted document (pre-upsert) against the incoming PlanIt record. The handler already loads existing docs to compute `HasSameBusinessFieldsAs` — extend that path to detect "previous state was non-decision, new state is decision".
- For first-time inserts where the application arrives already-decided, dispatch on insert — bookmark holders should still be notified.

Idempotency:
- `DispatchDecisionAlertCommandHandler` already enforces "one alert per user per application" via `GetByUserAndApplicationAsync`. No additional dedupe needed at the polling layer.

Observability:
- Emit a structured log line `decision_alert_dispatched { application_uid, previous_state, new_state, recipient_count }` so SRE can verify the path is firing.
- Add a counter metric `town_crier_decision_alerts_dispatched_total{state}` on the existing telemetry pipeline.

Testing:
- Add a `PollPlanItCommandHandler` test asserting that a state transition from `Undecided` to `Permitted` enqueues the dispatch command exactly once.
- Add a test for the first-time-insert path.
- Add a test for state transitions that should NOT dispatch (`Undecided → Withdrawn`, `Permitted → Conditions` — same-decision-class change).
