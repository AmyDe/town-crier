import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("WebhookViewModel")
@MainActor
struct WebhookViewModelTests {

  // MARK: - Helpers

  private let repository = SpyWebhookRepository()
  private let clipboard = SpyClipboardWriter()

  private func makeSUT(
    tier: SubscriptionTier = .pro,
    onViewPlans: @escaping () -> Void = {}
  ) -> WebhookViewModel {
    WebhookViewModel(
      repository: repository,
      clipboard: clipboard,
      tier: tier,
      onViewPlans: onViewPlans
    )
  }

  private func makeConfiguredSUT(
    _ webhook: Webhook = .fixture(), tier: SubscriptionTier = .pro
  ) async -> WebhookViewModel {
    repository.fetchResult = .success(webhook)
    let sut = makeSUT(tier: tier)
    await sut.load()
    return sut
  }

  private func makeSUTShowingSecret(tier: SubscriptionTier = .pro) async -> WebhookViewModel {
    let sut = await makeConfiguredSUT(tier: tier)
    repository.rotateSecretResult = .success("whsec_shown")
    await sut.rotateSecret()
    return sut
  }

  // MARK: - load

  @Test("starts in the loading state")
  func init_isLoading() {
    #expect(makeSUT().state == .loading)
  }

  @Test("load shows the upsell to a non-Pro user with no webhook", arguments: [
    SubscriptionTier.free, .personal,
  ])
  func load_nonProNoWebhook_isUpsell(tier: SubscriptionTier) async {
    let sut = makeSUT(tier: tier)

    await sut.load()

    #expect(sut.state == .upsell)
    #expect(repository.fetchCallCount == 1)
  }

  @Test("load shows the empty form to a Pro user with no webhook")
  func load_proNoWebhook_isEmpty() async {
    let sut = makeSUT(tier: .pro)

    await sut.load()

    #expect(sut.state == .empty)
    #expect(sut.urlText.isEmpty)
    #expect(sut.format == .json)
  }

  @Test("load shows the webhook and fills the form", arguments: [
    SubscriptionTier.free, .pro,
  ])
  func load_webhook_isConfigured(tier: SubscriptionTier) async {
    let webhook = Webhook.fixture(format: .slack, paused: tier != .pro)

    let sut = await makeConfiguredSUT(webhook, tier: tier)

    #expect(sut.state == .configured(webhook))
    #expect(sut.urlText == webhook.url)
    #expect(sut.format == .slack)
  }

  @Test("a webhook with an unknown format selects no format and cannot be saved")
  func load_unknownFormat_disablesSave() async {
    let sut = await makeConfiguredSUT(.fixture(format: nil))

    await sut.save()

    #expect(sut.format == nil)
    #expect(!sut.canSave)
    #expect(repository.saveCalls.isEmpty)
  }

  @Test("picking a format enables Save and sends that format")
  func save_afterPickingFormat_sendsIt() async {
    let sut = await makeConfiguredSUT(.fixture(format: nil))
    sut.format = .discord

    #expect(sut.canSave)
    await sut.save()

    #expect(repository.saveCalls.map(\.format) == [.discord])
  }

  @Test("removing a webhook with an unknown format resets the form to JSON")
  func confirmRemove_unknownFormat_resetsToJSON() async {
    let sut = await makeConfiguredSUT(.fixture(format: nil))

    await sut.confirmRemove()

    #expect(sut.format == .json)
  }

  @Test("load shows loadFailed on any other error")
  func load_failure_isLoadFailed() async {
    repository.fetchResult = .failure(DomainError.serverError(statusCode: 500, message: nil))
    let sut = makeSUT()

    await sut.load()

    #expect(sut.state == .loadFailed)
    #expect(sut.error == nil)
    #expect(sut.inlineError == nil)
  }

  @Test("load records session and network errors on the shared error property", arguments: [
    DomainError.sessionExpired, .networkUnavailable,
  ])
  func load_sharedError_isRecorded(failure: DomainError) async {
    repository.fetchResult = .failure(failure)
    let sut = makeSUT()

    await sut.load()

    #expect(sut.state == .loadFailed)
    #expect(sut.error == failure)
  }

  @Test("load stays loading when cancelled")
  func load_cancelled_staysLoading() async {
    repository.fetchResult = .failure(CancellationError())
    let sut = makeSUT()

    await sut.load()

    #expect(sut.state == .loading)
  }

  @Test("load recovers after a failure")
  func load_retry_recovers() async {
    repository.fetchResult = .failure(DomainError.networkUnavailable)
    let sut = makeSUT()
    await sut.load()
    repository.fetchResult = .success(.fixture())

    await sut.load()

    #expect(sut.state == .configured(.fixture()))
    #expect(sut.error == nil)
  }

  // MARK: - save

  @Test("canSave is false while the trimmed URL is empty", arguments: ["", "   ", "\n "])
  func canSave_blankURL_isFalse(text: String) async {
    let sut = makeSUT()
    await sut.load()
    sut.urlText = text

    await sut.save()

    #expect(!sut.canSave)
    #expect(repository.saveCalls.isEmpty)
  }

