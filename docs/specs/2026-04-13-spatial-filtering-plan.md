# Spatial Filtering for Watch Zone Applications — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the authority-scoped `GET /v1/applications?authorityId=` endpoint with a zone-scoped `GET /v1/me/watch-zones/{zoneId}/applications` that uses Cosmos `ST_DISTANCE` spatial filtering, and update both iOS and web clients to fetch by zone instead of authority.

**Architecture:** New CQRS query handler looks up the user's watch zone, extracts its centre/radius, and delegates to the existing `FindNearbyAsync` repository method. The old authority endpoint, handler, and query are deleted. iOS and web clients switch from authority-keyed fetching to zone-keyed fetching.

**Tech Stack:** .NET 10 (TUnit), Swift (XCTest), React/TypeScript (Vitest)

**Spec:** `docs/specs/spatial-filtering-watch-zones.md`

---

## File Map

### API — New/Modified
- **Create:** `api/src/town-crier.application/WatchZones/GetApplicationsByZoneQuery.cs`
- **Create:** `api/src/town-crier.application/WatchZones/GetApplicationsByZoneQueryHandler.cs`
- **Create:** `api/tests/town-crier.application.tests/WatchZones/GetApplicationsByZoneQueryHandlerTests.cs`
- **Modify:** `api/src/town-crier.application/WatchZones/IWatchZoneRepository.cs:6-19` — add `GetByUserAndZoneIdAsync`
- **Modify:** `api/src/town-crier.web/Endpoints/WatchZoneEndpoints.cs:9-127` — add applications sub-route
- **Modify:** `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs:157` — register new handler
- **Modify:** `api/tests/town-crier.application.tests/Polling/FakeWatchZoneRepository.cs:7-85` — implement new method

### API — Delete
- **Delete:** `api/src/town-crier.application/PlanningApplications/GetApplicationsByAuthorityQuery.cs`
- **Delete:** `api/src/town-crier.application/PlanningApplications/GetApplicationsByAuthorityQueryHandler.cs`
- **Delete:** `api/tests/town-crier.application.tests/PlanningApplications/GetApplicationsByAuthorityQueryHandlerTests.cs`
- **Modify:** `api/src/town-crier.web/Endpoints/PlanningApplicationEndpoints.cs:21-29` — remove `/applications` route
- **Modify:** `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs:157` — remove old handler registration

### iOS — Modified
- **Modify:** `mobile/ios/packages/town-crier-domain/Sources/Protocols/PlanningApplicationRepository.swift:1-5` — change `fetchApplications(for authority:)` to `fetchApplications(for zone:)`
- **Modify:** `mobile/ios/packages/town-crier-data/Sources/Repositories/APIPlanningApplicationRepository.swift:12-25` — call zone endpoint
- **Modify:** `mobile/ios/packages/town-crier-data/Sources/Repositories/InMemoryPlanningApplicationRepository.swift:13-19` — filter by zone
- **Modify:** `mobile/ios/packages/town-crier-domain/Sources/Entities/OfflineAwareRepository.swift:19-51` — cache by zone ID
- **Modify:** `mobile/ios/packages/town-crier-domain/Sources/Protocols/ApplicationCacheStore.swift:4-7` — key by `WatchZone`
- **Modify:** `mobile/ios/packages/town-crier-presentation/Sources/Features/ApplicationList/ApplicationListViewModel.swift` — accept zone, drop authority inits
- **Modify:** `mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapViewModel.swift` — fetch by zone
- **Modify:** `mobile/ios/town-crier-tests/Sources/Spies/SpyPlanningApplicationRepository.swift` — match new protocol

### Web — Modified
- **Modify:** `web/src/api/applications.ts:15-16` — replace `getByAuthority` with `getByZone`
- **Modify:** `web/src/domain/ports/applications-browse-port.ts:1-4` — `fetchByZone` replacing `fetchByAuthority`
- **Modify:** `web/src/domain/ports/map-port.ts:1-9` — `fetchApplicationsByZone` replacing `fetchApplicationsByAuthority`
- **Modify:** `web/src/features/Applications/useApplications.ts` — zone selector instead of authority
- **Modify:** `web/src/features/Applications/ApplicationsPage.tsx` — zone picker UI
- **Modify:** `web/src/features/Applications/ConnectedApplicationsPage.tsx` — wire zone port
- **Modify:** `web/src/features/Map/ApiMapAdapter.ts:20-22` — call zone endpoint
- **Modify:** `web/src/features/Map/useMapData.ts:40-48` — fan out by zone

---

## Task 1: API — Add `GetByUserAndZoneIdAsync` to repository interface

**Files:**
- Modify: `api/src/town-crier.application/WatchZones/IWatchZoneRepository.cs`
- Modify: `api/tests/town-crier.application.tests/Polling/FakeWatchZoneRepository.cs`
- Modify: `api/src/town-crier.infrastructure/WatchZones/CosmosWatchZoneRepository.cs`
- Modify: `api/src/town-crier.infrastructure/PlanningApplications/InMemoryPlanningApplicationRepository.cs` (only if it implements `IWatchZoneRepository` — check first)

The new handler needs to look up a specific zone by user+zone ID. The existing `IWatchZoneRepository` only has `GetByUserIdAsync` (returns all zones for a user). Add a targeted method.

- [ ] **Step 1: Add method to interface**

Add to `IWatchZoneRepository.cs` after the existing `GetByUserIdAsync` declaration:

```csharp
Task<WatchZone?> GetByUserAndZoneIdAsync(string userId, string zoneId, CancellationToken ct);
```

- [ ] **Step 2: Implement in `FakeWatchZoneRepository`**

Add to `FakeWatchZoneRepository.cs`:

```csharp
public Task<WatchZone?> GetByUserAndZoneIdAsync(string userId, string zoneId, CancellationToken ct)
{
    var zone = this.zones.FirstOrDefault(z => z.UserId == userId && z.Id == zoneId);
    return Task.FromResult(zone);
}
```

- [ ] **Step 3: Implement in `CosmosWatchZoneRepository`**

This is a point-read by document ID within the user partition — the most efficient Cosmos operation:

