# Testing (reference)

Read when writing any Vitest test, a hand-written spy, or a factory fixture. The core (`SKILL.md`) states the test-double conventions; this file is the full prose and examples (repository port, spy, fixture, hook test).

## 5. Testing Strategy (Vitest + Testing Library)

Testing is deferred for the static landing page (there's nothing to test beyond "does it render"), but these standards apply the moment interactive features arrive. The approach mirrors the .NET and iOS testing strategies: test behavior through the orchestration layer (hooks), not implementation details.

- **Framework:** Vitest (integrates with Vite) + React Testing Library. Vitest provides the test runner and assertions; Testing Library provides DOM queries that encourage accessible, behavioral testing.
- **Unit of Work:** Custom hooks are the primary test target — they contain the orchestration logic, just like Handlers (.NET) and ViewModels (iOS). Domain entities with business rules also warrant direct unit tests. Components are tested via integration tests that render them with real hooks.
- **Workflow:** Red-Green-Refactor. Write the test first.
- **Manual Test Doubles:** Hand-written spies and fakes, consistent with the no-mocking-frameworks policy across the monorepo. No `vi.fn()` or `vi.mock()` for repository dependencies — write explicit spy classes that implement the port interface. This keeps tests readable and avoids magic.
- **Fixtures:** Factory functions that return domain entities with sensible defaults. Override specific fields via spread syntax. This is the TypeScript equivalent of the Builder pattern (.NET) and static extensions (iOS).

**Example — Repository Port (Domain layer):**
```typescript
// domain/ports/planning-application-repository.ts
export interface PlanningApplicationRepository {
  fetchApplications(authority: LocalAuthority): Promise<PlanningApplication[]>;
  fetchApplication(id: PlanningApplicationId): Promise<PlanningApplication>;
}
```

**Example — Spy (Test double):**
```typescript
// __tests__/spies/spy-planning-application-repository.ts
export class SpyPlanningApplicationRepository implements PlanningApplicationRepository {
  fetchApplicationsCalls: LocalAuthority[] = [];
  fetchApplicationsResult: PlanningApplication[] = [];

  async fetchApplications(authority: LocalAuthority): Promise<PlanningApplication[]> {
    this.fetchApplicationsCalls.push(authority);
    return this.fetchApplicationsResult;
  }

  fetchApplicationCalls: PlanningApplicationId[] = [];
  fetchApplicationResult: PlanningApplication = pendingReview();

  async fetchApplication(id: PlanningApplicationId): Promise<PlanningApplication> {
    this.fetchApplicationCalls.push(id);
    return this.fetchApplicationResult;
  }
}
```

**Example — Fixture (Factory function):**
```typescript
// __tests__/fixtures/planning-application.fixtures.ts
export function pendingReview(
  overrides?: Partial<PlanningApplication>
): PlanningApplication {
  return {
    id: "APP-001" as PlanningApplicationId,
    reference: "2026/0042" as ApplicationReference,
    authority: { code: "CAM", name: "Cambridge City Council" },
    status: "under-review",
    receivedDate: new Date("2026-01-15"),
    description: "Erection of two-storey rear extension",
    address: "12 Mill Road, Cambridge, CB1 2AD",
    ...overrides,
  };
}

export function approved(
  overrides?: Partial<PlanningApplication>
): PlanningApplication {
  return {
    ...pendingReview(),
    id: "APP-002" as PlanningApplicationId,
    reference: "2026/0099" as ApplicationReference,
    status: "approved",
    description: "Change of use from retail to residential",
    address: "45 High Street, Cambridge, CB2 1LA",
    ...overrides,
  };
}
```

**Example — Hook Test:**
```typescript
// features/application-feed/__tests__/useApplicationFeed.test.ts
import { renderHook, waitFor } from "@testing-library/react";
import { useApplicationFeed } from "../useApplicationFeed";
import { SpyPlanningApplicationRepository } from "./spies/spy-planning-application-repository";
import { pendingReview, approved } from "./fixtures/planning-application.fixtures";

describe("useApplicationFeed", () => {
  it("populates applications on successful fetch", async () => {
    const spy = new SpyPlanningApplicationRepository();
    spy.fetchApplicationsResult = [pendingReview(), approved()];

    const { result } = renderHook(() =>
      useApplicationFeed(spy, { code: "CAM", name: "Cambridge" })
    );

    await waitFor(() => {
      expect(result.current.applications).toHaveLength(2);
      expect(result.current.isLoading).toBe(false);
      expect(result.current.error).toBeNull();
    });
  });

  it("sets error on failed fetch", async () => {
    const spy = new SpyPlanningApplicationRepository();
    spy.fetchApplications = async () => {
      throw new DomainError("Network unavailable");
    };

    const { result } = renderHook(() =>
      useApplicationFeed(spy, { code: "CAM", name: "Cambridge" })
    );

    await waitFor(() => {
      expect(result.current.error?.message).toBe("Network unavailable");
      expect(result.current.applications).toEqual([]);
    });
  });
});
```
