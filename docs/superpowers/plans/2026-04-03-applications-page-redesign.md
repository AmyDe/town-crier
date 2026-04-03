# Applications Page Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the authority search box on the Applications page with a list of the authenticated user's watch-zone authorities, so only authorities with data are shown.

**Architecture:** New backend query handler resolves the user's watch zone authority IDs into `AuthorityListItem` records via the cached authority provider. The frontend replaces `AuthoritySelector` with a card-based authority picker and adds breadcrumb navigation between authority list and application list views.

**Tech Stack:** .NET 10 (TUnit tests), React 19 + TypeScript (Vitest + Testing Library), CSS Modules with design tokens.

---

### Task 1: Backend — Query, Result, and Handler

**Files:**
- Create: `api/src/town-crier.application/PlanningApplications/GetUserApplicationAuthoritiesQuery.cs`
- Create: `api/src/town-crier.application/PlanningApplications/GetUserApplicationAuthoritiesResult.cs`
- Create: `api/src/town-crier.application/PlanningApplications/GetUserApplicationAuthoritiesQueryHandler.cs`
- Test: `api/tests/town-crier.application.tests/PlanningApplications/GetUserApplicationAuthoritiesQueryHandlerTests.cs`

- [ ] **Step 1: Create the query record**

```csharp
// api/src/town-crier.application/PlanningApplications/GetUserApplicationAuthoritiesQuery.cs
namespace TownCrier.Application.PlanningApplications;

public sealed record GetUserApplicationAuthoritiesQuery(string UserId);
```

- [ ] **Step 2: Create the result record**

Reuses the existing `AuthorityListItem` from the Authorities namespace.

```csharp
// api/src/town-crier.application/PlanningApplications/GetUserApplicationAuthoritiesResult.cs
using TownCrier.Application.Authorities;

namespace TownCrier.Application.PlanningApplications;

public sealed record GetUserApplicationAuthoritiesResult(
    IReadOnlyList<AuthorityListItem> Authorities,
    int Count);
```

- [ ] **Step 3: Write the failing tests**