```csharp
public async Task<WatchZone?> GetByUserAndZoneIdAsync(string userId, string zoneId, CancellationToken ct)
{
    var document = await this.client.ReadDocumentAsync<WatchZoneDocument>(
        DatabaseId, ContainerId, zoneId, userId, ct).ConfigureAwait(false);

    return document?.ToDomain();
}
```

If `ReadDocumentAsync` returns `null` on 404 (check existing pattern in the codebase), this is sufficient. If it throws, wrap in try-catch and return `null`.

- [ ] **Step 4: Build to confirm compilation**

Run: `cd api && dotnet build`
Expected: Build succeeded

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.application/WatchZones/IWatchZoneRepository.cs \
        api/tests/town-crier.application.tests/Polling/FakeWatchZoneRepository.cs \
        api/src/town-crier.infrastructure/WatchZones/CosmosWatchZoneRepository.cs
git commit -m "feat(api): add GetByUserAndZoneIdAsync to IWatchZoneRepository"
```

---

## Task 2: API — Create `GetApplicationsByZoneQueryHandler` with tests

**Files:**
- Create: `api/src/town-crier.application/WatchZones/GetApplicationsByZoneQuery.cs`
- Create: `api/src/town-crier.application/WatchZones/GetApplicationsByZoneQueryHandler.cs`
- Create: `api/tests/town-crier.application.tests/WatchZones/GetApplicationsByZoneQueryHandlerTests.cs`

- [ ] **Step 1: Write the failing test — zone found, returns nearby applications**

Create `api/tests/town-crier.application.tests/WatchZones/GetApplicationsByZoneQueryHandlerTests.cs`:

```csharp
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.Tests.WatchZones;

