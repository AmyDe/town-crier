import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("WebhookViewModel feedback")
@MainActor
struct WebhookViewModelFeedbackTests {

  // MARK: - Helpers

  private let repository = SpyWebhookRepository()

  private func makeConfiguredSUT(tier: SubscriptionTier = .pro) async -> WebhookViewModel {
    repository.fetchResult = .success(.fixture())
    let sut = WebhookViewModel(
      repository: repository, clipboard: SpyClipboardWriter(), tier: tier
    ) {}
    await sut.load()
    return sut
  }

  private func makeSUTShowingSecret(tier: SubscriptionTier = .pro) async -> WebhookViewModel {
    let sut = await makeConfiguredSUT(tier: tier)
    repository.rotateSecretResult = .success("whsec_shown")
    await sut.rotateSecret()
    return sut
  }

  // MARK: - errors

  nonisolated private static let codeErrors: [(DomainError, String)] = [
    (.webhookURLInvalid, "Enter a public https address. Only port 443 works."),
    (.webhookFormatInvalid, "Choose JSON, Slack or Discord."),
    (.webhookRequiresPro, "Webhooks need a Pro subscription."),
    (.webhookTestCooldown, "Wait 10 seconds before you send another test."),
    (.webhookNotFound, "That webhook no longer exists."),
  ]

  @Test("each webhook DomainError sets its inline copy", arguments: codeErrors)
  func webhookError_setsInlineCopy(failure: DomainError, copy: String) async {
    let sut = await makeConfiguredSUT()
    repository.sendTestResult = .failure(failure)

    await sut.sendTest()

    #expect(sut.inlineError == copy)
    #expect(sut.error == nil)
    #expect(!sut.isBusy)
  }

  @Test(
    "webhookNotFound forgets the secret and returns to the state for the tier",
    arguments: [
      (SubscriptionTier.pro, WebhookViewModel.State.empty),
      (.free, .upsell),
    ])
  func notFound_returnsToTierState(
    tier: SubscriptionTier, expected: WebhookViewModel.State
  ) async {
    let sut = await makeSUTShowingSecret(tier: tier)
    repository.rotateSecretResult = .failure(DomainError.webhookNotFound)

    await sut.rotateSecret()

    #expect(sut.state == expected)
    #expect(sut.revealedSecret == nil)
    #expect(sut.urlText.isEmpty)
    #expect(sut.inlineError == "That webhook no longer exists.")
  }

  @Test("other failures name the action that failed")
  func otherFailure_setsActionCopy() async {
    let failure = DomainError.serverError(statusCode: 500, message: nil)
    let sut = await makeConfiguredSUT()
    repository.saveResult = .failure(failure)
    repository.removeResult = .failure(failure)
    repository.sendTestResult = .failure(failure)
    repository.rotateSecretResult = .failure(failure)

    await sut.save()
    #expect(sut.inlineError == "We couldn't save your webhook. Try again in a moment.")
    await sut.confirmRemove()
    #expect(sut.inlineError == "We couldn't remove your webhook. Try again in a moment.")
    await sut.sendTest()
    #expect(sut.inlineError == "We couldn't send the test event. Try again in a moment.")
    await sut.rotateSecret()
    #expect(sut.inlineError == "We couldn't rotate the secret. Try again in a moment.")
    #expect(sut.state == .configured(.fixture()))
  }

  @Test("session and network errors use the shared error property", arguments: [
    DomainError.sessionExpired, .networkUnavailable,
  ])
  func sharedError_isNotInlineCopy(failure: DomainError) async {
    let sut = await makeConfiguredSUT()
    repository.saveResult = .failure(failure)

    await sut.save()

    #expect(sut.error == failure)
    #expect(sut.inlineError == nil)
  }

  @Test("the next action clears the previous error and test result")
  func nextAction_clearsFeedback() async {
    let sut = await makeConfiguredSUT()
    await sut.sendTest()
    repository.saveResult = .failure(DomainError.webhookURLInvalid)
    await sut.save()
    #expect(sut.testResult == nil)

    await sut.rotateSecret()

    #expect(sut.inlineError == nil)
  }

  // MARK: - sendTest

  @Test("sendTest reports a delivered event with its status")
  func sendTest_delivered() async {
    let sut = await makeConfiguredSUT()

    await sut.sendTest()

    #expect(sut.testResultText == "Test event delivered (status 200).")
  }

  @Test("sendTest reports a delivered event without a status")
  func sendTest_deliveredNoStatus() async {
    let sut = await makeConfiguredSUT()
    repository.sendTestResult = .success(
      WebhookTestResult(delivered: true, status: nil, failureReason: nil))

    await sut.sendTest()

    #expect(sut.testResultText == "Test event delivered.")
  }

  @Test(
    "sendTest explains every failure reason",
    arguments: [
      (WebhookFailureReason?.some(.gone), Int?.some(410),
        "It answered 410, which means the address no longer exists."),
      (.httpStatus, 500, "It answered with status 500."),
      (.httpStatus, nil, "It answered with an error status."),
      (.blockedAddress, nil, "That address points somewhere we're not allowed to send to."),
      (.timeout, nil, "It took too long to answer."),
      (.tls, nil, "We couldn't make a secure connection. Check its certificate."),
      (.connection, nil, "We couldn't connect to it."),
    ])
  func sendTest_notDelivered(
    reason: WebhookFailureReason?, status: Int?, sentence: String
  ) async {
    let sut = await makeConfiguredSUT()
    repository.sendTestResult = .success(
      WebhookTestResult(delivered: false, status: status, failureReason: reason))

    await sut.sendTest()

    #expect(sut.testResultText == "Test event not delivered. \(sentence)")
  }

  @Test("sendTest reports an unknown failure reason without a sentence")
  func sendTest_unknownReason() async {
    let sut = await makeConfiguredSUT()
    repository.sendTestResult = .success(
      WebhookTestResult(delivered: false, status: nil, failureReason: nil))

    await sut.sendTest()

    #expect(sut.testResultText == "Test event not delivered.")
  }
}