```csharp
// api/tests/town-crier.application.tests/PlanningApplications/GetUserApplicationAuthoritiesQueryHandlerTests.cs
using TownCrier.Application.Authorities;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Tests.Authorities;
using TownCrier.Application.Tests.Polling;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.Tests.PlanningApplications;

public sealed class GetUserApplicationAuthoritiesQueryHandlerTests
{
    private readonly FakeWatchZoneRepository watchZoneRepository = new();
    private readonly FakeAuthorityProvider authorityProvider = new();

    [Test]
    public async Task Should_ReturnEmpty_When_UserHasNoWatchZones()
    {
        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(0);
        await Assert.That(result.Count).IsEqualTo(0);
    }

    [Test]
    public async Task Should_ReturnMatchingAuthorities_When_UserHasWatchZones()
    {
        this.watchZoneRepository.Add(new WatchZone(
            "zone-1", "user-1", "Home", new Coordinates(50.7, -3.5), 5000, 42));
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(42).WithName("Cornwall Council").WithAreaType("Unitary").Build());

        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(1);
        await Assert.That(result.Authorities[0].Id).IsEqualTo(42);
        await Assert.That(result.Authorities[0].Name).IsEqualTo("Cornwall Council");
        await Assert.That(result.Count).IsEqualTo(1);
    }

    [Test]
    public async Task Should_DeduplicateAuthorities_When_MultipleZonesSameAuthority()
    {
        this.watchZoneRepository.Add(new WatchZone(
            "zone-1", "user-1", "Home", new Coordinates(50.7, -3.5), 5000, 42));
        this.watchZoneRepository.Add(new WatchZone(
            "zone-2", "user-1", "Office", new Coordinates(50.8, -3.6), 3000, 42));
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(42).WithName("Cornwall Council").WithAreaType("Unitary").Build());

        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_SortByName_When_MultipleAuthorities()
    {
        this.watchZoneRepository.Add(new WatchZone(
            "zone-1", "user-1", "Home", new Coordinates(50.7, -3.5), 5000, 42));
        this.watchZoneRepository.Add(new WatchZone(
            "zone-2", "user-1", "Work", new Coordinates(51.5, -0.1), 3000, 10));
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(42).WithName("Cornwall Council").WithAreaType("Unitary").Build());
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(10).WithName("Bath and NE Somerset").WithAreaType("Unitary").Build());

        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(2);
        await Assert.That(result.Authorities[0].Name).IsEqualTo("Bath and NE Somerset");
        await Assert.That(result.Authorities[1].Name).IsEqualTo("Cornwall Council");
    }

    [Test]
    public async Task Should_ExcludeOtherUsersZones_When_MultipleUsersExist()
    {
        this.watchZoneRepository.Add(new WatchZone(
            "zone-1", "user-1", "Home", new Coordinates(50.7, -3.5), 5000, 42));
        this.watchZoneRepository.Add(new WatchZone(
            "zone-2", "user-2", "Home", new Coordinates(51.5, -0.1), 3000, 10));
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(42).WithName("Cornwall Council").WithAreaType("Unitary").Build());
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(10).WithName("Camden").WithAreaType("London Borough").Build());

        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(1);
        await Assert.That(result.Authorities[0].Name).IsEqualTo("Cornwall Council");
    }

    [Test]
    public async Task Should_SkipAuthority_When_NotFoundInProvider()
    {
        this.watchZoneRepository.Add(new WatchZone(
            "zone-1", "user-1", "Home", new Coordinates(50.7, -3.5), 5000, 999));

        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(0);
        await Assert.That(result.Count).IsEqualTo(0);
    }
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `dotnet test api/tests/town-crier.application.tests --filter "GetUserApplicationAuthoritiesQueryHandlerTests"`
Expected: Build failure — `GetUserApplicationAuthoritiesQueryHandler` does not exist.

- [ ] **Step 5: Implement the handler**

```csharp
// api/src/town-crier.application/PlanningApplications/GetUserApplicationAuthoritiesQueryHandler.cs
using TownCrier.Application.Authorities;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.PlanningApplications;

public sealed class GetUserApplicationAuthoritiesQueryHandler
{
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly IAuthorityProvider authorityProvider;

    public GetUserApplicationAuthoritiesQueryHandler(
        IWatchZoneRepository watchZoneRepository,
        IAuthorityProvider authorityProvider)
    {
        this.watchZoneRepository = watchZoneRepository;
        this.authorityProvider = authorityProvider;
    }

    public async Task<GetUserApplicationAuthoritiesResult> HandleAsync(
        GetUserApplicationAuthoritiesQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var zones = await this.watchZoneRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);

        var distinctAuthorityIds = zones
            .Select(z => z.AuthorityId)
            .Distinct()
            .ToHashSet();

        var authorities = new List<AuthorityListItem>();
        foreach (var authorityId in distinctAuthorityIds)
        {
            var authority = await this.authorityProvider.GetByIdAsync(authorityId, ct).ConfigureAwait(false);
            if (authority is not null)
            {
                authorities.Add(new AuthorityListItem(authority.Id, authority.Name, authority.AreaType));
            }
        }

        authorities.Sort((a, b) => string.Compare(a.Name, b.Name, StringComparison.OrdinalIgnoreCase));

        return new GetUserApplicationAuthoritiesResult(authorities, authorities.Count);
    }
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `dotnet test api/tests/town-crier.application.tests --filter "GetUserApplicationAuthoritiesQueryHandlerTests"`
Expected: All 6 tests pass.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.application/PlanningApplications/GetUserApplicationAuthoritiesQuery.cs \
  api/src/town-crier.application/PlanningApplications/GetUserApplicationAuthoritiesResult.cs \
  api/src/town-crier.application/PlanningApplications/GetUserApplicationAuthoritiesQueryHandler.cs \
  api/tests/town-crier.application.tests/PlanningApplications/GetUserApplicationAuthoritiesQueryHandlerTests.cs