public sealed class GetApplicationsByZoneQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnNearbyApplications_When_ZoneExists()
    {
        // Arrange — zone centred on Camden Town, 1 km radius
        var zone = new WatchZone("zone-1", "user-1", "My Zone",
            new Coordinates(51.5390, -0.1426), 1000, 42, DateTimeOffset.UtcNow);

        var watchZoneRepo = new FakeWatchZoneRepository();
        watchZoneRepo.Add(zone);

        // Application ~200m from centre (inside zone)
        var nearby = new PlanningApplicationBuilder()
            .WithUid("uid-nearby")
            .WithAreaId(42)
            .WithCoordinates(51.5380, -0.1410)
            .Build();

        // Application ~5km from centre (outside zone)
        var far = new PlanningApplicationBuilder()
            .WithUid("uid-far")
            .WithAreaId(42)
            .WithCoordinates(51.5074, -0.1278)
            .Build();

        var appRepo = new FakePlanningApplicationRepository();
        await appRepo.UpsertAsync(nearby, CancellationToken.None);
        await appRepo.UpsertAsync(far, CancellationToken.None);

        var handler = new GetApplicationsByZoneQueryHandler(watchZoneRepo, appRepo);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationsByZoneQuery("user-1", "zone-1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Count).IsEqualTo(1);
        await Assert.That(result[0].Uid).IsEqualTo("uid-nearby");
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && dotnet test --filter "Should_ReturnNearbyApplications_When_ZoneExists"`
Expected: FAIL — `GetApplicationsByZoneQuery` and `GetApplicationsByZoneQueryHandler` do not exist

- [ ] **Step 3: Create the query record**

Create `api/src/town-crier.application/WatchZones/GetApplicationsByZoneQuery.cs`:

```csharp
namespace TownCrier.Application.WatchZones;

public sealed record GetApplicationsByZoneQuery(string UserId, string ZoneId);
```

- [ ] **Step 4: Create the handler**

Create `api/src/town-crier.application/WatchZones/GetApplicationsByZoneQueryHandler.cs`:

```csharp
using System.Globalization;
using TownCrier.Application.PlanningApplications;

namespace TownCrier.Application.WatchZones;

public sealed class GetApplicationsByZoneQueryHandler
{
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly IPlanningApplicationRepository applicationRepository;

    public GetApplicationsByZoneQueryHandler(
        IWatchZoneRepository watchZoneRepository,
        IPlanningApplicationRepository applicationRepository)
    {
        this.watchZoneRepository = watchZoneRepository;
        this.applicationRepository = applicationRepository;
    }

    public async Task<IReadOnlyList<PlanningApplicationResult>?> HandleAsync(
        GetApplicationsByZoneQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var zone = await this.watchZoneRepository.GetByUserAndZoneIdAsync(
            query.UserId, query.ZoneId, ct).ConfigureAwait(false);

        if (zone is null)
        {
            return null;
        }

        var authorityCode = zone.AuthorityId.ToString(CultureInfo.InvariantCulture);
        var applications = await this.applicationRepository.FindNearbyAsync(
            authorityCode, zone.Centre.Latitude, zone.Centre.Longitude,
            zone.RadiusMetres, ct).ConfigureAwait(false);

        return applications.Select(GetApplicationByUidQueryHandler.ToResult).ToList();
    }
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd api && dotnet test --filter "Should_ReturnNearbyApplications_When_ZoneExists"`
Expected: PASS

- [ ] **Step 6: Write test — zone not found returns null**

Add to `GetApplicationsByZoneQueryHandlerTests.cs`:

```csharp
[Test]
public async Task Should_ReturnNull_When_ZoneNotFound()
{
    var watchZoneRepo = new FakeWatchZoneRepository();
    var appRepo = new FakePlanningApplicationRepository();
    var handler = new GetApplicationsByZoneQueryHandler(watchZoneRepo, appRepo);

    var result = await handler.HandleAsync(
        new GetApplicationsByZoneQuery("user-1", "nonexistent"), CancellationToken.None);

    await Assert.That(result).IsNull();
}
```

- [ ] **Step 7: Run test to verify it passes**

Run: `cd api && dotnet test --filter "Should_ReturnNull_When_ZoneNotFound"`
Expected: PASS (handler already returns null when zone is null)

- [ ] **Step 8: Write test — zone owned by different user returns null**

Add to `GetApplicationsByZoneQueryHandlerTests.cs`:

```csharp
[Test]
public async Task Should_ReturnNull_When_ZoneOwnedByDifferentUser()
{
    var zone = new WatchZone("zone-1", "other-user", "Their Zone",
        new Coordinates(51.5390, -0.1426), 1000, 42, DateTimeOffset.UtcNow);

    var watchZoneRepo = new FakeWatchZoneRepository();
    watchZoneRepo.Add(zone);

    var appRepo = new FakePlanningApplicationRepository();
    var handler = new GetApplicationsByZoneQueryHandler(watchZoneRepo, appRepo);

    var result = await handler.HandleAsync(
        new GetApplicationsByZoneQuery("user-1", "zone-1"), CancellationToken.None);

    await Assert.That(result).IsNull();
}
```

- [ ] **Step 9: Run test to verify it passes**

Run: `cd api && dotnet test --filter "Should_ReturnNull_When_ZoneOwnedByDifferentUser"`
Expected: PASS (user ID mismatch means `GetByUserAndZoneIdAsync` returns null)

- [ ] **Step 10: Run full test suite**

Run: `cd api && dotnet test`
Expected: All tests pass

- [ ] **Step 11: Commit**

```bash
git add api/src/town-crier.application/WatchZones/GetApplicationsByZoneQuery.cs \
        api/src/town-crier.application/WatchZones/GetApplicationsByZoneQueryHandler.cs \
        api/tests/town-crier.application.tests/WatchZones/GetApplicationsByZoneQueryHandlerTests.cs
git commit -m "feat(api): add GetApplicationsByZoneQueryHandler with spatial filtering"
```

---

## Task 3: API — Wire endpoint and remove old authority endpoint

**Files:**
- Modify: `api/src/town-crier.web/Endpoints/WatchZoneEndpoints.cs`
- Modify: `api/src/town-crier.web/Endpoints/PlanningApplicationEndpoints.cs`
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`
- Delete: `api/src/town-crier.application/PlanningApplications/GetApplicationsByAuthorityQuery.cs`
- Delete: `api/src/town-crier.application/PlanningApplications/GetApplicationsByAuthorityQueryHandler.cs`
- Delete: `api/tests/town-crier.application.tests/PlanningApplications/GetApplicationsByAuthorityQueryHandlerTests.cs`

- [ ] **Step 1: Add the new endpoint to `WatchZoneEndpoints.cs`**

Add after the existing `MapDelete("/me/watch-zones/{zoneId}", ...)` block (before the preferences endpoints):

```csharp
group.MapGet("/me/watch-zones/{zoneId}/applications", async (
    ClaimsPrincipal user,
    string zoneId,
    GetApplicationsByZoneQueryHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var result = await handler.HandleAsync(
        new GetApplicationsByZoneQuery(userId, zoneId), ct).ConfigureAwait(false);
    return result is null ? Results.NotFound() : Results.Ok(result);
});
```

Add `using TownCrier.Application.WatchZones;` to the usings if not already present (it should be).

- [ ] **Step 2: Register the new handler in DI**

In `ServiceCollectionExtensions.cs`, in the `AddApplicationServices` method, add after the `ListWatchZonesQueryHandler` registration:

```csharp
services.AddTransient<GetApplicationsByZoneQueryHandler>();
```

- [ ] **Step 3: Remove the old authority endpoint from `PlanningApplicationEndpoints.cs`**

Delete the entire `group.MapGet("/applications", ...)` block (lines 21-29). Keep the `/me/application-authorities` and `/applications/{**uid}` endpoints.

The file should now contain only:
```csharp
group.MapGet("/me/application-authorities", ...);
group.MapGet("/applications/{**uid}", ...);
```

- [ ] **Step 4: Remove old handler DI registration**

In `ServiceCollectionExtensions.cs`, remove the line:
```csharp
services.AddTransient<GetApplicationsByAuthorityQueryHandler>();
```

- [ ] **Step 5: Delete old handler, query, and tests**

Delete these files:
- `api/src/town-crier.application/PlanningApplications/GetApplicationsByAuthorityQuery.cs`
- `api/src/town-crier.application/PlanningApplications/GetApplicationsByAuthorityQueryHandler.cs`
- `api/tests/town-crier.application.tests/PlanningApplications/GetApplicationsByAuthorityQueryHandlerTests.cs`

- [ ] **Step 6: Remove the using for `GetApplicationsByAuthorityQueryHandler` from `PlanningApplicationEndpoints.cs` if it becomes unused**

Check if `TownCrier.Application.PlanningApplications` is still needed (yes — `GetApplicationByUidQueryHandler` and `GetUserApplicationAuthoritiesQueryHandler` are still there). So the using stays.

- [ ] **Step 7: Build and run all tests**

Run: `cd api && dotnet build && dotnet test`
Expected: All tests pass. The old handler tests are deleted, no compilation errors.

- [ ] **Step 8: Commit**

```bash
git add -A api/
git commit -m "feat(api): wire zone applications endpoint, remove authority endpoint

BREAKING: GET /v1/applications?authorityId removed.
Use GET /v1/me/watch-zones/{zoneId}/applications instead."
```

---

## Task 4: iOS — Change `PlanningApplicationRepository` protocol from authority to zone

**Files:**
- Modify: `mobile/ios/packages/town-crier-domain/Sources/Protocols/PlanningApplicationRepository.swift`
- Modify: `mobile/ios/packages/town-crier-data/Sources/Repositories/APIPlanningApplicationRepository.swift`
- Modify: `mobile/ios/packages/town-crier-data/Sources/Repositories/InMemoryPlanningApplicationRepository.swift`
- Modify: `mobile/ios/town-crier-tests/Sources/Spies/SpyPlanningApplicationRepository.swift`

- [ ] **Step 1: Write failing test — fetch by zone calls correct endpoint**

Update the existing test in `mobile/ios/town-crier-tests/Sources/Features/APIPlanningApplicationRepositoryTests.swift`. Change the first test to verify the zone endpoint:

```swift
@Test("fetchApplications sends GET /v1/me/watch-zones/{zoneId}/applications")
func fetchApplications_sendsCorrectRequest() async throws {
    let zone = try WatchZone(
        id: WatchZoneId("zone-123"),
        name: "Camden",
        centre: Coordinate(latitude: 51.539, longitude: -0.1426),
        radiusMetres: 1000,
        authorityId: 42
    )
    stub.stubbedData = "[]".data(using: .utf8)!
    stub.stubbedStatusCode = 200

    _ = try await sut.fetchApplications(for: zone)

    #expect(stub.capturedRequests.count == 1)
    let request = stub.capturedRequests[0]
    #expect(request.url?.path == "/v1/me/watch-zones/zone-123/applications")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd mobile/ios && swift test --filter "fetchApplications_sendsCorrectRequest"`
Expected: FAIL — protocol still expects `LocalAuthority`

- [ ] **Step 3: Update the protocol**

Replace `mobile/ios/packages/town-crier-domain/Sources/Protocols/PlanningApplicationRepository.swift`:

```swift
/// Port for accessing planning application data.
public protocol PlanningApplicationRepository: Sendable {
  func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication]
  func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication
}
```

- [ ] **Step 4: Update `APIPlanningApplicationRepository`**

Change the `fetchApplications` method in `APIPlanningApplicationRepository.swift`:

```swift
public func fetchApplications(for zone: WatchZone) async throws
    -> [PlanningApplication] {
    let dtos: [PlanningApplicationDTO]
    do {
        dtos = try await apiClient.request(
            .get("/v1/me/watch-zones/\(zone.id.value)/applications")
        )
    } catch let domainError as DomainError {
        throw domainError
    } catch {
        throw error.toDomainError()
    }
    return dtos.map { $0.toDomain() }
}
```

- [ ] **Step 5: Update `InMemoryPlanningApplicationRepository`**

Change the `fetchApplications` method:

```swift
public func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication] {
    return applications.filter { app in
        guard let location = app.location else { return false }
        return zone.contains(location)
    }
}
```

- [ ] **Step 6: Update `SpyPlanningApplicationRepository`**

Replace authority-based spy tracking with zone-based:

```swift
final class SpyPlanningApplicationRepository: PlanningApplicationRepository, @unchecked Sendable {
  private(set) var fetchApplicationsCalls: [WatchZone] = []
  var fetchApplicationsResult: Result<[PlanningApplication], Error> = .success([])

  /// Per-zone results. When set, takes precedence over `fetchApplicationsResult`.
  var fetchApplicationsByZone: [String: [PlanningApplication]] = [:]

  /// Zone IDs that should throw an error when fetched.
  var fetchApplicationsFailureZones: Set<String> = []

  func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication] {
    fetchApplicationsCalls.append(zone)
    if fetchApplicationsFailureZones.contains(zone.id.value) {
      throw DomainError.unexpected("Simulated failure for \(zone.id.value)")
    }
    if let perZone = fetchApplicationsByZone[zone.id.value] {
      return perZone
    }
    return try fetchApplicationsResult.get()
  }

  private(set) var fetchApplicationCalls: [PlanningApplicationId] = []
  var fetchApplicationResult: Result<PlanningApplication, Error> = .success(.pendingReview)

  func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication {
    fetchApplicationCalls.append(id)
    return try fetchApplicationResult.get()
  }
}
```

- [ ] **Step 7: Fix remaining compilation errors in tests**

Update all test files that use `fetchApplicationsByAuthority` or `fetchApplicationsCalls` on the spy to use the new zone-based API. Search for `fetchApplicationsByAuthority` and `fetchApplicationsCalls` in the test target. Each occurrence needs to construct a `WatchZone` instead of a `LocalAuthority`.

- [ ] **Step 8: Run all tests**

Run: `cd mobile/ios && swift test`
Expected: All tests pass

- [ ] **Step 9: Commit**

```bash
git add mobile/ios/
git commit -m "feat(ios): change PlanningApplicationRepository protocol from authority to zone"
```

---

## Task 5: iOS — Update `OfflineAwareRepository` and `ApplicationCacheStore` to key by zone

**Files:**
- Modify: `mobile/ios/packages/town-crier-domain/Sources/Protocols/ApplicationCacheStore.swift`
- Modify: `mobile/ios/packages/town-crier-domain/Sources/Entities/OfflineAwareRepository.swift`

- [ ] **Step 1: Update `ApplicationCacheStore` protocol**

Replace the protocol:

```swift
import Foundation

/// Port for local persistence of cached planning applications.
public protocol ApplicationCacheStore: Sendable {
  func store(_ entry: CacheEntry<[PlanningApplication]>, for zone: WatchZone) async
  func retrieve(for zone: WatchZone) async -> CacheEntry<[PlanningApplication]>?
}
```

- [ ] **Step 2: Update `OfflineAwareRepository`**

Change `fetchApplications` to accept a `WatchZone`:

```swift
public func fetchApplications(for zone: WatchZone) async throws -> CacheEntry<
    [PlanningApplication]
> {
    let cached = await cache.retrieve(for: zone)

    if let cached, cached.isFresh() {
        return cached
    }

    guard connectivity.isConnected else {
        if let cached {
            return cached
        }
        throw DomainError.networkUnavailable
    }

    do {
        let applications = try await remote.fetchApplications(for: zone)
        let entry = CacheEntry(data: applications, fetchedAt: Date())
        await cache.store(entry, for: zone)
        return entry
    } catch {
        if let cached {
            return cached
        }
        throw error
    }
}
```

- [ ] **Step 3: Fix any cache store implementations**

Search the codebase for classes conforming to `ApplicationCacheStore` and update their method signatures to accept `WatchZone` instead of `LocalAuthority`. The cache key should use `zone.id.value` instead of `authority.code`.

- [ ] **Step 4: Build and test**

Run: `cd mobile/ios && swift build && swift test`
Expected: All pass

- [ ] **Step 5: Commit**

```bash
git add mobile/ios/
git commit -m "feat(ios): key application cache by watch zone instead of authority"
```

---

## Task 6: iOS — Update `ApplicationListViewModel` to fetch by zone

**Files:**
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/ApplicationList/ApplicationListViewModel.swift`
- Modify: related test files

The view model currently has three init paths (single repo+authority, offlineRepo+authority, authorityRepo+applicationRepo). Simplify to two: `repository + zone` and `offlineRepository + zone`. The authority-aggregation path is no longer needed — each zone maps to one spatial query.

- [ ] **Step 1: Write failing test — loads applications for zone**

Update or add a test in `ApplicationListViewModelTests.swift`:

```swift
@Test func loadApplications_fetchesByZone() async throws {
    let zone = try WatchZone(
        id: WatchZoneId("zone-1"),
        name: "Camden",
        centre: Coordinate(latitude: 51.539, longitude: -0.1426),
        radiusMetres: 1000,
        authorityId: 42
    )
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsByZone = ["zone-1": [.pendingReview]]

    let sut = ApplicationListViewModel(repository: appSpy, zone: zone)
    await sut.loadApplications()

    #expect(appSpy.fetchApplicationsCalls.count == 1)
    #expect(appSpy.fetchApplicationsCalls[0].id == zone.id)
    #expect(sut.applications.count == 1)
}
```

- [ ] **Step 2: Run test to verify it fails**

Expected: FAIL — no `zone:` init parameter

- [ ] **Step 3: Refactor `ApplicationListViewModel`**

Replace the three init paths with two zone-based ones:

```swift
@MainActor
public final class ApplicationListViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var applications: [PlanningApplication] = []
  @Published var selectedStatusFilter: ApplicationStatus?
  @Published private(set) var isLoading = false
  @Published var error: DomainError?

  private let repository: PlanningApplicationRepository?
  private let offlineRepository: OfflineAwareRepository?
  private let zone: WatchZone
  private let tier: SubscriptionTier

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  public var canFilter: Bool { tier != .free }

  public var filteredApplications: [PlanningApplication] {
    guard canFilter, let filter = selectedStatusFilter else { return applications }
    return applications.filter { $0.status == filter }
  }

  public var isEmpty: Bool { filteredApplications.isEmpty && error == nil && !isLoading }
  public var isNetworkError: Bool { error == .networkUnavailable }
  public var isServerError: Bool { if case .serverError = error { return true }; return false }
  public var isSessionExpired: Bool { error == .sessionExpired }

  public init(
    repository: PlanningApplicationRepository,
    zone: WatchZone,
    tier: SubscriptionTier = .free
  ) {
    self.repository = repository
    self.offlineRepository = nil
    self.zone = zone
    self.tier = tier
  }

  public init(
    offlineRepository: OfflineAwareRepository,
    zone: WatchZone,
    tier: SubscriptionTier = .free
  ) {
    self.repository = nil
    self.offlineRepository = offlineRepository
    self.zone = zone
    self.tier = tier
  }

  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      let fetched: [PlanningApplication]
      if let offlineRepository {
        let entry = try await offlineRepository.fetchApplications(for: zone)
        fetched = entry.data
      } else if let repository {
        fetched = try await repository.fetchApplications(for: zone)
      } else {
        fetched = []
      }
      applications = fetched.sorted { $0.receivedDate > $1.receivedDate }
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  public func selectApplication(_ id: PlanningApplicationId) {
    onApplicationSelected?(id)
  }
}
```

- [ ] **Step 4: Fix all tests that construct `ApplicationListViewModel`**

Every test that uses `ApplicationListViewModel(repository:, authority:)` must change to `ApplicationListViewModel(repository:, zone:)`. Construct a `WatchZone` from the authority data.

- [ ] **Step 5: Run all tests**

Run: `cd mobile/ios && swift test`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add mobile/ios/
git commit -m "feat(ios): update ApplicationListViewModel to fetch by zone"
```

---

## Task 7: iOS — Update `MapViewModel` to fetch by zone

**Files:**
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapViewModel.swift`
- Modify: related test files

The map view model currently has three init paths like the list. Simplify: it already loads the first watch zone for centre/radius — now also use that zone to fetch applications.

- [ ] **Step 1: Write failing test — map fetches by zone**

```swift
@Test func loadApplications_fetchesBySelectedZone() async throws {
    let zone = try WatchZone(
        id: WatchZoneId("zone-1"),
        name: "Camden",
        centre: Coordinate(latitude: 51.539, longitude: -0.1426),
        radiusMetres: 1000,
        authorityId: 42
    )
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsByZone = ["zone-1": [.pendingReview]]

    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.zones = [zone]

    let sut = MapViewModel(repository: appSpy, watchZoneRepository: zoneSpy)
    await sut.loadApplications()

    #expect(appSpy.fetchApplicationsCalls.count == 1)
    #expect(appSpy.fetchApplicationsCalls[0].id == zone.id)
}
```

- [ ] **Step 2: Run test to verify it fails**

Expected: FAIL — `MapViewModel` still calls `fetchApplications(for: LocalAuthority(...))`

- [ ] **Step 3: Refactor `MapViewModel`**

Simplify to use zones. The key change: after loading the first zone for map centre, use it to fetch applications:

```swift
@MainActor
public final class MapViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var annotations: [MapAnnotationItem] = []
  @Published private(set) var isLoading = false
  @Published var error: DomainError?
  @Published private(set) var selectedApplication: PlanningApplication?
  @Published private(set) var hasLoaded = false

  @Published private(set) var centreLat: Double = 51.5074
  @Published private(set) var centreLon: Double = -0.1278
  @Published private(set) var radiusMetres: Double = 2000

  private let repository: PlanningApplicationRepository
  private let watchZoneRepository: WatchZoneRepository
  private var applications: [PlanningApplication] = []

  public var isEmpty: Bool { hasLoaded && annotations.isEmpty && error == nil && !isLoading }
  public var isNetworkError: Bool { error == .networkUnavailable }
  public var isServerError: Bool { if case .serverError = error { return true }; return false }
  public var isSessionExpired: Bool { error == .sessionExpired }

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  public init(repository: PlanningApplicationRepository, watchZoneRepository: WatchZoneRepository) {
    self.repository = repository
    self.watchZoneRepository = watchZoneRepository
  }

  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      let zones = try await watchZoneRepository.loadAll()
      guard let zone = zones.first else {
        isLoading = false
        hasLoaded = true
        return
      }

      centreLat = zone.centre.latitude
      centreLon = zone.centre.longitude
      radiusMetres = zone.radiusMetres

      let fetched = try await repository.fetchApplications(for: zone)
      applications = fetched
      annotations = fetched.compactMap { app in
        guard let location = app.location else { return nil }
        return MapAnnotationItem(application: app, coordinate: location)
      }
    } catch {
      handleError(error)
    }
    isLoading = false
    hasLoaded = true
  }

  public func selectApplication(_ id: PlanningApplicationId) {
    selectedApplication = applications.first { $0.id == id }
    onApplicationSelected?(id)
  }

  public func clearSelection() {
    selectedApplication = nil
  }
}
```

- [ ] **Step 4: Fix all tests that construct `MapViewModel`**

Remove the multi-init paths. Tests that used the `authorityRepository + applicationRepository` init need to change to the single `repository + watchZoneRepository` init with zone data.

- [ ] **Step 5: Run all tests**

Run: `cd mobile/ios && swift test`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add mobile/ios/
git commit -m "feat(ios): update MapViewModel to fetch applications by zone"
```

