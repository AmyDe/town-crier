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

  @Test func handleDeepLink_applicationDetail_fetchesAndSetsDetailApplication() async throws {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationResult = .success(.permitted)

    sut.handleDeepLink(.applicationDetail(PlanningApplicationId("APP-002")))

    try await Task.sleep(for: .milliseconds(200))

    #expect(sut.detailApplication == .permitted)
    #expect(spy.fetchApplicationCalls == [PlanningApplicationId("APP-002")])
  }

  @Test func handleDeepLink_successClearsPreviousError() async throws {
    let (sut, spy) = makeSUT()
    sut.deepLinkError = .applicationNotFound(PlanningApplicationId("OLD"))
    spy.fetchApplicationResult = .success(.permitted)

    sut.handleDeepLink(.applicationDetail(PlanningApplicationId("APP-002")))

    try await Task.sleep(for: .milliseconds(200))

    #expect(sut.deepLinkError == nil)
    #expect(sut.detailApplication == .permitted)
  }

  @Test func handleDeepLink_applicationNotFound_setsDeepLinkError() async throws {
    let (sut, spy) = makeSUT()
    let missingId = PlanningApplicationId("GONE-001")
    spy.fetchApplicationResult = .failure(DomainError.applicationNotFound(missingId))

    sut.handleDeepLink(.applicationDetail(missingId))

    try await Task.sleep(for: .milliseconds(200))

    #expect(sut.detailApplication == nil)
    #expect(sut.deepLinkError == .applicationNotFound(missingId))
  }
}

@Suite("NotificationPayloadParser")
struct NotificationPayloadParserTests {
  @Test func parseDeepLink_withApplicationId_returnsApplicationDetailLink() {
    let userInfo: [AnyHashable: Any] = ["applicationId": "APP-001"]

    let result = NotificationPayloadParser.parseDeepLink(from: userInfo)

    #expect(result == .applicationDetail(PlanningApplicationId("APP-001")))
  }

  @Test func parseDeepLink_withoutApplicationId_returnsNil() {
    let userInfo: [AnyHashable: Any] = ["unrelated": "data"]

    let result = NotificationPayloadParser.parseDeepLink(from: userInfo)

    #expect(result == nil)
  }

  @Test func parseDeepLink_withEmptyPayload_returnsNil() {
    let userInfo: [AnyHashable: Any] = [:]

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
