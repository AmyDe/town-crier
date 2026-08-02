import Combine
import TownCrierDomain

/// Steps in the first-launch onboarding flow.
public enum OnboardingStep: CaseIterable, Equatable, Sendable {
  case welcome
  case postcodeEntry
  case radiusPicker
  /// Custom-shape boundary drawing (GH#1031, tc-6he3x.10). Reached only via
  /// ``OnboardingViewModel/reconcileTierAfterCustomShapeUpgrade()`` after a
  /// successful in-wizard purchase from the radius step's custom-shape
  /// upsell — never part of the default welcome → postcode → radius →
  /// notification sequence.
  case boundaryDrawing
  case notificationPermission
}

/// Drives the onboarding flow: welcome → postcode entry → radius picker → notification permission → complete.
@MainActor
public final class OnboardingViewModel: ObservableObject, ErrorHandlingViewModel, BoundaryDrawingViewModel {
  @Published public private(set) var currentStep: OnboardingStep = .welcome
  @Published public var postcodeInput: String = ""
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  private var validatedPostcode: Postcode?
  /// Set by ``prefill(name:coordinate:radiusMetres:)`` (GH#879 Phase 5): a
  /// device-local zone's name is free text the user chose (or the postcode
  /// it was seeded from), not necessarily a valid ``Postcode`` — so this
  /// carries the name straight through to the zone ``confirmRadius()``
  /// creates rather than round-tripping it through `Postcode` validation.
  private var prefillZoneName: String?
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

  /// Circle (the default) vs. custom polygon shape (GH#1031). Mirrors
  /// `WatchZoneEditorViewModel.shapeMode`: stays `.circle` for the whole
  /// default flow, and is only ever set to `.custom` by
  /// ``reconcileTierAfterCustomShapeUpgrade()`` when a mid-flow purchase
  /// unlocks ``currentStep`` == ``OnboardingStep/boundaryDrawing``. Tracked
  /// so ``goBack()`` can route `.notificationPermission` back to whichever
  /// of the radius or boundary step the zone was actually built from.
  @Published public private(set) var shapeMode: WatchZoneShapeMode = .circle

  /// The custom-shape ring being drawn, as an OPEN sequence of vertices in
  /// drop order — NOT closed. Mirrors
  /// `WatchZoneEditorViewModel.boundaryVertices`; see
  /// ``BoundaryDrawingViewModel`` for the shared surface `BoundaryDrawingMapView`
  /// needs to render it.
  @Published public private(set) var boundaryVertices: [Coordinate] = []