---

## Task 8: Web — Change API layer and ports from authority to zone

**Files:**
- Modify: `web/src/api/applications.ts`
- Modify: `web/src/domain/ports/applications-browse-port.ts`
- Modify: `web/src/domain/ports/map-port.ts`

- [ ] **Step 1: Update `applications-browse-port.ts`**

```typescript
import type { WatchZoneId, PlanningApplicationSummary } from '../types';

export interface ApplicationsBrowsePort {
  fetchByZone(zoneId: WatchZoneId): Promise<readonly PlanningApplicationSummary[]>;
}
```

- [ ] **Step 2: Update `map-port.ts`**

Replace `fetchApplicationsByAuthority` with `fetchApplicationsByZone`:

```typescript
import type { ApplicationUid, WatchZoneId, WatchZoneSummary, PlanningApplication, SavedApplication } from '../types';

export interface MapPort {
  fetchMyZones(): Promise<readonly WatchZoneSummary[]>;
  fetchApplicationsByZone(zoneId: WatchZoneId): Promise<readonly PlanningApplication[]>;
  fetchSavedApplications(): Promise<readonly SavedApplication[]>;
  saveApplication(uid: ApplicationUid): Promise<void>;
  unsaveApplication(uid: ApplicationUid): Promise<void>;
}
```

