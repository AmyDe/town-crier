import Foundation
import Testing
import TownCrierDomain

#if canImport(UIKit)
  import UIKit
#else
  import AppKit
#endif

@testable import TownCrierPresentation

@Suite("WebhookView layout")
@MainActor
struct WebhookViewLayoutTests {

  // MARK: - Helpers

  private let repository = SpyWebhookRepository()

  private func makeConfiguredSUT() async -> WebhookViewModel {
    repository.fetchResult = .success(.fixture())
    let sut = WebhookViewModel(
      repository: repository, clipboard: SpyClipboardWriter(), tier: .pro
    ) {}
    await sut.load()
    return sut
  }

  // MARK: - Secret wrapping

  @Test("the secret wraps between characters and is never hyphenated")
  func secretParagraphStyle_neverHyphenates() {
    let style = SecretTextLayout.paragraphStyle

    #expect(style.lineBreakMode == .byCharWrapping)
    #expect(style.hyphenationFactor == 0)
    #expect(style.lineBreakStrategy.isEmpty)
  }

  // MARK: - Feedback visibility

  @Test("there is nothing to scroll to without feedback")
  func feedbackAnchor_noFeedback_isNil() async {
    let sut = await makeConfiguredSUT()

    #expect(sut.feedbackAnchor == nil)
  }

  @Test("a test result is the feedback to scroll to")
  func feedbackAnchor_testResult() async {
    let sut = await makeConfiguredSUT()

    await sut.sendTest()

    #expect(sut.feedbackAnchor == .testResult)
  }

  @Test(
    "an error is the feedback to scroll to",
    arguments: [DomainError.webhookURLInvalid, .networkUnavailable])
  func feedbackAnchor_error(failure: DomainError) async {
    let sut = await makeConfiguredSUT()
    repository.saveResult = .failure(failure)

    await sut.save()

    #expect(sut.feedbackAnchor == .error)
  }
}
