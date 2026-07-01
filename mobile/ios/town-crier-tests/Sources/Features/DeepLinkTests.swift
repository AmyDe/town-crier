import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("Deep Link Handling")
@MainActor
struct DeepLinkTests {
  private func makeSUT() -> (AppCoordinator, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    let coordinator = AppCoordinator(
      repository: spy,
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )
    return (coordinator, spy)
  }

  @Test func handleDeepLink_applicationDetail_fetchesAndSetsDetailApplication() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationResult = .success(.permitted)

    sut.handleDeepLink(.applicationDetail(PlanningApplicationId(authority: "42", name: "APP-002")))

    await sut.waitForPendingDetailLoad()

    #expect(sut.detailApplication == .permitted)
    #expect(spy.fetchApplicationCalls == [PlanningApplicationId(authority: "42", name: "APP-002")])
  }

  @Test func handleDeepLink_successClearsPreviousError() async {
    let (sut, spy) = makeSUT()
    sut.deepLinkError = .applicationNotFound(PlanningApplicationId(authority: "42", name: "OLD"))
    spy.fetchApplicationResult = .success(.permitted)

    sut.handleDeepLink(.applicationDetail(PlanningApplicationId(authority: "42", name: "APP-002")))

    await sut.waitForPendingDetailLoad()

    #expect(sut.deepLinkError == nil)
    #expect(sut.detailApplication == .permitted)
  }

  @Test func handleDeepLink_applicationNotFound_setsDeepLinkError() async {
    let (sut, spy) = makeSUT()
    let missingId = PlanningApplicationId(authority: "42", name: "GONE-001")
    spy.fetchApplicationResult = .failure(DomainError.applicationNotFound(missingId))

    sut.handleDeepLink(.applicationDetail(missingId))

    await sut.waitForPendingDetailLoad()

    #expect(sut.detailApplication == nil)
    #expect(sut.deepLinkError == .applicationNotFound(missingId))
  }

  // Regression for tc-dt3x: tapping a digest email card opened the app on
  // whichever tab was previously active and the detail sheet never presented
  // because the sheet modifier lived only on the Applications tab's
  // NavigationStack. The deep-link handler must now switch to the
  // Applications tab so the sheet binding is in scope when SwiftUI evaluates
  // the modifier hierarchy.
  @Test func handleDeepLink_applicationDetail_setsSelectedTabToApplications() async {
    let (sut, spy) = makeSUT()
    sut.selectedTab = .saved
    spy.fetchApplicationResult = .success(.permitted)

    sut.handleDeepLink(.applicationDetail(PlanningApplicationId(authority: "42", name: "APP-002")))

    await sut.waitForPendingDetailLoad()

    #expect(sut.selectedTab == .applications)
  }

  // Regression for tc-dt3x: rapid 4× taps on a digest email card spawned
  // four overlapping `pendingDetailLoad` tasks. The last to complete wins
  // its mutation of `detailApplication`, but the earlier tasks could
  // re-publish the property after a stale fetch resolved, causing the
  // sheet to flicker or fail to present at all. The fix cancels any prior
  // task before kicking off a new one and bails out post-`await` when the
  // task has been cancelled.
  @Test func showApplicationDetail_cancelsPriorPendingDetailLoad() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationResult = .success(.permitted)

    sut.handleDeepLink(.applicationDetail(PlanningApplicationId(authority: "42", name: "APP-001")))
    let firstTask = sut.pendingDetailLoad
    sut.handleDeepLink(.applicationDetail(PlanningApplicationId(authority: "42", name: "APP-002")))

    await sut.waitForPendingDetailLoad()

    #expect(firstTask?.isCancelled == true)
  }

  // MARK: - Share Universal Link (GH #738 Slice 4)

  @Test func handleDeepLink_shareApplication_fetchesBySlugAndSetsDetailApplication() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationBySlugResult = .success(.permitted)

    sut.handleDeepLink(.shareApplication(authoritySlug: "kingston", ref: "Kingston/25/02755/CLC"))

    await sut.waitForPendingDetailLoad()

    #expect(sut.detailApplication == .permitted)
    #expect(sut.selectedTab == .applications)
    #expect(
      spy.fetchApplicationBySlugCalls == [
        .init(authoritySlug: "kingston", ref: "Kingston/25/02755/CLC")
      ])
  }

  @Test func handleDeepLink_shareApplicationFailure_setsDeepLinkError() async {
    let (sut, spy) = makeSUT()
    let missingId = PlanningApplicationId(authority: "kingston", name: "Kingston/25/GONE")
    spy.fetchApplicationBySlugResult = .failure(DomainError.applicationNotFound(missingId))

    sut.handleDeepLink(.shareApplication(authoritySlug: "kingston", ref: "Kingston/25/GONE"))

    await sut.waitForPendingDetailLoad()

    #expect(sut.detailApplication == nil)
    #expect(sut.deepLinkError == .applicationNotFound(missingId))
  }
}