git commit -m "feat(api): add GetUserApplicationAuthoritiesQueryHandler"
```

---

### Task 2: Backend — Endpoint, DI Registration, and Serialization

**Files:**
- Modify: `api/src/town-crier.web/Endpoints/PlanningApplicationEndpoints.cs`
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`
- Modify: `api/src/town-crier.web/AppJsonSerializerContext.cs`
- Modify: `api/tests/town-crier.web.tests/DependencyInjection/EndpointMappingTests.cs`

- [ ] **Step 1: Write a failing endpoint mapping test**

Add one test argument to the authenticated endpoints test in `EndpointMappingTests.cs`:

```csharp
// In EndpointMappingTests.cs, add a new Arguments attribute to the authenticated test:
[Arguments("/v1/me/application-authorities", "GET")]
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `dotnet test api/tests/town-crier.web.tests --filter "Should_MapAuthenticatedEndpoints_When_MapAllEndpointsCalled"`
Expected: Failure — 404 for the new route.

- [ ] **Step 3: Register the handler in DI**

In `ServiceCollectionExtensions.cs`, in the `AddApplicationServices` method, add after the `GetApplicationsByAuthorityQueryHandler` registration:

```csharp
services.AddTransient<GetUserApplicationAuthoritiesQueryHandler>();
```

- [ ] **Step 4: Register result type in serializer context**

In `AppJsonSerializerContext.cs`, add a new attribute before the class declaration:

```csharp
[JsonSerializable(typeof(GetUserApplicationAuthoritiesResult))]
```

Add the using at the top if not already present (it should be — `TownCrier.Application.PlanningApplications` is already imported).

- [ ] **Step 5: Add the endpoint**

In `PlanningApplicationEndpoints.cs`, add a new endpoint inside `MapPlanningApplicationEndpoints`:

```csharp
group.MapGet("/me/application-authorities", async (
    ClaimsPrincipal user,
    GetUserApplicationAuthoritiesQueryHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var result = await handler.HandleAsync(
        new GetUserApplicationAuthoritiesQuery(userId), ct).ConfigureAwait(false);
    return Results.Ok(result);
});
```

Add the required using at the top of the file:

```csharp
using System.Security.Claims;
```

- [ ] **Step 6: Run the endpoint mapping test to verify it passes**

Run: `dotnet test api/tests/town-crier.web.tests --filter "Should_MapAuthenticatedEndpoints_When_MapAllEndpointsCalled"`
Expected: All test cases pass.

- [ ] **Step 7: Run full API test suite**

Run: `dotnet test api/`
Expected: All tests pass.

- [ ] **Step 8: Commit**

```bash
git add api/src/town-crier.web/Endpoints/PlanningApplicationEndpoints.cs \
  api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs \
  api/src/town-crier.web/AppJsonSerializerContext.cs \
  api/tests/town-crier.web.tests/DependencyInjection/EndpointMappingTests.cs
git commit -m "feat(api): add GET /v1/me/application-authorities endpoint"
```

---

### Task 3: Frontend — New Port, API Adapter, Hook, and Tests

**Files:**
- Create: `web/src/domain/ports/user-authorities-port.ts`
- Create: `web/src/features/Applications/__tests__/spies/spy-user-authorities-port.ts`
- Create: `web/src/features/Applications/__tests__/fixtures/authority.fixtures.ts`
- Create: `web/src/features/Applications/useUserAuthorities.ts`
- Create: `web/src/features/Applications/__tests__/useUserAuthorities.test.ts`

- [ ] **Step 1: Create the port interface**

```typescript
// web/src/domain/ports/user-authorities-port.ts
import type { AuthorityListItem } from '../types';