- [ ] **Step 3: Add `getByZone` to `applicationsApi`, remove `getByAuthority`**

```typescript
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
    getByZone: (zoneId: string) =>
      client.get<readonly PlanningApplication[]>(`/v1/me/watch-zones/${zoneId}/applications`),
    getByUid: (uid: string) =>
      client.get<PlanningApplication>(`/v1/applications/${uid}`),
  };
}
```

- [ ] **Step 4: Verify type check**

Run: `cd web && npx tsc --noEmit`
Expected: FAIL — downstream consumers of the old ports don't compile yet (this is expected; Tasks 9-10 fix them)

- [ ] **Step 5: Commit**

```bash
git add web/src/api/applications.ts \
        web/src/domain/ports/applications-browse-port.ts \
        web/src/domain/ports/map-port.ts
git commit -m "feat(web): change application ports from authority-scoped to zone-scoped"
```

---

## Task 9: Web — Update `useApplications`, `ApplicationsPage`, and `ConnectedApplicationsPage` to zone-based

**Files:**
- Modify: `web/src/features/Applications/useApplications.ts`
- Modify: `web/src/features/Applications/ApplicationsPage.tsx`
- Modify: `web/src/features/Applications/ConnectedApplicationsPage.tsx`
- Modify: `web/src/features/Applications/__tests__/useApplications.test.ts`
- Modify: `web/src/features/Applications/__tests__/spies/spy-applications-browse-port.ts`
- Modify: `web/src/features/Applications/__tests__/ApplicationsPage.test.tsx`

