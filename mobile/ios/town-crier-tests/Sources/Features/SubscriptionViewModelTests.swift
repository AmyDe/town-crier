import Testing
import TownCrierDomain
@testable import TownCrierPresentation

@Suite("SubscriptionViewModel")
@MainActor
struct SubscriptionViewModelTests {
    private func makeSUT() -> (SubscriptionViewModel, SpySubscriptionService) {
        let spy = SpySubscriptionService()
        let sut = SubscriptionViewModel(subscriptionService: spy)
        return (sut, spy)
    }

    // MARK: - Initial state

    @Test func init_hasNoProducts() {
        let (sut, _) = makeSUT()
        #expect(sut.products.isEmpty)
    }

    @Test func init_isNotLoading() {
        let (sut, _) = makeSUT()
        #expect(!sut.isLoading)
    }

    @Test func init_hasNoError() {
        let (sut, _) = makeSUT()
        #expect(sut.error == nil)
    }

    @Test func init_hasNoEntitlement() {
        let (sut, _) = makeSUT()
        #expect(sut.currentEntitlement == nil)
    }

    @Test func init_isPurchasingIsFalse() {
        let (sut, _) = makeSUT()
        #expect(!sut.isPurchasing)
    }

    @Test func init_isRestoringIsFalse() {
        let (sut, _) = makeSUT()
        #expect(!sut.isRestoring)
    }

    // MARK: - Load products

    @Test func loadProducts_populatesProducts_onSuccess() async {
        let (sut, spy) = makeSUT()
        let expected: [SubscriptionProduct] = [.personal, .pro]
        spy.availableProductsResult = .success(expected)

        await sut.loadProducts()

        #expect(sut.products == expected)
        #expect(!sut.isLoading)
        #expect(sut.error == nil)
    }

    @Test func loadProducts_callsService() async {
        let (sut, spy) = makeSUT()
        spy.availableProductsResult = .success([.personal])

        await sut.loadProducts()

        #expect(spy.availableProductsCallCount == 1)
    }

    @Test func loadProducts_setsError_onFailure() async {
        let (sut, spy) = makeSUT()
        spy.availableProductsResult = .failure(DomainError.networkUnavailable)

        await sut.loadProducts()

        #expect(sut.products.isEmpty)
        #expect(sut.error == .networkUnavailable)
    }

    @Test func loadProducts_alsoLoadsCurrentEntitlement() async {
        let (sut, spy) = makeSUT()
        spy.availableProductsResult = .success([.personal])
        spy.currentEntitlementResult = .personalActive

        await sut.loadProducts()

        #expect(sut.currentEntitlement == .personalActive)
        #expect(spy.currentEntitlementCallCount == 1)
    }

    // MARK: - Purchase

    @Test func purchase_setsEntitlement_onSuccess() async {
        let (sut, spy) = makeSUT()
        spy.purchaseResult = .success(.personalActive)

        await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

        #expect(sut.currentEntitlement == .personalActive)
        #expect(!sut.isPurchasing)
        #expect(sut.error == nil)
    }

    @Test func purchase_callsServiceWithProductId() async {
        let (sut, spy) = makeSUT()
        spy.purchaseResult = .success(.personalActive)

        await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

        #expect(spy.purchaseCalls == ["uk.co.towncrier.personal.monthly"])
    }

    @Test func purchase_setsError_onFailure() async {
        let (sut, spy) = makeSUT()
        spy.purchaseResult = .failure(DomainError.purchaseFailed("payment declined"))

        await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

        #expect(sut.currentEntitlement == nil)
        #expect(sut.error == .purchaseFailed("payment declined"))
    }

    @Test func purchase_setsNilError_onCancellation() async {
        let (sut, spy) = makeSUT()
        spy.purchaseResult = .failure(DomainError.purchaseCancelled)

        await sut.purchase(productId: "uk.co.towncrier.personal.monthly")

        #expect(sut.error == nil)
        #expect(!sut.isPurchasing)
    }

    @Test func purchase_clearsError_beforeAttempt() async {
        let (sut, spy) = makeSUT()
        spy.purchaseResult = .failure(DomainError.purchaseFailed("fail"))
        await sut.purchase(productId: "id")
        #expect(sut.error != nil)

        spy.purchaseResult = .success(.personalActive)
        await sut.purchase(productId: "id")

        #expect(sut.error == nil)
    }

    // MARK: - Restore purchases

    @Test func restorePurchases_setsEntitlement_whenFound() async {
        let (sut, spy) = makeSUT()
        spy.restorePurchasesResult = .success(.proActive)

        await sut.restorePurchases()

        #expect(sut.currentEntitlement == .proActive)
        #expect(!sut.isRestoring)
    }

    @Test func restorePurchases_callsService() async {
        let (sut, spy) = makeSUT()
        spy.restorePurchasesResult = .success(nil)

        await sut.restorePurchases()

        #expect(spy.restorePurchasesCallCount == 1)
    }

    @Test func restorePurchases_leavesEntitlementNil_whenNoneFound() async {
        let (sut, spy) = makeSUT()
        spy.restorePurchasesResult = .success(nil)

        await sut.restorePurchases()

        #expect(sut.currentEntitlement == nil)
        #expect(sut.error == nil)
    }

    @Test func restorePurchases_setsError_onFailure() async {
        let (sut, spy) = makeSUT()
        spy.restorePurchasesResult = .failure(DomainError.restoreFailed("no account"))

        await sut.restorePurchases()

        #expect(sut.error == .restoreFailed("no account"))
    }

    // MARK: - Subscription disclosure

    @Test func subscriptionDisclosure_returnsTermsForProduct() {
        let (sut, _) = makeSUT()

        let disclosure = sut.subscriptionDisclosure(for: .personal)

        #expect(disclosure.contains("£1.99"))
        #expect(disclosure.contains("automatically renew"))
    }

    @Test func subscriptionDisclosure_mentionsTrial_whenAvailable() {
        let (sut, _) = makeSUT()

        let disclosure = sut.subscriptionDisclosure(for: .personal)

        #expect(disclosure.contains("7-day free trial"))
    }

    @Test func subscriptionDisclosure_omitsTrial_whenNotAvailable() {
        let (sut, _) = makeSUT()

        let disclosure = sut.subscriptionDisclosure(for: .pro)

        #expect(!disclosure.contains("free trial"))
    }

    // MARK: - Computed properties

    @Test func isSubscribed_returnsFalse_whenNoEntitlement() {
        let (sut, _) = makeSUT()
        #expect(!sut.isSubscribed)
    }

    @Test func isSubscribed_returnsTrue_whenHasActiveEntitlement() async {
        let (sut, spy) = makeSUT()
        spy.purchaseResult = .success(.personalActive)
        await sut.purchase(productId: "id")

        #expect(sut.isSubscribed)
    }
}