export interface UserAuthoritiesPort {
  fetchMyAuthorities(): Promise<readonly AuthorityListItem[]>;
}
```

- [ ] **Step 2: Create the spy for testing**

```typescript
// web/src/features/Applications/__tests__/spies/spy-user-authorities-port.ts
import type { AuthorityListItem } from '../../../../domain/types';
import type { UserAuthoritiesPort } from '../../../../domain/ports/user-authorities-port';

export class SpyUserAuthoritiesPort implements UserAuthoritiesPort {
  fetchMyAuthoritiesCalls = 0;
  fetchMyAuthoritiesResult: readonly AuthorityListItem[] = [];
  fetchMyAuthoritiesError: Error | null = null;

  async fetchMyAuthorities(): Promise<readonly AuthorityListItem[]> {
    this.fetchMyAuthoritiesCalls++;
    if (this.fetchMyAuthoritiesError) {
      throw this.fetchMyAuthoritiesError;
    }
    return this.fetchMyAuthoritiesResult;
  }
}
```

- [ ] **Step 3: Create authority fixtures for this feature**

```typescript
// web/src/features/Applications/__tests__/fixtures/authority.fixtures.ts
import type { AuthorityListItem } from '../../../../domain/types';
import { asAuthorityId } from '../../../../domain/types';

export function cornwallAuthority(
  overrides?: Partial<AuthorityListItem>,
): AuthorityListItem {
  return {
    id: asAuthorityId(42),
    name: 'Cornwall Council',
    areaType: 'Unitary',
    ...overrides,
  };
}

export function bathAuthority(
  overrides?: Partial<AuthorityListItem>,
): AuthorityListItem {
  return {
    id: asAuthorityId(10),
    name: 'Bath and NE Somerset',
    areaType: 'Unitary',
    ...overrides,
  };
}
```

- [ ] **Step 4: Write the failing hook tests**

```typescript
// web/src/features/Applications/__tests__/useUserAuthorities.test.ts
import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useUserAuthorities } from '../useUserAuthorities';
import { SpyUserAuthoritiesPort } from './spies/spy-user-authorities-port';
import { cornwallAuthority, bathAuthority } from './fixtures/authority.fixtures';

describe('useUserAuthorities', () => {
  it('fetches authorities on mount', async () => {
    const spy = new SpyUserAuthoritiesPort();
    spy.fetchMyAuthoritiesResult = [cornwallAuthority(), bathAuthority()];

    const { result } = renderHook(() => useUserAuthorities(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.authorities).toHaveLength(2);
    expect(spy.fetchMyAuthoritiesCalls).toBe(1);
  });

  it('starts in loading state', () => {
    const spy = new SpyUserAuthoritiesPort();

    const { result } = renderHook(() => useUserAuthorities(spy));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.authorities).toEqual([]);
  });

  it('sets error when fetch fails', async () => {
    const spy = new SpyUserAuthoritiesPort();
    spy.fetchMyAuthoritiesError = new Error('Network error');

    const { result } = renderHook(() => useUserAuthorities(spy));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).not.toBeNull();
    expect(result.current.error?.message).toBe('Network error');
    expect(result.current.authorities).toEqual([]);
  });
});
```

- [ ] **Step 5: Run the tests to verify they fail**

Run: `cd web && npx vitest run src/features/Applications/__tests__/useUserAuthorities.test.ts`
Expected: Failure — `useUserAuthorities` does not exist.

- [ ] **Step 6: Implement the hook**

```typescript
// web/src/features/Applications/useUserAuthorities.ts
import { useState, useEffect } from 'react';
import type { AuthorityListItem } from '../../domain/types';
import type { UserAuthoritiesPort } from '../../domain/ports/user-authorities-port';

interface UserAuthoritiesState {
  authorities: readonly AuthorityListItem[];
  isLoading: boolean;
  error: Error | null;
}