- [ ] **Step 1: Update the spy**

Replace `spy-applications-browse-port.ts`:

```typescript
import type { ApplicationsBrowsePort } from '../../../../domain/ports/applications-browse-port';
import type { WatchZoneId, PlanningApplicationSummary } from '../../../../domain/types';

export function spyApplicationsBrowsePort(): ApplicationsBrowsePort & {
  fetchByZoneCalls: WatchZoneId[];
  fetchByZoneResults: Map<string, readonly PlanningApplicationSummary[]>;
} {
  const spy = {
    fetchByZoneCalls: [] as WatchZoneId[],
    fetchByZoneResults: new Map<string, readonly PlanningApplicationSummary[]>(),
    async fetchByZone(zoneId: WatchZoneId) {
      spy.fetchByZoneCalls.push(zoneId);
      return spy.fetchByZoneResults.get(zoneId as string) ?? [];
    },
  };
  return spy;
}
```

- [ ] **Step 2: Update `useApplications.ts`**

Change from authority selection to zone selection:

```typescript
import { useState, useCallback } from 'react';
import type { WatchZoneSummary, PlanningApplicationSummary } from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import { useFetchData } from '../../hooks/useFetchData';

export function useApplications(port: ApplicationsBrowsePort) {
  const [selectedZone, setSelectedZone] = useState<WatchZoneSummary | null>(null);
  const [fetchKey, setFetchKey] = useState(0);

  const { data, isLoading, error } = useFetchData<readonly PlanningApplicationSummary[]>(
    () => port.fetchByZone(selectedZone!.id),
    [selectedZone?.id, fetchKey],
    { enabled: selectedZone !== null },
  );

  const selectZone = useCallback(
    (zone: WatchZoneSummary | null) => {
      setSelectedZone(zone);
      setFetchKey((k) => k + 1);
    },
    [],
  );

  return {
    selectedZone,
    applications: data ?? [],
    isLoading,
    error,
    selectZone,
  };
}
```

- [ ] **Step 3: Update `ApplicationsPage.tsx`**

Replace authority-based UI with zone-based. The page now shows zone cards instead of authority cards. Update the component to accept a zone-loading port (reuse the existing `watchZonesApi`):

