import Foundation
import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("WebhookView visible elements")
@MainActor
struct WebhookViewTests {

  // MARK: - Helpers

  private func makeViewModel(
    _ webhook: Webhook?, tier: SubscriptionTier = .pro
  ) async -> WebhookViewModel {
    let repository = SpyWebhookRepository()
    repository.fetchResult = .success(webhook)
    let sut = WebhookViewModel(
      repository: repository,
      clipboard: SpyClipboardWriter(),
      tier: tier,
      timeZone: TimeZone(identifier: "UTC") ?? .current
    ) {}
    await sut.load()
    return sut
  }

  // MARK: - Paused

  @Test("the paused state shows Paused and only Remove")
  func paused_showsOnlyRemove() async {
    let sut = await makeViewModel(
      .fixture(status: .disabled, disabledReason: .gone, paused: true), tier: .free)

    #expect(sut.statusText == "Paused")
    #expect(sut.statusTone == .paused)
    #expect(sut.webhookActions == [.remove])
    #expect(sut.switchedOffReason == nil)
    #expect(!sut.showsEditableForm)
    #expect(!sut.showsDeliveryHistory)
    #expect(sut.showsViewPlans)
  }

  // MARK: - Configured

  @Test("the configured state shows Send test event, Rotate secret and Remove")
  func configured_showsAllActions() async {
    let sut = await makeViewModel(.fixture())

    #expect(sut.webhookActions == [.sendTest, .rotateSecret, .remove])
    #expect(sut.webhookActions.map(\.title) == ["Send test event", "Rotate secret", "Remove"])
    #expect(sut.statusText == "Active")
    #expect(sut.statusTone == .active)
    #expect(sut.switchedOffReason == nil)
    #expect(sut.showsEditableForm)
    #expect(sut.showsDeliveryHistory)
    #expect(!sut.showsViewPlans)
  }

  @Test(
    "a switched-off webhook explains why",
    arguments: [
      (
        WebhookDisabledReason?.some(.failures),
        "We couldn't deliver alerts to your webhook for a day, so we've switched it off. "
          + "To switch it back on, check the address and save it again."
      ),
      (
        .gone,
        "Your webhook told us it no longer exists, so we've switched it off. "
          + "To switch it back on, check the address and save it again."
      ),
      (
        nil,
        "We couldn't deliver alerts to your webhook for a day, so we've switched it off. "
          + "To switch it back on, check the address and save it again."
      ),
    ])
  func switchedOff_explainsWhy(reason: WebhookDisabledReason?, sentence: String) async {
    let sut = await makeViewModel(.fixture(status: .disabled, disabledReason: reason))

    #expect(sut.statusText == "Switched off")
    #expect(sut.statusTone == .switchedOff)
    #expect(sut.switchedOffReason == sentence)
    #expect(sut.webhookActions == [.sendTest, .rotateSecret, .remove])
  }

  @Test("delivery history reads None yet before any delivery")
  func history_empty() async {
    let sut = await makeViewModel(.fixture())

    #expect(sut.lastSuccessText == "None yet")
    #expect(sut.lastFailureText == "None yet")
    #expect(sut.lastFailureReasonText == nil)
  }

  @Test("delivery history formats the dates and explains the last failure")
  func history_dates() async {
    let sut = await makeViewModel(
      .fixture(
        lastSuccessAt: Date(timeIntervalSince1970: 1_795_701_600),
        lastFailureAt: Date(timeIntervalSince1970: 1_795_705_500),
        lastFailureReason: .httpStatus,
        lastFailureStatus: 503
      ))

    #expect(sut.lastSuccessText == "26 Nov 2026, 14:00")
    #expect(sut.lastFailureText == "26 Nov 2026, 15:05")
    #expect(sut.lastFailureReasonText == "It answered with status 503.")
  }

  // MARK: - Without a webhook

  @Test("the empty state has an editable form and no webhook actions")
  func empty_formOnly() async {
    let sut = await makeViewModel(nil)

    #expect(sut.showsEditableForm)
    #expect(sut.webhookActions.isEmpty)
    #expect(!sut.showsViewPlans)
  }

  @Test("the upsell offers View Plans and no form")
  func upsell_offersPlans() async {
    let sut = await makeViewModel(nil, tier: .personal)

    #expect(sut.showsViewPlans)
    #expect(!sut.showsEditableForm)
    #expect(sut.webhookActions.isEmpty)
  }

  @Test("the view builds its body for every state")
  func body_builds() async {
    for webhook in [Webhook?.none, .fixture(), .fixture(paused: true)] {
      let sut = await makeViewModel(webhook)

      _ = WebhookView(viewModel: sut)
    }
  }

  @Test("the docs link points at the public webhook docs")
  func docsLink_url() {
    #expect(WebhookCopy.docsURLString == "https://towncrierapp.uk/developers/webhooks")
  }
}