export function useUserAuthorities(port: UserAuthoritiesPort) {
  const [state, setState] = useState<UserAuthoritiesState>({
    authorities: [],
    isLoading: true,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;

    port
      .fetchMyAuthorities()
      .then((authorities) => {
        if (!cancelled) {
          setState({ authorities, isLoading: false, error: null });
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setState({
            authorities: [],
            isLoading: false,
            error: err instanceof Error ? err : new Error(String(err)),
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [port]);

  return state;
}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd web && npx vitest run src/features/Applications/__tests__/useUserAuthorities.test.ts`
Expected: All 3 tests pass.

- [ ] **Step 8: Commit**

```bash
git add web/src/domain/ports/user-authorities-port.ts \
  web/src/features/Applications/useUserAuthorities.ts \
  web/src/features/Applications/__tests__/useUserAuthorities.test.ts \
  web/src/features/Applications/__tests__/spies/spy-user-authorities-port.ts \
  web/src/features/Applications/__tests__/fixtures/authority.fixtures.ts
git commit -m "feat(web): add useUserAuthorities hook and port"
```

---

### Task 4: Frontend — Redesign ApplicationsPage Component

**Files:**
- Modify: `web/src/features/Applications/ApplicationsPage.tsx`
- Modify: `web/src/features/Applications/ApplicationsPage.module.css`
- Rewrite: `web/src/features/Applications/__tests__/ApplicationsPage.test.tsx`

- [ ] **Step 1: Write the failing page tests**

Replace the entire test file to test the new authority-card-based design:

```typescript
// web/src/features/Applications/__tests__/ApplicationsPage.test.tsx
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { ApplicationsPage } from '../ApplicationsPage';
import { SpyUserAuthoritiesPort } from './spies/spy-user-authorities-port';
import { SpyApplicationsBrowsePort } from './spies/spy-applications-browse-port';
import { cornwallAuthority, bathAuthority } from './fixtures/authority.fixtures';
import {
  undecidedApplication,
  approvedApplication,
} from '../../../components/ApplicationCard/__tests__/fixtures/planning-application-summary.fixtures';

function renderPage(
  userAuthoritiesPort: SpyUserAuthoritiesPort,
  browsePort: SpyApplicationsBrowsePort,
) {
  return render(
    <MemoryRouter>
      <ApplicationsPage userAuthoritiesPort={userAuthoritiesPort} browsePort={browsePort} />
    </MemoryRouter>,
  );
}

describe('ApplicationsPage', () => {
  let userAuthoritiesPort: SpyUserAuthoritiesPort;
  let browsePort: SpyApplicationsBrowsePort;

  beforeEach(() => {
    userAuthoritiesPort = new SpyUserAuthoritiesPort();
    browsePort = new SpyApplicationsBrowsePort();
  });

  it('renders page heading', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [];
    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Applications' })).toBeInTheDocument();
    });
  });

  it('shows loading state while fetching authorities', () => {
    renderPage(userAuthoritiesPort, browsePort);

    expect(screen.getByText('Loading authorities...')).toBeInTheDocument();
  });

  it('shows empty state when user has no watch zones', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [];
    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(
        screen.getByText('Set up a watch zone to start browsing applications.'),
      ).toBeInTheDocument();
    });
  });

  it('shows authority cards when user has watch zones', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [cornwallAuthority(), bathAuthority()];
    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
    });
    expect(screen.getByText('Bath and NE Somerset')).toBeInTheDocument();
  });

  it('shows applications when authority card is clicked', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [cornwallAuthority()];
    browsePort.fetchByAuthorityResult = [undecidedApplication(), approvedApplication()];
    const user = userEvent.setup();

    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cornwall Council'));

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });

    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
    expect(browsePort.fetchByAuthorityCalls).toEqual([cornwallAuthority().id]);
  });

  it('shows breadcrumb when viewing applications', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [cornwallAuthority()];
    browsePort.fetchByAuthorityResult = [undecidedApplication()];
    const user = userEvent.setup();

    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cornwall Council'));

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });

    expect(screen.getByRole('link', { name: 'Authorities' })).toBeInTheDocument();
  });

  it('returns to authority list when breadcrumb is clicked', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [cornwallAuthority(), bathAuthority()];
    browsePort.fetchByAuthorityResult = [undecidedApplication()];
    const user = userEvent.setup();

    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cornwall Council'));

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('link', { name: 'Authorities' }));

    await waitFor(() => {
      expect(screen.getByText('Bath and NE Somerset')).toBeInTheDocument();
    });
    expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
  });

  it('shows empty state when authority has no applications', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [cornwallAuthority()];
    browsePort.fetchByAuthorityResult = [];
    const user = userEvent.setup();

    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cornwall Council'));

    await waitFor(() => {
      expect(
        screen.getByText('No applications found for this authority.'),
      ).toBeInTheDocument();
    });
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd web && npx vitest run src/features/Applications/__tests__/ApplicationsPage.test.tsx`
Expected: Failure — `ApplicationsPage` still expects old props.

- [ ] **Step 3: Update the CSS module**

Replace `ApplicationsPage.module.css` with:

```css
/* web/src/features/Applications/ApplicationsPage.module.css */
.container {
  max-width: 800px;
  margin: 0 auto;
  padding: var(--tc-space-lg) var(--tc-space-md);
}

.heading {
  font-size: var(--tc-text-display-large);
  font-weight: var(--tc-weight-bold);
  color: var(--tc-text-primary);
  margin: 0 0 var(--tc-space-lg);
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: var(--tc-space-xs);
  margin-bottom: var(--tc-space-lg);
  font-size: var(--tc-text-body);
  color: var(--tc-text-secondary);
}

.breadcrumbLink {
  color: var(--tc-amber);
  text-decoration: none;
  cursor: pointer;
  background: none;
  border: none;
  font: inherit;
  padding: 0;
}

.breadcrumbLink:hover {
  text-decoration: underline;
}

.breadcrumbCurrent {
  color: var(--tc-text-primary);
  font-weight: var(--tc-weight-medium);
}

.authorityGrid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: var(--tc-space-md);
}