```typescript
import { useNavigate } from 'react-router-dom';
import type { WatchZoneSummary } from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import { useApplications } from './useApplications';
import { ApplicationCard } from '../../components/ApplicationCard/ApplicationCard';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import { useFetchData } from '../../hooks/useFetchData';
import styles from './ApplicationsPage.module.css';

interface ZonesPort {
  fetchZones(): Promise<readonly WatchZoneSummary[]>;
}

interface Props {
  zonesPort: ZonesPort;
  browsePort: ApplicationsBrowsePort;
}

export function ApplicationsPage({ zonesPort, browsePort }: Props) {
  const navigate = useNavigate();
  const { data: zones, isLoading: isLoadingZones, error: zonesError } =
    useFetchData(() => zonesPort.fetchZones(), [zonesPort]);
  const { selectedZone, applications, isLoading: isLoadingApps, error: appsError, selectZone } =
    useApplications(browsePort);

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Applications</h1>

      {selectedZone !== null && (
        <nav className={styles.breadcrumb} aria-label="Breadcrumb">
          <button className={styles.breadcrumbLink} onClick={() => selectZone(null)}>
            Watch Zones
          </button>
          <span aria-hidden="true">&rsaquo;</span>
          <span className={styles.breadcrumbCurrent}>{selectedZone.name}</span>
        </nav>
      )}

      {selectedZone === null && (
        <>
          {isLoadingZones && (
            <div className={styles.loading} aria-live="polite">Loading zones...</div>
          )}

          {zonesError !== null && (
            <EmptyState title="Something went wrong" message={zonesError.message} />
          )}

          {!isLoadingZones && zonesError === null && (zones ?? []).length === 0 && (
            <EmptyState
              icon="📍"
              title="No watch zones yet"
              message="Set up a watch zone to start browsing applications."
              actionLabel="Create watch zone"
              onAction={() => navigate('/watch-zones/new')}
            />
          )}

          {!isLoadingZones && zonesError === null && (zones ?? []).length > 0 && (
            <div className={styles.authorityGrid}>
              {(zones ?? []).map((zone) => (
                <button
                  key={zone.id}
                  className={styles.authorityCard}
                  onClick={() => selectZone(zone)}
                >
                  <span className={styles.authorityName}>{zone.name}</span>
                </button>
              ))}
            </div>
          )}
        </>
      )}

      {selectedZone !== null && (
        <>
          {isLoadingApps && (
            <div className={styles.loading} aria-live="polite">Loading applications...</div>
          )}

          {appsError !== null && (
            <EmptyState
              title="Something went wrong"
              message={appsError}
              actionLabel="Try again"
              onAction={() => selectZone(selectedZone)}
            />
          )}

          {!isLoadingApps && appsError === null && applications.length === 0 && (
            <EmptyState
              icon="📋"
              title="No applications"
              message="No applications found in this zone."
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

- [ ] **Step 4: Update `ConnectedApplicationsPage.tsx`**

Wire the zone ports:

```typescript
import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { applicationsApi } from '../../api/applications';
import { watchZonesApi } from '../../api/watchZones';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import { ApplicationsPage } from './ApplicationsPage';

export function ConnectedApplicationsPage() {
  const client = useApiClient();

  const browsePort: ApplicationsBrowsePort = useMemo(
    () => ({
      fetchByZone: (zoneId) =>
        applicationsApi(client).getByZone(zoneId as string),
    }),
    [client],
  );

  const zonesPort = useMemo(
    () => ({
      fetchZones: () => watchZonesApi(client).list(),
    }),
    [client],
  );

  return <ApplicationsPage zonesPort={zonesPort} browsePort={browsePort} />;
}
```

- [ ] **Step 5: Update tests**

Update `useApplications.test.ts` and `ApplicationsPage.test.tsx` to use zone-based spies and assertions. Replace authority references with zone references throughout.

- [ ] **Step 6: Type check and run tests**

Run: `cd web && npx tsc --noEmit && npx vitest run`
Expected: Type check may still fail if Map feature not updated yet — that's Task 10. Run Application-specific tests only if needed: `cd web && npx vitest run --grep "Applications"`

- [ ] **Step 7: Commit**

```bash
git add web/src/features/Applications/ web/src/domain/ports/applications-browse-port.ts
git commit -m "feat(web): update Applications feature to fetch by zone"
```

---

## Task 10: Web — Update Map feature from authority to zone

**Files:**
- Modify: `web/src/features/Map/ApiMapAdapter.ts`
- Modify: `web/src/features/Map/useMapData.ts`
- Modify: `web/src/features/Map/__tests__/spies/spy-map-port.ts`
- Modify: `web/src/features/Map/__tests__/useMapData.test.ts`
- Modify: `web/src/features/Map/__tests__/MapPage.test.tsx`

- [ ] **Step 1: Update `spy-map-port.ts`**

Replace authority-based spy methods with zone-based:

```typescript
import type { MapPort } from '../../../../domain/ports/map-port';
import type { ApplicationUid, WatchZoneId, WatchZoneSummary, PlanningApplication, SavedApplication } from '../../../../domain/types';

