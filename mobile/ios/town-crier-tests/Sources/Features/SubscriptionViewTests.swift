import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SubscriptionView")
@MainActor
struct SubscriptionViewTests {
  private func makeViewModel() -> SubscriptionViewModel {
    SubscriptionViewModel(
      subscriptionService: SpySubscriptionService(),
      authenticationService: SpyAuthenticationService()
    )
  }

  @Test func body_rendersWithoutCrashing() {
    let sut = SubscriptionView(viewModel: makeViewModel())
    _ = sut.body
  }

  @Test func body_rendersWithAllFourProductsLoaded() async {
    let (viewModel, spy) = makeLoadableViewModel()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    await viewModel.loadProducts()

    let sut = SubscriptionView(viewModel: viewModel)
    _ = sut.body

    #expect(viewModel.selectedProProduct == .proAnnual)
  }

  @Test func body_rendersWithLifetimeEntitlement() async {
    let (viewModel, spy) = makeLoadableViewModel()
    spy.availableProductsResult = .success([.personal, .pro, .proAnnual, .proLifetime])
    spy.currentEntitlementResult = .proLifetime
    await viewModel.loadProducts()

    let sut = SubscriptionView(viewModel: viewModel)
    _ = sut.body

    #expect(!viewModel.showsPurchaseButtons)
  }

  @Test func body_rendersWhileCancelPromptIsPresented() async {
    let (viewModel, spy) = makeLoadableViewModel()
    spy.purchaseResult = .success(.proLifetime)
    spy.hasActiveAutoRenewingSubscriptionResult = true
    await viewModel.purchase(productId: "uk.towncrierapp.pro.lifetime")

    let sut = SubscriptionView(viewModel: viewModel)
    _ = sut.body

    #expect(viewModel.isCancelSubscriptionPromptPresented)
  }

  private func makeLoadableViewModel() -> (SubscriptionViewModel, SpySubscriptionService) {
    let spy = SpySubscriptionService()
    let viewModel = SubscriptionViewModel(
      subscriptionService: spy,
      authenticationService: SpyAuthenticationService()
    )
    return (viewModel, spy)
  }
}
