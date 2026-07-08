import Combine
import Foundation
import TownCrierDomain

/// ViewModel managing the settings and account screen.
@MainActor
public final class SettingsViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var userEmail: String?
  @Published public private(set) var userName: String?
  @Published public private(set) var authMethod: AuthMethod?
  @Published public private(set) var subscriptionTier: SubscriptionTier = .free
  @Published public private(set) var isTrialPeriod = false
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  @Published public var isShowingDeleteConfirmation = false

  /// True while the data export request is in flight. The View disables the
  /// row and shows a spinner so the user can't trigger a second export.
  @Published public private(set) var isExporting = false

  /// The temp file URL of a completed export, ready to hand to the iOS share
  /// sheet. `nil` until an export succeeds, and reset once the sheet is
  /// dismissed. Setting this is what triggers the View to present the sheet.
  @Published public var exportFileURL: URL?

  /// User-facing message shown when an export fails. `nil` when there is no
  /// export error to display; the View presents an alert while it is non-nil.
  @Published public private(set) var exportErrorMessage: String?

  public var onLogout: (() -> Void)?

  /// Alias kept for source compatibility — the key itself now lives on
  /// ``AppearanceStore``, the single live source of truth shared with the
  /// anonymous welcome screen's appearance control (GH#878).
  static let appearanceModeKey = AppearanceStore.appearanceModeKey

  private let authService: AuthenticationService
  private let subscriptionService: SubscriptionService
  private let userProfileRepository: UserProfileRepository
  private let tierResolver: SubscriptionTierResolving
  private let appVersionProvider: AppVersionProvider
  private let notificationService: NotificationService
  private let appearanceStore: AppearanceStore
  private let exportFileWriter: (Data) throws -> URL

  public init(
    authService: AuthenticationService,
    subscriptionService: SubscriptionService,
    userProfileRepository: UserProfileRepository,
    tierResolver: SubscriptionTierResolving? = nil,
    appVersionProvider: AppVersionProvider,
    notificationService: NotificationService,
    defaults: UserDefaults = .standard,
    appearanceStore: AppearanceStore? = nil,
    exportFileWriter: @escaping (Data) throws -> URL = SettingsViewModel.writeExportToTempFile
  ) {
    self.authService = authService
    self.subscriptionService = subscriptionService
    self.userProfileRepository = userProfileRepository
    let server = ServerTierResolver(userProfileRepository: userProfileRepository)
    self.tierResolver =
      tierResolver
      ?? SubscriptionTierResolver(
        serverFetcher: { await server.ensureServerProfileTier() },
        storeKitFetcher: { await subscriptionService.currentEntitlement() },
        authService: authService
      )
    self.appVersionProvider = appVersionProvider
    self.notificationService = notificationService
    self.appearanceStore = appearanceStore ?? AppearanceStore(defaults: defaults)
    self.exportFileWriter = exportFileWriter
  }

  /// The currently active appearance mode — a live read-through to the
  /// shared ``AppearanceStore`` (GH#878), never a separate copy. Bound by the
  /// Settings picker via `$viewModel.appearanceMode`.
  public var appearanceMode: AppearanceMode {
    get { appearanceStore.appearanceMode }
    set { appearanceStore.appearanceMode = newValue }
  }

  public var appVersion: String {
    "\(appVersionProvider.version) (\(appVersionProvider.buildNumber))"
  }

  public var attributionItems: [AttributionItem] {
    [
      AttributionItem(
        name: "PlanIt",
        detail: "Planning application data",
        url: URL(string: "https://www.planit.org.uk")
      ),
      AttributionItem(
        name: "Crown Copyright",
        detail: "Contains public sector information"
      ),
      AttributionItem(
        name: "Ordnance Survey",
        detail: "Mapping data"
      ),
      AttributionItem(
        name: "Apple Maps",
        detail: "Map rendering and geocoding",
        url: URL(string: "https://www.apple.com/maps/")
      ),
    ]
  }

  public func load() async {
    isLoading = true
    error = nil

    var jwtTier: SubscriptionTier = .free
    var userSub: String?
    if let session = await authService.currentSession() {
      userEmail = session.userProfile.email
      userName = session.userProfile.name
      authMethod = session.userProfile.authMethod
      jwtTier = session.subscriptionTier
      userSub = session.userProfile.userId
    }

    let resolved = await tierResolver.resolve(
      jwtTier: jwtTier,
      previousTier: subscriptionTier,
      userSub: userSub
    )
    subscriptionTier = resolved.tier
    isTrialPeriod = resolved.isTrialPeriod

    isLoading = false
  }

  public func logout() async {
    error = nil
    do {
      try? await notificationService.removeDeviceToken()
      try await authService.logout()
      clearSession()
      onLogout?()
    } catch {
      handleError(error) { .logoutFailed($0) }
    }
  }

  public func requestAccountDeletion() {
    isShowingDeleteConfirmation = true
  }

  public func cancelDeletion() {
    isShowingDeleteConfirmation = false
  }

  public func confirmDeleteAccount() async {
    isShowingDeleteConfirmation = false
    error = nil
    do {
      // UK GDPR Art. 17: server-side erasure must succeed BEFORE we drop the
      // local credentials. If we clear the keychain first and DELETE /v1/me
      // fails, the user's server data is orphaned and they can never retry.
      try await userProfileRepository.delete()
      try? await notificationService.removeDeviceToken()
      try await authService.deleteAccount()
      clearSession()
      onLogout?()
    } catch {
      handleError(error)
    }
  }

  /// Fetches the full GDPR data export (GET /v1/me/data), writes the opaque
  /// server bytes to a temp `.json` file, and publishes the file URL so the
  /// View can present the iOS share sheet. On failure, sets a user-facing
  /// error and produces no artifact.
  public func exportData() async {
    guard !isExporting else { return }
    isExporting = true
    error = nil
    exportErrorMessage = nil
    exportFileURL = nil
    do {
      let bytes = try await userProfileRepository.exportData()
      exportFileURL = try exportFileWriter(bytes)
    } catch {
      handleError(error)
      exportErrorMessage =
        (error as? DomainError)?.userMessage
        ?? "We couldn't export your data. Please try again."
    }
    isExporting = false
  }

  /// Clears the shareable artifact once the share sheet has been dismissed.
  public func dismissExportShare() {
    exportFileURL = nil
  }

  /// Clears the export error once its alert has been dismissed.
  public func dismissExportError() {
    exportErrorMessage = nil
  }

  /// Default writer: persists the export bytes verbatim to a `.json` file in
  /// the temporary directory and returns its URL. Overwrites any prior export
  /// so the temp directory doesn't accumulate stale copies. `nonisolated`
  /// because it touches no actor-isolated state and is used as a default
  /// argument to the `@MainActor` initialiser.
  public nonisolated static func writeExportToTempFile(_ data: Data) throws -> URL {
    let url = FileManager.default.temporaryDirectory
      .appendingPathComponent("towncrier-data-export.json")
    try data.write(to: url, options: .atomic)
    return url
  }

  private func clearSession() {
    userEmail = nil
    userName = nil
    authMethod = nil
    subscriptionTier = .free
    isTrialPeriod = false
  }
}