  /// Drives the in-wizard custom-shape upsell paywall (GH#1031, tc-6he3x.10),
  /// presented from the radius step alongside (but distinct from)
  /// ``isRadiusUpsellPresented``. Also a sheet *over* the wizard so the
  /// in-progress postcode/geocode survives the purchase round-trip.
  @Published public var isCustomShapeUpsellPresented = false

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
    case .boundaryDrawing:
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
    case .boundaryDrawing:
      // Stepping back out of drawing returns to the radius step as a plain
      // circle pick — both shapeMode and any in-progress vertices are
      // discarded, mirroring the editor's `selectShapeMode(.circle)`.
      shapeMode = .circle
      boundaryVertices = []
      currentStep = .radiusPicker
    case .notificationPermission:
      // Which step precedes notification depends on which shape the zone
      // was actually built from (tc-6he3x.10) — `shapeMode` is the source of
      // truth for that, since it's only ever `.custom` after a genuine
      // mid-flow tier change routed here via ``boundaryDrawing``.
      currentStep = shapeMode == .custom ? .boundaryDrawing : .radiusPicker
    }
  }

  /// Seeds the wizard's postcode/coordinate/radius from an already-resolved
  /// anonymous browse session and jumps straight to the radius step,
  /// skipping postcode entry a second time. Used for the anonymous browse
  /// post-signup handoff (GH#868 Phase 3.5, radius carried through in the
  /// Phase 3 refinement): a user who located themselves — and picked a live
  /// monitoring radius — before creating an account shouldn't be asked for
  /// either again. `radiusMetres` is clamped to `[100, maxRadiusMetres]` so a
  /// stale or legacy persisted value can never land outside the radius
  /// step's own slider bounds. Additive — the normal (non-prefilled) flow
  /// through `.postcodeEntry` -> ``submitPostcode()`` is unchanged for every
  /// other caller.
  public func prefill(postcode: Postcode, coordinate: Coordinate, radiusMetres: Double) {
    postcodeInput = postcode.value
    validatedPostcode = postcode
    geocodedCoordinate = coordinate
    selectedRadiusMetres = min(max(radiusMetres, 100), maxRadiusMetres)
    currentStep = .radiusPicker
  }

  /// Seeds the wizard's zone name/coordinate/radius from an already-created
  /// device-local zone (GH#879 Phase 5) and jumps straight to the radius
  /// step, mirroring ``prefill(postcode:coordinate:radiusMetres:)`` but for a
  /// zone whose name is arbitrary user text rather than a validated
  /// postcode. `radiusMetres` is clamped the same way.
  public func prefill(name: String, coordinate: Coordinate, radiusMetres: Double) {
    prefillZoneName = name
    postcodeInput = name
    validatedPostcode = nil
    geocodedCoordinate = coordinate
    selectedRadiusMetres = min(max(radiusMetres, 100), maxRadiusMetres)
    currentStep = .radiusPicker
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
    guard let coordinate = geocodedCoordinate else { return }
    do {
      let zone: WatchZone
      if let prefillZoneName {
        zone = try WatchZone(
          name: prefillZoneName, centre: coordinate, radiusMetres: selectedRadiusMetres)
      } else if let postcode = validatedPostcode {
        zone = try WatchZone(
          postcode: postcode, centre: coordinate, radiusMetres: selectedRadiusMetres)
      } else {
        return
      }
      createdWatchZone = zone
      currentStep = .notificationPermission
    } catch {
      self.error = .invalidWatchZoneRadius
    }
  }

  // MARK: - Custom-shape boundary drawing (GH#1031, tc-6he3x.10)

  /// Mirrors `WatchZoneBoundary`'s geometric minimum (3 distinct vertices
  /// are required to form a polygon).
  private static let minimumBoundaryVertexCount = 3

  /// Whether the user's tier may draw a custom-shape zone at all — gates
  /// showing the custom-shape upsell on the radius step. Mirrors
  /// `WatchZoneEditorViewModel.canDrawCustomShape` via the same
  /// ``WatchZoneLimits`` source of truth.
  public var canDrawCustomShape: Bool {
    WatchZoneLimits(tier: subscriptionTier).allowsCustomBoundary
  }

  /// Whether ``boundaryVertices`` currently has enough points to form a
  /// valid polygon (the domain layer's geometric minimum of 3). Drives the
  /// boundary step's "Continue" button.
  public var hasMinimumBoundaryVertices: Bool {
    boundaryVertices.count >= Self.minimumBoundaryVertexCount
  }

  /// Appends a new vertex at the end of the ring being drawn.
  public func addVertex(_ coordinate: Coordinate) {
    boundaryVertices.append(coordinate)
  }

  /// Moves the vertex at `index` to `coordinate` (e.g. after a drag). A
  /// stale or out-of-range index is a no-op rather than a crash.
  public func moveVertex(at index: Int, to coordinate: Coordinate) {
    guard boundaryVertices.indices.contains(index) else { return }
    boundaryVertices[index] = coordinate
  }

  /// Removes the vertex at `index`. A stale or out-of-range index is a
  /// no-op rather than a crash.
  public func removeVertex(at index: Int) {
    guard boundaryVertices.indices.contains(index) else { return }
    boundaryVertices.remove(at: index)
  }

  /// Removes the most recently added vertex. A no-op on an empty ring.
  public func undoLastVertex() {
    guard !boundaryVertices.isEmpty else { return }
    boundaryVertices.removeLast()
  }

  /// Validates the current vertices as a closed ring without saving —
  /// invoked when the user taps the first vertex to close the shape while
  /// drawing (``BoundaryDrawingMapView``). Surfaces problems (self-
  /// intersection, an out-of-UK vertex, a duplicate point) immediately via
  /// ``error``; below the minimum vertex count this is a silent no-op since
  /// there's nothing yet to validate. Mirrors
  /// `WatchZoneEditorViewModel.finishDrawing()`.
  public func finishDrawing() {
    guard hasMinimumBoundaryVertices else { return }
    do {
      _ = try WatchZoneBoundary(vertices: boundaryVertices)
      error = nil
    } catch {
      handleError(error)
    }
  }

  /// Surfaces the in-wizard custom-shape paywall when the user taps the
  /// upsell on the radius step.
  public func requestCustomShapeUpgrade() {
    isCustomShapeUpsellPresented = true
  }

  /// Re-enters the drawing step for a tier that already allows custom
  /// shapes — the way back in after ``goBack()`` leaves ``boundaryDrawing``,
  /// since the upsell card that reaches it hides once already entitled.
  public func selectCustomShape() {
    guard canDrawCustomShape, currentStep == .radiusPicker else { return }
    shapeMode = .custom
    currentStep = .boundaryDrawing
  }

  /// Called when the custom-shape paywall sheet dismisses. Re-resolves the
  /// tier via the same ``onUpgradeFlowCompleted`` hook as
  /// ``reconcileTierAfterUpgrade()``, then swaps `.radiusPicker` for
  /// `.boundaryDrawing` in place if the purchase unlocked custom shapes —
  /// preserving the already-entered postcode/geocode (tc-w3cb.3).
  public func reconcileTierAfterCustomShapeUpgrade() async {
    await onUpgradeFlowCompleted?()
    if currentStep == .radiusPicker, canDrawCustomShape {
      shapeMode = .custom
      currentStep = .boundaryDrawing
    }
  }

  /// Builds a custom-shape ``WatchZone`` from ``boundaryVertices`` and
  /// advances to the notification step — the ``boundaryDrawing`` step's
  /// counterpart to ``confirmRadius()``. The centre and radius are derived
  /// from the boundary's centroid and enclosing radius, exactly as
  /// `WatchZoneEditorViewModel.save()` does in `.custom` mode, so every
  /// existing circle-shaped read path (map centring, list rows) keeps
  /// working unchanged.
  public func confirmBoundary() {
    guard geocodedCoordinate != nil, hasMinimumBoundaryVertices else { return }
    do {
      let boundary = try WatchZoneBoundary(vertices: boundaryVertices)
      let centroid = boundary.centroid
      let centre = try Coordinate(latitude: centroid.latitude, longitude: centroid.longitude)
      let radius = boundary.enclosingRadiusMetres
      let zone: WatchZone
      if let prefillZoneName {
        zone = try WatchZone(
          name: prefillZoneName, centre: centre, radiusMetres: radius, boundary: boundary)
      } else if let postcode = validatedPostcode {
        zone = try WatchZone(
          postcode: postcode, centre: centre, radiusMetres: radius, boundary: boundary)
      } else {
        return
      }
      createdWatchZone = zone
      currentStep = .notificationPermission
    } catch {
      handleError(error)
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
