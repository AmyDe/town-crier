import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("DashboardViewModel")
@MainActor
struct DashboardViewModelTests {

  // MARK: - Helpers

  private func makeSUT(
    tier: SubscriptionTier = .free,
    zones: [WatchZone] = [],
    authorities: [LocalAuthority] = [],
    authorityCount: Int? = nil
  ) -> (DashboardViewModel, SpyWatchZoneRepository, SpyApplicationAuthorityRepository) {
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success(zones)
    let authoritySpy = SpyApplicationAuthorityRepository()
    authoritySpy.fetchAuthoritiesResult = .success(
      ApplicationAuthorityResult(
        authorities: authorities,
        count: authorityCount ?? authorities.count
      )
    )
    let vm = DashboardViewModel(
      watchZoneRepository: zoneSpy,
      authorityRepository: authoritySpy,
      featureGate: FeatureGate(tier: tier)
    )
    return (vm, zoneSpy, authoritySpy)
  }

  // MARK: - Load

  @Test("load populates zones on success")
  func load_populatesZones() async {
    let (sut, _, _) = makeSUT(zones: [.cambridge, .london])

    await sut.load()

    #expect(sut.zones == [.cambridge, .london])
  }

  @Test("load populates authorities on success")
  func load_populatesAuthorities() async {
    let bath = LocalAuthority(code: "123", name: "Bath and NE Somerset")
    let (sut, _, _) = makeSUT(authorities: [bath])

    await sut.load()

    #expect(sut.authorities == [bath])
  }

  @Test("load calls both repositories")
  func load_callsBothRepositories() async {
    let (sut, zoneSpy, authoritySpy) = makeSUT()

    await sut.load()

    #expect(zoneSpy.loadAllCallCount == 1)
    #expect(authoritySpy.fetchAuthoritiesCallCount == 1)
  }

  @Test("load sets isLoading false after completion")
  func load_setsIsLoadingFalse() async {
    let (sut, _, _) = makeSUT()

    await sut.load()

    #expect(!sut.isLoading)
  }

  @Test("load sets error when zone fetch fails")
  func load_setsErrorOnZoneFailure() async {
    let (sut, zoneSpy, _) = makeSUT()
    zoneSpy.loadAllResult = .failure(DomainError.networkUnavailable)

    await sut.load()

    #expect(sut.error == .networkUnavailable)
  }

  @Test("load sets error when authority fetch fails")
  func load_setsErrorOnAuthorityFailure() async {
    let (sut, _, authoritySpy) = makeSUT()
    authoritySpy.fetchAuthoritiesResult = .failure(DomainError.networkUnavailable)

    await sut.load()

    #expect(sut.error == .networkUnavailable)
  }

  @Test("load clears error on retry")
  func load_clearsErrorOnRetry() async {
    let (sut, zoneSpy, _) = makeSUT()
    zoneSpy.loadAllResult = .failure(DomainError.networkUnavailable)
    await sut.load()

    zoneSpy.loadAllResult = .success([.cambridge])
    await sut.load()

    #expect(sut.error == nil)
    #expect(sut.zones.count == 1)
  }

  @Test("load still populates authorities when zone fetch fails")
  func load_populatesAuthoritiesWhenZoneFails() async {
    let bath = LocalAuthority(code: "123", name: "Bath and NE Somerset")
    let (sut, zoneSpy, _) = makeSUT(authorities: [bath])
    zoneSpy.loadAllResult = .failure(DomainError.networkUnavailable)

    await sut.load()

    #expect(sut.authorities == [bath])
  }

  @Test("load still populates zones when authority fetch fails")
  func load_populatesZonesWhenAuthorityFails() async {
    let (sut, _, authoritySpy) = makeSUT(zones: [.cambridge])
    authoritySpy.fetchAuthoritiesResult = .failure(DomainError.networkUnavailable)

    await sut.load()

    #expect(sut.zones == [.cambridge])
  }

  // MARK: - Feature Gate

  @Test("featureGate exposes injected gate")
  func featureGate_exposesInjectedGate() {
    let gate = FeatureGate(tier: .personal)
    let (sut, _, _) = makeSUT(tier: .personal)

    #expect(sut.featureGate == gate)
  }

  @Test("zoneCount returns the count of loaded zones")
  func zoneCount_returnsLoadedCount() async {
    let (sut, _, _) = makeSUT(zones: [.cambridge, .london])

    await sut.load()

    #expect(sut.zoneCount == 2)
  }

  @Test("authorityCount returns the count from the result")
  func authorityCount_returnsResultCount() async {
    let bath = LocalAuthority(code: "123", name: "Bath and NE Somerset")
    let (sut, _, _) = makeSUT(authorities: [bath], authorityCount: 1)

    await sut.load()

    #expect(sut.authorityCount == 1)
  }

  // MARK: - Navigation Callbacks

  @Test("navigateToZones invokes onNavigateToZones")
  func navigateToZones_invokesCallback() {
    let (sut, _, _) = makeSUT()
    var called = false
    sut.onNavigateToZones = { called = true }

    sut.navigateToZones()

    #expect(called)
  }

  @Test("navigateToSaved invokes onNavigateToSaved")
  func navigateToSaved_invokesCallback() {
    let (sut, _, _) = makeSUT()
    var called = false
    sut.onNavigateToSaved = { called = true }

    sut.navigateToSaved()

    #expect(called)
  }

  @Test("navigateToNotifications invokes onNavigateToNotifications")
  func navigateToNotifications_invokesCallback() {
    let (sut, _, _) = makeSUT()
    var called = false
    sut.onNavigateToNotifications = { called = true }

    sut.navigateToNotifications()

    #expect(called)
  }

  @Test("navigateToMap invokes onNavigateToMap")
  func navigateToMap_invokesCallback() {
    let (sut, _, _) = makeSUT()
    var called = false
    sut.onNavigateToMap = { called = true }

    sut.navigateToMap()

    #expect(called)
  }

  @Test("navigateToAuthority invokes onNavigateToAuthority with authority")
  func navigateToAuthority_invokesCallback() {
    let bath = LocalAuthority(code: "123", name: "Bath and NE Somerset")
    let (sut, _, _) = makeSUT()
    var receivedAuthority: LocalAuthority?
    sut.onNavigateToAuthority = { receivedAuthority = $0 }

    sut.navigateToAuthority(bath)

    #expect(receivedAuthority == bath)
  }

  // MARK: - Empty States

  @Test("hasZones is false before loading")
  func hasZones_beforeLoad_returnsFalse() {
    let (sut, _, _) = makeSUT()

    #expect(!sut.hasZones)
  }

  @Test("hasZones is true when zones exist")
  func hasZones_withZones_returnsTrue() async {
    let (sut, _, _) = makeSUT(zones: [.cambridge])

    await sut.load()

    #expect(sut.hasZones)
  }

  @Test("hasAuthorities is false before loading")
  func hasAuthorities_beforeLoad_returnsFalse() {
    let (sut, _, _) = makeSUT()

    #expect(!sut.hasAuthorities)
  }

  @Test("hasAuthorities is true when authorities exist")
  func hasAuthorities_withAuthorities_returnsTrue() async {
    let bath = LocalAuthority(code: "123", name: "Bath and NE Somerset")
    let (sut, _, _) = makeSUT(authorities: [bath])

    await sut.load()

    #expect(sut.hasAuthorities)
  }
}