.authorityCard {
  display: flex;
  flex-direction: column;
  gap: var(--tc-space-xs);
  background: var(--tc-surface);
  border: 1px solid var(--tc-border);
  border-radius: var(--tc-radius-md);
  padding: var(--tc-space-md);
  box-shadow: var(--tc-shadow-card);
  cursor: pointer;
  transition: border-color var(--tc-duration-fast) ease;
  text-align: left;
  font: inherit;
  color: inherit;
}

.authorityCard:hover {
  border-color: var(--tc-amber);
}

.authorityCard:focus-visible {
  outline: 2px solid var(--tc-border-focused);
  outline-offset: 2px;
}

.authorityName {
  font-size: var(--tc-text-headline);
  font-weight: var(--tc-weight-semibold);
  color: var(--tc-text-primary);
}

.authorityType {
  font-size: var(--tc-text-caption);
  color: var(--tc-text-secondary);
}

.list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: var(--tc-space-md);
}

.loading {
  text-align: center;
  padding: var(--tc-space-xl);
  color: var(--tc-text-secondary);
  font-size: var(--tc-text-body);
}
```

- [ ] **Step 4: Rewrite the ApplicationsPage component**

```tsx
// web/src/features/Applications/ApplicationsPage.tsx
import type { AuthorityListItem } from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { UserAuthoritiesPort } from '../../domain/ports/user-authorities-port';
import { useUserAuthorities } from './useUserAuthorities';
import { useApplications } from './useApplications';
import { ApplicationCard } from '../../components/ApplicationCard/ApplicationCard';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import styles from './ApplicationsPage.module.css';

interface Props {
  userAuthoritiesPort: UserAuthoritiesPort;
  browsePort: ApplicationsBrowsePort;
}

