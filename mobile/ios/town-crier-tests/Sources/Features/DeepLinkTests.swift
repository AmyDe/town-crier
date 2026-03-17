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
            subscriptionService: SpySubscriptionService()
        )
        return (coordinator, spy)
    }

    @Test func handleDeepLink_applicationDetail_fetchesAndSetsDetailApplication() async throws {
        let (sut, spy) = makeSUT()
        spy.fetchApplicationResult = .success(.approved)

        sut.handleDeepLink(.applicationDetail(PlanningApplicationId("APP-002")))

        try await Task.sleep(for: .milliseconds(50))

        #expect(sut.detailApplication == .approved)
        #expect(spy.fetchApplicationCalls == [PlanningApplicationId("APP-002")])
    }

    @Test func handleDeepLink_successClearsPreviousError() async throws {
        let (sut, spy) = makeSUT()
        sut.deepLinkError = .applicationNotFound(PlanningApplicationId("OLD"))
        spy.fetchApplicationResult = .success(.approved)

        sut.handleDeepLink(.applicationDetail(PlanningApplicationId("APP-002")))

        try await Task.sleep(for: .milliseconds(50))

        #expect(sut.deepLinkError == nil)
        #expect(sut.detailApplication == .approved)
    }

    @Test func handleDeepLink_applicationNotFound_setsDeepLinkError() async throws {
        let (sut, spy) = makeSUT()
        let missingId = PlanningApplicationId("GONE-001")
        spy.fetchApplicationResult = .failure(DomainError.applicationNotFound(missingId))

        sut.handleDeepLink(.applicationDetail(missingId))

        try await Task.sleep(for: .milliseconds(50))

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
}