  @Test("save sends the trimmed URL and chosen format")
  func save_sendsTrimmedURL() async {
    let sut = makeSUT()
    await sut.load()
    sut.urlText = "  https://example.com/hook \n"
    sut.format = .slack

    await sut.save()

    #expect(repository.saveCalls.map(\.url) == ["https://example.com/hook"])
    #expect(repository.saveCalls.map(\.format) == [.slack])
  }

  @Test("save sends a chosen Discord format")
  func save_discord() async {
    let sut = makeSUT()
    await sut.load()
    sut.urlText = "https://example.com/hook"
    sut.format = .discord

    await sut.save()

    #expect(repository.saveCalls.map(\.format) == [.discord])
  }

  @Test("save on create reveals the secret and shows the webhook")
  func save_create_revealsSecret() async {
    let saved = Webhook.fixture()
    repository.saveResult = .success(WebhookSaveResult(webhook: saved, secret: "whsec_abc"))
    let sut = makeSUT()
    await sut.load()
    sut.urlText = saved.url

    await sut.save()

    #expect(sut.state == .configured(saved))
    #expect(sut.revealedSecret == "whsec_abc")
    #expect(!sut.isBusy)
  }

  @Test("save on replace clears the revealed secret")
  func save_replace_clearsSecret() async {
    let sut = await makeSUTShowingSecret()
    let replaced = Webhook.fixture(format: .slack)
    repository.saveResult = .success(WebhookSaveResult(webhook: replaced, secret: nil))

    await sut.save()

    #expect(sut.revealedSecret == nil)
    #expect(sut.state == .configured(replaced))
  }

  @Test("a second save while busy is ignored")
  func save_whileBusy_isIgnored() async {
    let sut = await makeConfiguredSUT()
    repository.whileSaving = { [repository] in
      await MainActor.run { #expect(sut.isBusy) }
      repository.whileSaving = nil
      await sut.save()
    }

    await sut.save()

    #expect(repository.saveCalls.count == 1)
    #expect(!sut.isBusy)
  }

  // MARK: - rotateSecret

  @Test("rotateSecret reveals the new secret")
  func rotateSecret_revealsNewSecret() async {
    let sut = await makeSUTShowingSecret()
    repository.rotateSecretResult = .success("whsec_second")

    await sut.rotateSecret()

    #expect(sut.revealedSecret == "whsec_second")
  }

  // MARK: - remove

  @Test("requestRemove asks first and removes nothing")
  func requestRemove_presentsAlert() async {
    let sut = await makeConfiguredSUT()

    sut.requestRemove()

    #expect(sut.isRemoveAlertPresented)
    #expect(repository.removeCallCount == 0)
  }

  @Test(
    "confirmRemove clears the secret and returns to the state for the tier",
    arguments: [
      (SubscriptionTier.pro, WebhookViewModel.State.empty),
      (.personal, .upsell),
      (.free, .upsell),
    ])
  func confirmRemove_returnsToTierState(
    tier: SubscriptionTier, expected: WebhookViewModel.State
  ) async {
    let sut = await makeSUTShowingSecret(tier: tier)
    sut.requestRemove()

    await sut.confirmRemove()

    #expect(repository.removeCallCount == 1)
    #expect(sut.state == expected)
    #expect(sut.revealedSecret == nil)
    #expect(sut.urlText.isEmpty)
    #expect(sut.format == .json)
    #expect(!sut.isRemoveAlertPresented)
  }

  // MARK: - copy, plans, dismissal

  @Test("copySecret writes the secret to the clipboard")
  func copySecret_writesSecret() async {
    let sut = await makeSUTShowingSecret()

    sut.copySecret()

    #expect(clipboard.copyCalls == ["whsec_shown"])
  }

  @Test("copySecret does nothing without a secret")
  func copySecret_noSecret_doesNothing() async {
    let sut = await makeConfiguredSUT()

    sut.copySecret()

    #expect(clipboard.copyCalls.isEmpty)
  }

  @Test("viewPlans calls the callback")
  func viewPlans_callsCallback() {
    var calls = 0
    let sut = makeSUT { calls += 1 }

    sut.viewPlans()

    #expect(calls == 1)
  }

  @Test("reset forgets the secret and the test result")
  func reset_clearsSecret() async {
    let sut = await makeSUTShowingSecret()

    sut.reset()

    #expect(sut.revealedSecret == nil)
    #expect(sut.testResult == nil)
  }

  @Test("the view model holds no persistence dependency")
  func dependencies_includeNoPersistence() async {
    let sut = await makeSUTShowingSecret()
    let banned = ["UserDefaults", "ModelContext", "ModelContainer", "Keychain", "Store", "Cache"]

    let storedTypes = Mirror(reflecting: sut).children.map { child in
      String(describing: type(of: child.value))
    }

    #expect(!storedTypes.isEmpty)
    for storedType in storedTypes where !storedType.hasPrefix("Published<") {
      #expect(!banned.contains { storedType.contains($0) }, "\(storedType)")
    }
  }
}
