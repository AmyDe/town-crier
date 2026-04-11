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

  var onComplete: ((WatchZone) -> Void)?

  private let geocoder: PostcodeGeocoder
  private let watchZoneRepository: WatchZoneRepository
  private let onboardingRepository: OnboardingRepository
  private let notificationService: NotificationService

  public init(
    geocoder: PostcodeGeocoder,
    watchZoneRepository: WatchZoneRepository,
    onboardingRepository: OnboardingRepository,
    notificationService: NotificationService
  ) {
    self.geocoder = geocoder
    self.watchZoneRepository = watchZoneRepository
    self.onboardingRepository = onboardingRepository
    self.notificationService = notificationService
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
