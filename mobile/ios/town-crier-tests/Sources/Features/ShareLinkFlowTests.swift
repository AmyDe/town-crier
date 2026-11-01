import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData
@testable import TownCrierPresentation

@Suite("Share link flow")
@MainActor
struct ShareLinkFlowTests {
  private var kingston: PlanningApplication {
    PlanningApplication(
      id: PlanningApplicationId(authority: "789", name: "Kingston/25/02755/CLC"),
      reference: ApplicationReference("Kingston/25/02755/CLC"),
      authority: LocalAuthority(code: "789", name: "Kingston upon Thames", slug: "kingston"),
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "Certificate of lawfulness",
      address: "1 Market Place, Kingston, KT1 1JS"
    )
  }

  private func makeCoordinator(hasSession: Bool) -> (
    AppCoordinator, SpyPlanningApplicationRepository, SpyAnonymousApplicationDetailRepository
  ) {
    let planningSpy = SpyPlanningApplicationRepository()
    planningSpy.fetchApplicationResult = .success(kingston)
    let anonSpy = SpyAnonymousApplicationDetailRepository()
    anonSpy.fetchApplicationBySlugResult = .success(kingston)
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = hasSession ? .valid : nil
    let coordinator = AppCoordinator(
      repository: planningSpy,
      authService: authSpy,
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      anonymousApplicationDetailRepository: anonSpy
    )
    return (coordinator, planningSpy, anonSpy)
  }

  @Test func eachSheetOpenGetsAFreshCode() throws {
    let sut = ApplicationDetailViewModel(application: kingston)

    let first = try #require(sut.makeSharePresentation())
    let second = try #require(sut.makeSharePresentation())

    #expect(first.url != second.url)
    #expect(first.id != second.id)
    #expect(
      first.url.absoluteString.wholeMatch(
        of: /https:\/\/share\.towncrierapp\.uk\/a\/kingston\/Kingston\/25\/02755\/CLC\?s=[A-Za-z0-9]{6}/
      ) != nil)
  }

  @Test func theCleanShareURLCarriesNoCode() {
    let sut = ApplicationDetailViewModel(application: kingston)

    #expect(sut.shareURL?.query == nil)
  }

  @Test func noSlugMeansNoPresentation() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(sut.makeSharePresentation() == nil)
  }

  @Test func anonymousOpen_passesTheCodeOnTheFirstReadOnly() async {
    let (sut, _, anonSpy) = makeCoordinator(hasSession: false)

    sut.handleDeepLink(
      .shareApplication(authoritySlug: "kingston", ref: "Kingston/25/02755/CLC", linkCode: "aZ09bY"))
    await sut.waitForPendingDetailLoad()
    let viewModel = sut.makeApplicationDetailViewModel(application: kingston)
    await viewModel.refresh()

    #expect(anonSpy.fetchApplicationBySlugCalls.map(\.linkCode) == ["aZ09bY", nil])
  }

  @Test func signedInOpen_neverPassesTheCode() async {
    let (sut, planningSpy, anonSpy) = makeCoordinator(hasSession: true)

    sut.handleDeepLink(
      .shareApplication(authoritySlug: "kingston", ref: "Kingston/25/02755/CLC", linkCode: "aZ09bY"))
    await sut.waitForPendingDetailLoad()

    #expect(anonSpy.fetchApplicationBySlugCalls.isEmpty)
    #expect(planningSpy.fetchApplicationBySlugCalls.count == 1)
  }

  @Test func linkWithNoCode_sendsNoCode() async {
    let (sut, _, anonSpy) = makeCoordinator(hasSession: false)

    sut.handleDeepLink(.shareApplication(authoritySlug: "kingston", ref: "Kingston/25/02755/CLC"))
    await sut.waitForPendingDetailLoad()

    #expect(anonSpy.fetchApplicationBySlugCalls.map(\.linkCode) == [nil])
  }

  @Test func finishedShare_reportsTheDestination() async {
    let reporter = SpyShareEventReporter()
    let sut = ApplicationDetailViewModel(application: kingston, shareEventReporter: reporter)

    await sut.shareCompleted(activityType: "com.apple.UIKit.activity.Mail")

    #expect(reporter.reportCalls == [.mail])
  }

  @Test func repository_sendsTheCodeAsAQueryItem() async throws {
    let baseURL = try #require(URL(string: "https://api.example"))
    let transport = StubHTTPTransport()
    let client = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)
    let sut = APIAnonymousApplicationDetailRepository(apiClient: client)

    _ = try? await sut.fetchApplication(bySlug: "kingston", ref: "K/25/1", linkCode: "aZ09bY")
    _ = try? await sut.fetchApplication(bySlug: "kingston", ref: "K/25/1")

    #expect(transport.requests.map(\.url?.query) == ["s=aZ09bY", nil])
  }

  @Test func reporter_postsOnlyTheDestinationWithNoAuth() async throws {
    let baseURL = try #require(URL(string: "https://api.example"))
    let transport = StubHTTPTransport()
    transport.responses = [(Data(), httpResponse(url: baseURL, statusCode: 204))]
    let client = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)

    await APIShareEventReporter(apiClient: client).report(.whatsapp)

    let request = try #require(transport.requests.first)
    #expect(request.httpMethod == "POST")
    #expect(request.url?.path() == "/v1/share-events")
    #expect(request.url?.query == nil)
    #expect(request.value(forHTTPHeaderField: "Authorization") == nil)
    let body = try #require(request.httpBody)
    let json = try #require(JSONSerialization.jsonObject(with: body) as? [String: String])
    #expect(json == ["destination": "whatsapp"])
  }

  @Test func reporter_ignoresFailure() async throws {
    let baseURL = try #require(URL(string: "https://api.example"))
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let client = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)

    await APIShareEventReporter(apiClient: client).report(.mail)

    #expect(transport.requests.count == 1)
  }
}
