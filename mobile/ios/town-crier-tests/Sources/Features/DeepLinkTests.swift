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

    sut.handleDeepLink(.applicationDetail(PlanningApplicationId("APP-002")))

    await sut.waitForPendingDetailLoad()

    #expect(sut.detailApplication == .permitted)
    #expect(spy.fetchApplicationCalls == [PlanningApplicationId("APP-002")])
  }

  @Test func handleDeepLink_successClearsPreviousError() async {
    let (sut, spy) = makeSUT()
    sut.deepLinkError = .applicationNotFound(PlanningApplicationId("OLD"))
    spy.fetchApplicationResult = .success(.permitted)

    sut.handleDeepLink(.applicationDetail(PlanningApplicationId("APP-002")))

    await sut.waitForPendingDetailLoad()

    #expect(sut.deepLinkError == nil)
    #expect(sut.detailApplication == .permitted)
  }

  @Test func handleDeepLink_applicationNotFound_setsDeepLinkError() async {
    let (sut, spy) = makeSUT()
    let missingId = PlanningApplicationId("GONE-001")
    spy.fetchApplicationResult = .failure(DomainError.applicationNotFound(missingId))

    sut.handleDeepLink(.applicationDetail(missingId))

    await sut.waitForPendingDetailLoad()

    #expect(sut.detailApplication == nil)
    #expect(sut.deepLinkError == .applicationNotFound(missingId))
  }
}

@Suite("NotificationPayloadParser")
struct NotificationPayloadParserTests {
  // The API contract (api/src/town-crier.infrastructure/Notifications/ApnsAlertPayload.cs)
  // sends `applicationRef` — see docs/specs/apns-push-sender.md. The parser must read
  // the same key. Reading the wrong key returned nil for every push, taking the
  // delegate's early-return path and triggering the actor-hop crash (tc-fcwv).
  @Test func parseDeepLink_withApplicationRef_returnsApplicationDetailLink() {
    let userInfo: [AnyHashable: Any] = ["applicationRef": "APP-001"]

    let result = NotificationPayloadParser.parseDeepLink(from: userInfo)

    #expect(result == .applicationDetail(PlanningApplicationId("APP-001")))
  }

  @Test func parseDeepLink_withLegacyApplicationIdKey_returnsNil() {
    // Guard against accidental reintroduction of the wrong key. The API never
    // sends `applicationId`; only `applicationRef` is in the contract.
    let userInfo: [AnyHashable: Any] = ["applicationId": "APP-001"]

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

  // MARK: - parseCreatedAt (push-tap watermark advance, tc-1nsa.9)

  @Test func parseCreatedAt_withIso8601String_returnsDate() {
    // The API serialises notification.createdAt as ISO-8601 (Z suffix). The
    // parser must decode it without depending on a JSONDecoder strategy
    // because userInfo is a plist-style [AnyHashable: Any], not JSON.
    let userInfo: [AnyHashable: Any] = [
      "createdAt": "2026-05-04T08:11:57Z"
    ]

    let result = NotificationPayloadParser.parseCreatedAt(from: userInfo)

    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    #expect(result == formatter.date(from: "2026-05-04T08:11:57Z"))
  }

  @Test func parseCreatedAt_withFractionalSecondsIso8601_returnsDate() {
    // Some payloads include sub-second precision. The parser must accept
    // both shapes so a server change does not silently break watermark
    // advance.
    let userInfo: [AnyHashable: Any] = [
      "createdAt": "2026-05-04T08:11:57.123Z"
    ]

    let result = NotificationPayloadParser.parseCreatedAt(from: userInfo)

    #expect(result != nil)
  }

  @Test func parseCreatedAt_withMissingKey_returnsNil() {
    // Older API builds (and digest pushes) do not carry createdAt. The
    // parser must defensively return nil rather than crash so the push-tap
    // path continues to deep-link even before the server-side payload is
    // updated.
    let userInfo: [AnyHashable: Any] = ["applicationRef": "APP-1"]

    let result = NotificationPayloadParser.parseCreatedAt(from: userInfo)

    #expect(result == nil)
  }

  @Test func parseCreatedAt_withMalformedString_returnsNil() {
    let userInfo: [AnyHashable: Any] = ["createdAt": "not-a-date"]

    let result = NotificationPayloadParser.parseCreatedAt(from: userInfo)

    #expect(result == nil)
  }

  @Test func parseCreatedAt_withNonStringValue_returnsNil() {
    // Defensive: APNs userInfo is loosely typed. A numeric value (e.g.
    // accidental Unix timestamp) must not be coerced into a date.
    let userInfo: [AnyHashable: Any] = ["createdAt": 1_712_000_000]

    let result = NotificationPayloadParser.parseCreatedAt(from: userInfo)

    #expect(result == nil)
  }
}