@Suite("NotificationPayloadParser")
struct NotificationPayloadParserTests {
  // The APNs payload now carries both `applicationRef` (the PlanIt case ref) and
  // `authorityId` (the area integer ID) — both are required to construct a
  // `PlanningApplicationId` struct (tc-dzwo.1). The parser must read both keys.
  @Test func parseDeepLink_withApplicationRefAndAuthorityId_returnsApplicationDetailLink() {
    let userInfo: [AnyHashable: Any] = [
      "applicationRef": "22/1234/FUL",
      "authorityId": 42,
    ]

    let result = NotificationPayloadParser.parseDeepLink(from: userInfo)

    #expect(
      result == .applicationDetail(PlanningApplicationId(authority: "42", name: "22/1234/FUL")))
  }

  @Test func parseDeepLink_missingAuthorityId_returnsNil() {
    // `authorityId` is required — without it we cannot do a partitioned Cosmos
    // point read, so the deep link must be dropped rather than fall back to the
    // cross-partition scan that triggers "Server Error".
    let userInfo: [AnyHashable: Any] = ["applicationRef": "22/1234/FUL"]

    let result = NotificationPayloadParser.parseDeepLink(from: userInfo)

    #expect(result == nil)
  }

  @Test func parseDeepLink_missingApplicationRef_returnsNil() {
    let userInfo: [AnyHashable: Any] = ["authorityId": 42]

    let result = NotificationPayloadParser.parseDeepLink(from: userInfo)

    #expect(result == nil)
  }

  @Test func parseDeepLink_authorityIdAsString_isIgnored() {
    // APNs encodes authorityId as a JSON number (Int). A string value must not
    // be coerced — if the server accidentally sends "42" as a string the parser
    // treats the payload as malformed and returns nil.
    let userInfo: [AnyHashable: Any] = [
      "applicationRef": "22/1234/FUL",
      "authorityId": "42",
    ]

    let result = NotificationPayloadParser.parseDeepLink(from: userInfo)

    #expect(result == nil)
  }

  @Test func parseDeepLink_withLegacyApplicationIdKey_returnsNil() {
    // Guard against accidental reintroduction of the wrong key. The API never
    // sends `applicationId`; only `applicationRef` is in the contract.
    let userInfo: [AnyHashable: Any] = ["applicationId": "22/1234/FUL", "authorityId": 42]

    let result = NotificationPayloadParser.parseDeepLink(from: userInfo)

    #expect(result == nil)
  }

  @Test func parseDeepLink_withoutApplicationRef_returnsNil() {
    let userInfo: [AnyHashable: Any] = ["unrelated": "data"]

    let result = NotificationPayloadParser.parseDeepLink(from: userInfo)

    #expect(result == nil)
  }

  @Test func parseDeepLink_withEmptyPayload_returnsNil() {
    // Digest pushes contain no applicationRef — this is the no-deep-link
    // early-return path that previously crashed when the delegate failed to
    // hop back to MainActor (tc-fcwv). Production code now wraps the
    // delegate body in `await MainActor.run { ... }` so this nil path is safe.
    let userInfo: [AnyHashable: Any] = [:]

    let result = NotificationPayloadParser.parseDeepLink(from: userInfo)

    #expect(result == nil)
  }

  @Test func parseDeepLink_withDigestPayloadShape_returnsNil() {
    // Real digest payload shape per docs/specs/apns-push-sender.md — no
    // applicationRef, just aps + badge. Must return nil, not crash.
    let userInfo: [AnyHashable: Any] = [
      "aps": [
        "alert": ["title": "Town Crier", "body": "5 new applications this week"],
        "sound": "default",
        "badge": 5,
      ]
    ]

    let result = NotificationPayloadParser.parseDeepLink(from: userInfo)

    #expect(result == nil)
  }

  // MARK: - renderBody (DecisionUpdate)

  @Test func renderBody_forDecisionUpdateWithPermitted_returnsApprovedSentence() {
    let userInfo: [AnyHashable: Any] = [
      "eventType": "DecisionUpdate",
      "applicationName": "12345",
      "decision": "Permitted",
    ]

    let result = NotificationPayloadParser.renderBody(from: userInfo)

    #expect(result == "Application 12345 was Approved")
  }

  @Test func renderBody_forDecisionUpdateWithConditions_returnsApprovedWithConditionsSentence() {
    let userInfo: [AnyHashable: Any] = [
      "eventType": "DecisionUpdate",
      "applicationName": "2026/0099",
      "decision": "Conditions",
    ]

    let result = NotificationPayloadParser.renderBody(from: userInfo)

    #expect(result == "Application 2026/0099 was Approved with conditions")
  }

  @Test func renderBody_forDecisionUpdateWithRejected_returnsRefusedSentence() {
    let userInfo: [AnyHashable: Any] = [
      "eventType": "DecisionUpdate",
      "applicationName": "APP-1",
      "decision": "Rejected",
    ]

    let result = NotificationPayloadParser.renderBody(from: userInfo)

    #expect(result == "Application APP-1 was Refused")
  }

  @Test func renderBody_forDecisionUpdateWithAppealed_returnsRefusalAppealedSentence() {
    let userInfo: [AnyHashable: Any] = [
      "eventType": "DecisionUpdate",
      "applicationName": "APP-1",
      "decision": "Appealed",
    ]

    let result = NotificationPayloadParser.renderBody(from: userInfo)

    #expect(result == "Application APP-1 was Refusal appealed")
  }

  @Test func renderBody_forDecisionUpdateWithUnrecognisedDecision_returnsNil() {
    let userInfo: [AnyHashable: Any] = [
      "eventType": "DecisionUpdate",
      "applicationName": "APP-1",
      "decision": "Withdrawn",
    ]

    let result = NotificationPayloadParser.renderBody(from: userInfo)

    #expect(result == nil)
  }

  @Test func renderBody_forDecisionUpdateMissingDecision_returnsNil() {
    let userInfo: [AnyHashable: Any] = [
      "eventType": "DecisionUpdate",
      "applicationName": "APP-1",
    ]

    let result = NotificationPayloadParser.renderBody(from: userInfo)

    #expect(result == nil)
  }

  @Test func renderBody_forDecisionUpdateMissingApplicationName_returnsNil() {
    let userInfo: [AnyHashable: Any] = [
      "eventType": "DecisionUpdate",
      "decision": "Permitted",
    ]

    let result = NotificationPayloadParser.renderBody(from: userInfo)

    #expect(result == nil)
  }

  @Test func renderBody_forNonDecisionEventType_returnsNil() {
    let userInfo: [AnyHashable: Any] = [
      "eventType": "NewApplication",
      "applicationName": "APP-1",
      "decision": "Permitted",
    ]

    let result = NotificationPayloadParser.renderBody(from: userInfo)

    #expect(result == nil)
  }

  @Test func renderBody_forEmptyPayload_returnsNil() {
    let userInfo: [AnyHashable: Any] = [:]

    let result = NotificationPayloadParser.renderBody(from: userInfo)

    #expect(result == nil)
  }

}