export function spyMapPort(): MapPort & {
  fetchMyZonesCalls: number;
  fetchMyZonesResult: readonly WatchZoneSummary[];
  fetchApplicationsByZoneCalls: WatchZoneId[];
  fetchApplicationsByZoneResults: Map<string, readonly PlanningApplication[]>;
  fetchApplicationsByZoneError: Error | null;
  fetchSavedApplicationsResult: readonly SavedApplication[];
  saveApplicationCalls: ApplicationUid[];
  unsaveApplicationCalls: ApplicationUid[];
} {
  const spy = {
    fetchMyZonesCalls: 0,
    fetchMyZonesResult: [] as readonly WatchZoneSummary[],
    fetchApplicationsByZoneCalls: [] as WatchZoneId[],
    fetchApplicationsByZoneResults: new Map<string, readonly PlanningApplication[]>(),
    fetchApplicationsByZoneError: null as Error | null,
    fetchSavedApplicationsResult: [] as readonly SavedApplication[],
    saveApplicationCalls: [] as ApplicationUid[],
    unsaveApplicationCalls: [] as ApplicationUid[],

    async fetchMyZones() {
      spy.fetchMyZonesCalls++;
      return spy.fetchMyZonesResult;
    },
    async fetchApplicationsByZone(zoneId: WatchZoneId) {
      spy.fetchApplicationsByZoneCalls.push(zoneId);
      if (spy.fetchApplicationsByZoneError) throw spy.fetchApplicationsByZoneError;
      return spy.fetchApplicationsByZoneResults.get(zoneId as string) ?? [];
    },
    async fetchSavedApplications() {
      return spy.fetchSavedApplicationsResult;
    },
    async saveApplication(uid: ApplicationUid) {
      spy.saveApplicationCalls.push(uid);
    },
    async unsaveApplication(uid: ApplicationUid) {
      spy.unsaveApplicationCalls.push(uid);
    },
  };
  return spy;
}
```

- [ ] **Step 2: Update `useMapData.ts`**

Change from authority fan-out to zone fan-out:

```typescript
import { useState, useMemo, type Dispatch, type SetStateAction } from 'react';
import type { ApplicationUid, PlanningApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { useFetchData } from '../../hooks/useFetchData';

interface MapData {
  readonly applications: readonly PlanningApplication[];
  readonly fetchedSavedUids: ReadonlySet<ApplicationUid>;
}

type UidSetSetter = Dispatch<SetStateAction<Set<ApplicationUid>>>;

function makeToggle(
  setAdd: UidSetSetter,
  setRemove: UidSetSetter,
  portAction: (uid: ApplicationUid) => Promise<void>,
) {
  return async (uid: ApplicationUid) => {
    setRemove(prev => {
      const next = new Set(prev);
      next.delete(uid);
      return next;
    });
    setAdd(prev => new Set([...prev, uid]));
    try {
      await portAction(uid);
    } catch {
      setAdd(prev => {
        const next = new Set(prev);
        next.delete(uid);
        return next;
      });
    }
  };
}

export function useMapData(port: MapPort) {
  const { data, isLoading, error, refresh } = useFetchData<MapData>(
    async () => {
      const [zones, savedApps] = await Promise.all([
        port.fetchMyZones(),
        port.fetchSavedApplications(),
      ]);

      const applicationArrays = await Promise.all(
        zones.map(z => port.fetchApplicationsByZone(z.id)),
      );

      // Deduplicate across zones (applications may appear in overlapping zones)
      const seen = new Set<string>();
      const deduped: PlanningApplication[] = [];
      for (const apps of applicationArrays) {
        for (const app of apps) {
          if (!seen.has(app.uid as string)) {
            seen.add(app.uid as string);
            deduped.push(app);
          }
        }
      }

      return {
        applications: deduped,
        fetchedSavedUids: new Set(savedApps.map(s => s.applicationUid)),
      };
    },
    [port],
  );

  const [pendingSaves, setPendingSaves] = useState(new Set<ApplicationUid>());
  const [pendingRemoves, setPendingRemoves] = useState(new Set<ApplicationUid>());

  const savedUids: ReadonlySet<ApplicationUid> = useMemo(() => {
    const result = new Set(data?.fetchedSavedUids ?? []);
    for (const uid of pendingSaves) result.add(uid);
    for (const uid of pendingRemoves) result.delete(uid);
    return result;
  }, [data?.fetchedSavedUids, pendingSaves, pendingRemoves]);

  const saveApplication = useMemo(
    () => makeToggle(setPendingSaves, setPendingRemoves, uid => port.saveApplication(uid)),
    [port],
  );

  const unsaveApplication = useMemo(
    () => makeToggle(setPendingRemoves, setPendingSaves, uid => port.unsaveApplication(uid)),
    [port],
  );

  return {
    applications: data?.applications ?? [],
    savedUids,
    isLoading,
    error,
    refresh,
    saveApplication,
    unsaveApplication,
  };
}
```

- [ ] **Step 3: Update `ApiMapAdapter.ts`**

```typescript
import type { ApiClient } from '../../api/client';
import type { ApplicationUid, WatchZoneId, WatchZoneSummary, PlanningApplication, SavedApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { applicationsApi } from '../../api/applications';
import { watchZonesApi } from '../../api/watchZones';
import { savedApplicationsApi } from '../../api/savedApplications';

export class ApiMapAdapter implements MapPort {
  private readonly apps: ReturnType<typeof applicationsApi>;
  private readonly zones: ReturnType<typeof watchZonesApi>;
  private readonly saved: ReturnType<typeof savedApplicationsApi>;

  constructor(client: ApiClient) {
    this.apps = applicationsApi(client);
    this.zones = watchZonesApi(client);
    this.saved = savedApplicationsApi(client);
  }

  async fetchMyZones(): Promise<readonly WatchZoneSummary[]> {
    return this.zones.list();
  }

  async fetchApplicationsByZone(zoneId: WatchZoneId): Promise<readonly PlanningApplication[]> {
    return this.apps.getByZone(zoneId as string);
  }

  async fetchSavedApplications(): Promise<readonly SavedApplication[]> {
    return this.saved.list();
  }

  async saveApplication(uid: ApplicationUid): Promise<void> {
    await this.saved.save(uid as string);
  }

  async unsaveApplication(uid: ApplicationUid): Promise<void> {
    await this.saved.remove(uid as string);
  }
}
```

- [ ] **Step 4: Update `useMapData.test.ts` and `MapPage.test.tsx`**

Replace all `fetchApplicationsByAuthority` references with `fetchApplicationsByZone`. Replace `fetchMyAuthorities` calls with `fetchMyZones`. Construct zone data instead of authority data. Update assertion checks.

- [ ] **Step 5: Type check and run all tests**

Run: `cd web && npx tsc --noEmit && npx vitest run`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add web/src/features/Map/ web/src/domain/ports/map-port.ts
git commit -m "feat(web): update Map feature to fetch applications by zone"
```

---

## Task 11: Web — Remove dead `UserAuthoritiesPort` and cleanup

**Files:**
- Check and potentially delete: `web/src/domain/ports/user-authorities-port.ts`
- Check and potentially delete: `web/src/features/Applications/useUserAuthorities.ts`
- Modify: any remaining references

- [ ] **Step 1: Search for remaining `UserAuthoritiesPort` usage**

Search for `UserAuthoritiesPort` and `useUserAuthorities` across the web codebase. If the only consumer was `ApplicationsPage` (which now uses `ZonesPort`), these are dead code.

- [ ] **Step 2: Delete dead files if confirmed unused**

Remove the port and hook if no other feature uses them.

- [ ] **Step 3: Type check and run all tests**

Run: `cd web && npx tsc --noEmit && npx vitest run`
Expected: All pass

- [ ] **Step 4: Commit**

```bash
git add -A web/
git commit -m "chore(web): remove unused UserAuthoritiesPort after zone migration"
```