export function ApplicationsPage({ userAuthoritiesPort, browsePort }: Props) {
  const { authorities, isLoading: isLoadingAuthorities, error: authoritiesError } =
    useUserAuthorities(userAuthoritiesPort);
  const { selectedAuthority, applications, isLoading: isLoadingApps, error: appsError, selectAuthority } =
    useApplications(browsePort);

  function handleAuthorityClick(authority: AuthorityListItem) {
    selectAuthority(authority);
  }

  function handleBackToAuthorities() {
    selectAuthority(null);
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Applications</h1>

      {selectedAuthority !== null && (
        <nav className={styles.breadcrumb} aria-label="Breadcrumb">
          <a
            className={styles.breadcrumbLink}
            role="link"
            tabIndex={0}
            onClick={handleBackToAuthorities}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                handleBackToAuthorities();
              }
            }}
          >
            Authorities
          </a>
          <span aria-hidden="true">&rsaquo;</span>
          <span className={styles.breadcrumbCurrent}>{selectedAuthority.name}</span>
        </nav>
      )}

      {selectedAuthority === null && (
        <>
          {isLoadingAuthorities && (
            <div className={styles.loading} aria-live="polite">Loading authorities...</div>
          )}

          {authoritiesError !== null && (
            <EmptyState
              title="Something went wrong"
              message={authoritiesError.message}
            />
          )}

          {!isLoadingAuthorities && authoritiesError === null && authorities.length === 0 && (
            <EmptyState
              icon="🏛️"
              title="No watch zones yet"
              message="Set up a watch zone to start browsing applications."
              actionLabel="Create watch zone"
              onAction={() => {
                window.location.href = '/watch-zones/new';
              }}
            />
          )}

          {!isLoadingAuthorities && authoritiesError === null && authorities.length > 0 && (
            <div className={styles.authorityGrid}>
              {authorities.map((authority) => (
                <button
                  key={authority.id}
                  className={styles.authorityCard}
                  onClick={() => handleAuthorityClick(authority)}
                >
                  <span className={styles.authorityName}>{authority.name}</span>
                  <span className={styles.authorityType}>{authority.areaType}</span>
                </button>
              ))}
            </div>
          )}
        </>
      )}

      {selectedAuthority !== null && (
        <>
          {isLoadingApps && (
            <div className={styles.loading} aria-live="polite">Loading applications...</div>
          )}

          {appsError !== null && (
            <EmptyState
              title="Something went wrong"
              message={appsError.message}
              actionLabel="Try again"
              onAction={() => selectAuthority(selectedAuthority)}
            />
          )}

          {!isLoadingApps && appsError === null && applications.length === 0 && (
            <EmptyState
              icon="📋"
              title="No applications"
              message="No applications found for this authority."
            />
          )}

          {!isLoadingApps && appsError === null && applications.length > 0 && (
            <ul className={styles.list}>
              {applications.map((app) => (
                <li key={app.uid}>
                  <ApplicationCard application={app} />
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </div>
  );
}
```

- [ ] **Step 5: Update `useApplications` to support resetting selection**

The `selectAuthority` callback currently only accepts `AuthorityListItem`. Update it to also accept `null` for clearing selection:

In `web/src/features/Applications/useApplications.ts`, change the `selectAuthority` callback:

```typescript
const selectAuthority = useCallback(
  (authority: AuthorityListItem | null) => {
    if (authority === null) {
      setState({
        selectedAuthority: null,
        applications: [],
        isLoading: false,
        error: null,
      });
      return;
    }

    setState((prev) => ({
      ...prev,
      selectedAuthority: authority,
      isLoading: true,
      error: null,
    }));

    port
      .fetchByAuthority(authority.id)
      .then((applications) => {
        setState((prev) => ({
          ...prev,
          applications,
          isLoading: false,
        }));
      })
      .catch((err: unknown) => {
        setState((prev) => ({
          ...prev,
          applications: [],
          isLoading: false,
          error: err instanceof Error ? err : new Error(String(err)),
        }));
      });
  },
  [port],
);
```

- [ ] **Step 6: Run the page tests to verify they pass**

Run: `cd web && npx vitest run src/features/Applications/__tests__/ApplicationsPage.test.tsx`
Expected: All 8 tests pass.

- [ ] **Step 7: Commit**

```bash
git add web/src/features/Applications/ApplicationsPage.tsx \
  web/src/features/Applications/ApplicationsPage.module.css \
  web/src/features/Applications/useApplications.ts \
  web/src/features/Applications/__tests__/ApplicationsPage.test.tsx
git commit -m "feat(web): redesign ApplicationsPage with authority cards and breadcrumb"
```

---

### Task 5: Frontend — Update ConnectedApplicationsPage and API Adapter

**Files:**
- Modify: `web/src/features/Applications/ConnectedApplicationsPage.tsx`
- Modify: `web/src/api/applications.ts`

- [ ] **Step 1: Add the API function**

In `web/src/api/applications.ts`, add a new method to the returned object:

```typescript
// web/src/api/applications.ts
import type { ApiClient } from './client';
import type { AuthorityListItem, PlanningApplication } from '../domain/types';

interface UserApplicationAuthoritiesResponse {
  readonly authorities: readonly AuthorityListItem[];
  readonly count: number;
}

export function applicationsApi(client: ApiClient) {
  return {
    getMyAuthorities: () =>
      client
        .get<UserApplicationAuthoritiesResponse>('/v1/me/application-authorities')
        .then((r) => r.authorities),
    getByAuthority: (authorityId: number) =>
      client.get<readonly PlanningApplication[]>('/v1/applications', { authorityId: String(authorityId) }),
    getByUid: (uid: string) =>
      client.get<PlanningApplication>(`/v1/applications/${uid}`),
  };
}
```

- [ ] **Step 2: Update the connected page**

Replace the entire `ConnectedApplicationsPage.tsx`:

```tsx
// web/src/features/Applications/ConnectedApplicationsPage.tsx
import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { applicationsApi } from '../../api/applications';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { UserAuthoritiesPort } from '../../domain/ports/user-authorities-port';
import { ApplicationsPage } from './ApplicationsPage';

export function ConnectedApplicationsPage() {
  const client = useApiClient();

  const userAuthoritiesPort: UserAuthoritiesPort = useMemo(
    () => ({
      fetchMyAuthorities: () => applicationsApi(client).getMyAuthorities(),
    }),
    [client],
  );

  const browsePort: ApplicationsBrowsePort = useMemo(
    () => ({
      fetchByAuthority: (authorityId) =>
        applicationsApi(client).getByAuthority(authorityId),
    }),
    [client],
  );

  return <ApplicationsPage userAuthoritiesPort={userAuthoritiesPort} browsePort={browsePort} />;
}
```

- [ ] **Step 3: Run the full frontend test suite**

Run: `cd web && npx vitest run`
Expected: All tests pass. (The `useApplications` hook tests should still pass since `selectAuthority` is backwards-compatible — the tests pass `AuthorityListItem` which satisfies `AuthorityListItem | null`.)

- [ ] **Step 4: Run type check**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/features/Applications/ConnectedApplicationsPage.tsx \
  web/src/api/applications.ts
git commit -m "feat(web): wire ConnectedApplicationsPage to new authority endpoint"
```

---

### Task 6: Cleanup — Remove Unused AuthoritySearchPort Dependency

**Files:**
- Modify: `web/src/features/Applications/__tests__/useApplications.test.ts` (remove authority fixture import if unused)

The `AuthoritySelector` component and `AuthoritySearchPort` are still used by the Search page, so they stay. The only cleanup is removing the old authority search port import from the Applications page test — which was already replaced in Task 4.

- [ ] **Step 1: Verify no stale imports**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors.

- [ ] **Step 2: Run full test suites**

Run these in parallel:

```bash
cd web && npx vitest run
dotnet test api/
```

Expected: All tests pass in both projects.

- [ ] **Step 3: Commit if any cleanup was needed**

If the type check or tests revealed stale imports, fix and commit:

```bash
git add -u
git commit -m "refactor(web): remove unused authority search imports from Applications feature"
```
