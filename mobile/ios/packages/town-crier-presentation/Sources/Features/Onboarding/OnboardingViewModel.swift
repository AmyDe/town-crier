import Combine
import TownCrierDomain

/// Steps in the first-launch onboarding flow.
public enum OnboardingStep: CaseIterable, Equatable, Sendable {
  case welcome
  case postcodeEntry
  case radiusPicker
  case notificationPermission
}

/// Drives the onboarding flow: welcome → postcode entry → radius picker → notification permission → complete.
@MainActor
public final class OnboardingViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var currentStep: OnboardingStep = .welcome
  @Published public var postcodeInput: String = ""
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  private var validatedPostcode: Postcode?
  @Published public private(set) var geocodedCoordinate: Coordinate?
  @Published public var selectedRadiusMetres: Double = 1000
  private var createdWatchZone: WatchZone?
  @Published public private(set) var isComplete = false

  /// The user's current subscription tier, injected at construction and kept
  /// fresh by ``AppCoordinator`` (it updates this in place when the live tier
  /// resolves, e.g. after an in-wizard purchase). It is `@Published` so the
  /// radius step can unlock the larger paid range reactively *without* the
  /// wizard being rebuilt — a `.id(tier)` rebuild would discard the in-progress
  /// postcode/geocode, which must survive the upgrade round-trip (tc-w3cb.3).
  @Published public internal(set) var subscriptionTier: SubscriptionTier

  /// Drives the in-wizard subscription paywall (tc-w3cb.3). Presented as a sheet
  /// *over* the wizard so the StateObject — and the in-progress postcode/geocode
  /// — survives the purchase round-trip.
  @Published public var isRadiusUpsellPresented = false

  var onComplete: ((WatchZone) -> Void)?

  /// Builds the paywall view-model for the in-wizard upsell sheet. Injected by
  /// ``AppCoordinator`` so the wizard stays decoupled from the composition root.
  /// Optional return so the view degrades gracefully if the factory is unset.
  var makeUpsellViewModel: (() -> SubscriptionViewModel?)?

  /// Invoked when the upsell sheet dismisses, so the coordinator can re-resolve
  /// the subscription tier and unlock the larger radius range live.
  var onUpgradeFlowCompleted: (() async -> Void)?

  private let geocoder: PostcodeGeocoder
  private let watchZoneRepository: WatchZoneRepository
  private let onboardingRepository: OnboardingRepository
  private let notificationService: NotificationService

  public init(
    geocoder: PostcodeGeocoder,
    watchZoneRepository: WatchZoneRepository,
    onboardingRepository: OnboardingRepository,
    notificationService: NotificationService,
    subscriptionTier: SubscriptionTier = .free
  ) {
    self.geocoder = geocoder
    self.watchZoneRepository = watchZoneRepository
    self.onboardingRepository = onboardingRepository
    self.notificationService = notificationService
    self.subscriptionTier = subscriptionTier
  }

  public func advance() {
    switch currentStep {
    case .welcome:
      currentStep = .postcodeEntry
    case .postcodeEntry:
      guard geocodedCoordinate != nil else { return }
      currentStep = .radiusPicker
    case .radiusPicker:
      currentStep = .notificationPermission
    case .notificationPermission:
      break
    }
  }

  public func goBack() {
    switch currentStep {
    case .welcome:
      break
    case .postcodeEntry:
      currentStep = .welcome
    case .radiusPicker:
      currentStep = .postcodeEntry
    case .notificationPermission:
      currentStep = .radiusPicker
    }
  }

  public func submitPostcode() async {
    isLoading = true
    error = nil

    let postcode: Postcode
    do {
      postcode = try Postcode(postcodeInput)
    } catch {
      handleError(error)
      isLoading = false
      return
    }

    do {
      validatedPostcode = postcode
      geocodedCoordinate = try await geocoder.geocode(postcode)
      currentStep = .radiusPicker
    } catch {
      handleError(error)
    }

    isLoading = false
  }

  /// The maximum radius the user's tier permits, in metres. The radius slider
  /// is bounded at this so a free user cannot pick a zone larger than their
  /// tier allows — replacing the old discrete picker that wrongly offered 5 km
  /// to free accounts (cap 2 km). Shared source of truth with the editor via
  /// ``WatchZoneLimits`` (tc-w3cb.2).
  public var maxRadiusMetres: Double {
    WatchZoneLimits(tier: subscriptionTier).maxRadiusMetres
  }

  /// Whether the user's tier still has radius headroom to unlock — true for any
  /// tier below the top (Pro, 10 km). Drives the "Unlock larger zones" chip.
  public var canUnlockLargerRadius: Bool {
    subscriptionTier < .pro
  }

  /// Whether the user's tier delivers instant alerts (push and instant email).
  /// Free accounts receive only the weekly email digest, so the notification
  /// step adapts its copy — and shows a light upgrade nudge — accordingly
  /// (tc-w3cb.4). Same entitlement the editor's instant-alert toggles gate on.
  public var deliversInstantAlerts: Bool {
    EntitlementMap.hasEntitlement(.statusChangeAlerts, for: subscriptionTier)
  }

  /// Surfaces the in-wizard paywall when the user taps the unlock chip.
  public func requestLargerRadiusUpgrade() {
    isRadiusUpsellPresented = true
  }

  /// Called when the paywall sheet dismisses. Re-resolves the tier so a
  /// successful upgrade opens the larger radius range without rebuilding the
  /// wizard (which would discard the in-progress postcode/geocode).
  public func reconcileTierAfterUpgrade() async {
    await onUpgradeFlowCompleted?()
  }

  /// Whether to surface the "this zone may produce lots of notifications" callout
  /// (tc-1zb7). Triggered just above the free tier's 2 km cap so only paid tiers
  /// see it — see `LargeRadiusWarningView` for the threshold rationale.
  public var showsLargeRadiusWarning: Bool {
    selectedRadiusMetres >= LargeRadiusWarning.thresholdMetres
  }

  public func confirmRadius() {
    guard let coordinate = geocodedCoordinate, let postcode = validatedPostcode else { return }
    do {
      let zone = try WatchZone(
        postcode: postcode, centre: coordinate, radiusMetres: selectedRadiusMetres)
      createdWatchZone = zone
      currentStep = .notificationPermission
    } catch {
      self.error = .invalidWatchZoneRadius
    }
  }

  public func requestNotificationPermission() async {
    _ = try? await notificationService.requestPermission()
    await completeOnboarding()
  }

  public func skipNotifications() async {
    await completeOnboarding()
  }

  private func completeOnboarding() async {
    guard let zone = createdWatchZone else { return }
    try? await watchZoneRepository.save(zone)
    onboardingRepository.markOnboardingComplete()
    isComplete = true
    onComplete?(zone)
  }
}
