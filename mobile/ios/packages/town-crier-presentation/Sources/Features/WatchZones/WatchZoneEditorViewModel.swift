import Combine
import Foundation
import TownCrierDomain

/// Whether a watch zone is drawn as a circle (radius-based, the default) or a
/// custom polygon shape (GH#1031). Free tier is locked to `.circle`;
/// Personal/Pro may switch to `.custom` — see
/// ``WatchZoneEditorViewModel/canDrawCustomShape``.
public enum WatchZoneShapeMode: Hashable, Sendable {
  case circle
  case custom
}

/// Drives create/edit of a single watch zone with postcode geocoding and tier-based radius limits.
@MainActor
public final class WatchZoneEditorViewModel: ObservableObject, EntitlementGatingViewModel {
  @Published public var nameInput: String = ""
  @Published public var postcodeInput: String = ""
  @Published public var selectedRadiusMetres: Double = 1000
  @Published public var pushEnabled: Bool = true
  @Published public var emailInstantEnabled: Bool = true
  @Published public private(set) var geocodedCoordinate: Coordinate?
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?

  /// Circle (radius-based) vs. custom polygon shape (GH#1031). Defaults to
  /// `.circle`; populated from the edited zone's `boundary` when present.
  @Published public private(set) var shapeMode: WatchZoneShapeMode = .circle

  /// The custom-shape ring being drawn, as an OPEN sequence of vertices in
  /// drop order — NOT closed (the first vertex is not repeated at the end).
  /// ``BoundaryDrawingMapView`` renders these as pins plus a preview
  /// polygon/polyline; ``save()`` closes and validates the ring via
  /// `WatchZoneBoundary.init` when ``shapeMode`` is `.custom`.
  @Published public private(set) var boundaryVertices: [Coordinate] = []

  /// The pre-canned notification filter selected for this zone, or `nil` for
  /// unfiltered (GH#1098). Defaults to `nil`; populated from the edited
  /// zone's `filterKey` when present.
  @Published public private(set) var selectedFilterKey: WatchZoneFilterKey?

  /// Mirrors `WatchZoneBoundary`'s geometric minimum (3 distinct vertices
  /// are required to form a polygon) so the UI can gate "finish drawing"
  /// without constructing (and discarding) a throwaway boundary just to
  /// check the count.
  private static let minimumBoundaryVertexCount = 3

  /// Drives the in-editor subscription upsell sheet. Non-nil while the gate is
  /// presented for the given entitlement; cleared when the sheet dismisses.
  @Published public var entitlementGate: Entitlement?

  var onSave: ((WatchZone) -> Void)?

  /// Invoked when a save fails because the user has hit their tier's watch-zone
  /// quota (tc-gpjk). The coordinator dismisses the editor and presents the
  /// subscription paywall — no inline error is shown for this case.
  public var onUpgradeRequired: (() -> Void)?

  public let isEditing: Bool

  private let geocoder: PostcodeGeocoder
  private let repository: WatchZoneRepository
  private let limits: WatchZoneLimits
  private let tier: SubscriptionTier
  private let existingId: WatchZoneId?

  public init(
    geocoder: PostcodeGeocoder,
    repository: WatchZoneRepository,
    tier: SubscriptionTier,
    editing zone: WatchZone? = nil
  ) {
    self.geocoder = geocoder
    self.repository = repository
    self.limits = WatchZoneLimits(tier: tier)
    self.tier = tier
    self.isEditing = zone != nil
    self.existingId = zone?.id

    if let zone {
      self.nameInput = zone.name
      self.selectedRadiusMetres = zone.radiusMetres
      self.geocodedCoordinate = zone.centre
      self.pushEnabled = zone.pushEnabled
      self.emailInstantEnabled = zone.emailInstantEnabled
      if let boundary = zone.boundary {
        self.shapeMode = .custom
        // The ring is closed (last vertex repeats the first); the editor
        // works with the open sequence and re-closes on save.
        self.boundaryVertices = Array(boundary.vertices.dropLast())
      }
      self.selectedFilterKey = zone.filterKey
    }
  }

  public var isPostcodeFieldVisible: Bool {
    !isEditing
  }

  public var maxRadiusMetres: Double {
    limits.maxRadiusMetres
  }

  /// Whether the user's tier still has radius headroom to unlock — true for any
  /// tier below the top (Pro, 10 km). Drives the "Unlock larger zones" chip
  /// shown beneath the radius slider, mirroring the onboarding wizard (tc-w3cb.3).
  public var canUnlockLargerRadius: Bool {
    tier < .pro
  }

  /// Proactive UI gate exposing the session's tier to entitlement-aware controls
  /// (e.g. ``GatedToggle``) without a network round-trip.
  public var featureGate: FeatureGate {
    FeatureGate(tier: tier)
  }

  /// The entitlement that the per-zone instant push/email toggles are gated behind.
  ///
  /// Instant alerts (push and instant email) are paid-only; free accounts receive
  /// the weekly digest only and the server never delivers instant alerts to them
  /// (tc-bd6i). All paid entitlements travel together in ``EntitlementMap``, so any
  /// paid-only entitlement correctly distinguishes free (locked) from paid (open).
  public var instantAlertEntitlement: Entitlement {
    .statusChangeAlerts
  }

  /// The notifications section is always shown. For free accounts the instant
  /// push/email toggles render locked with an upgrade prompt via ``GatedToggle``;
  /// for Personal/Pro they are fully interactive (tc-bd6i).
  public var areNotificationTogglesVisible: Bool {
    true
  }

  /// Surfaces the in-editor subscription upsell when a free-tier user taps a locked
  /// instant-alert toggle. Routes through the entitlement gate so the editor stays
  /// open and the user can dismiss without losing their work.
  public func requestInstantAlertUpgrade() {
    entitlementGate = instantAlertEntitlement
  }

  /// Invoked when the user taps "View Plans" in the upsell sheet — hands off to the
  /// coordinator (via ``onUpgradeRequired``) to present the subscription screen.
  public func viewPlans() {
    onUpgradeRequired?()
  }

  /// Whether to surface the "this zone may produce lots of notifications" callout
  /// (tc-1zb7). Triggered just above the free tier's 2 km cap so only paid tiers
  /// see it — see `LargeRadiusWarningView` for the threshold rationale.
  public var showsLargeRadiusWarning: Bool {
    selectedRadiusMetres >= LargeRadiusWarning.thresholdMetres
  }

  // MARK: - Shape mode (GH#1031)

  /// Whether the current tier may draw a custom-shape zone at all — gates
  /// showing the Circle/Custom segmented control in the View. Free tier is
  /// always `false`; Personal/Pro are `true`. Mirrors the server's
  /// `SubscriptionTier.AllowsCustomBoundary()` via `WatchZoneLimits`.
  public var canDrawCustomShape: Bool {
    limits.allowsCustomBoundary
  }

  /// Whether ``boundaryVertices`` currently has enough points to form a
  /// valid polygon (the domain layer's geometric minimum of 3). Drives the
  /// Save button's enabled state while ``shapeMode`` is `.custom`.
  public var canFinishCustomShape: Bool {
    boundaryVertices.count >= Self.minimumBoundaryVertexCount
  }

  /// True when editing an existing custom-shape zone on a tier that can no
  /// longer draw one (a Free-tier user who downgraded after saving a
  /// polygon zone — GH#1031's paused-zone posture keeps the zone editable,
  /// never silently converted). ``shapeMode`` is `.custom` here because it
  /// reflects the zone's actual stored shape, not the tier's entitlement —
  /// see the `editing:` initializer — so the View must check this
  /// separately to keep the drawing UI out of a Free tier's reach.
  public var isCustomShapeLocked: Bool {
    shapeMode == .custom && !canDrawCustomShape
  }

  /// Switches between Circle and Custom. A request to switch to `.custom`
  /// on a tier that doesn't allow it (``canDrawCustomShape`` is `false`) is
  /// silently ignored — defense in depth behind the View only offering the
  /// control on paid tiers.
  public func selectShapeMode(_ mode: WatchZoneShapeMode) {
    guard mode != .custom || canDrawCustomShape else { return }
    shapeMode = mode
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
  /// ``error`` rather than waiting for Save; below the minimum vertex count
  /// this is a silent no-op since there's nothing yet to validate.
  public func finishDrawing() {
    guard canFinishCustomShape else { return }
    do {
      _ = try WatchZoneBoundary(vertices: boundaryVertices)
      error = nil
    } catch {
      handleError(error)
    }
  }

  // MARK: - Pre-canned filter (GH#1098)

  /// Whether the current tier may set a pre-canned filter at all -- gates
  /// showing the filter picker row in the View. Pro only; Free/Personal are
  /// always `false`. Mirrors the server's
  /// `SubscriptionTier.AllowsWatchZoneFilter()` via `WatchZoneLimits`.
  public var canSetWatchZoneFilter: Bool {
    limits.allowsWatchZoneFilter
  }

  /// True when editing an existing filtered zone on a tier that can no
  /// longer set one (a downgraded Pro user who set a filter before
  /// downgrading) -- same posture as ``isCustomShapeLocked``: the filter
  /// stays applied and visible, never silently cleared, but the picker is
  /// unreachable.
  public var isFilterLocked: Bool {
    selectedFilterKey != nil && !canSetWatchZoneFilter
  }

  /// Sets or clears the selected filter. Clearing (`nil`) is always allowed
  /// regardless of tier, matching the server's "clearing never requires an
  /// entitlement check". Setting a non-nil value while ineligible
  /// (``canSetWatchZoneFilter`` is `false`) is a silent no-op, matching
  /// ``selectShapeMode``'s defence-in-depth pattern.
  public func selectFilterKey(_ key: WatchZoneFilterKey?) {
    guard key == nil || canSetWatchZoneFilter else { return }
    selectedFilterKey = key
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
      geocodedCoordinate = try await geocoder.geocode(postcode)
      if nameInput.trimmingCharacters(in: .whitespaces).isEmpty {
        nameInput = postcode.value
      }
    } catch {
      handleError(error)
    }

    isLoading = false
  }

  /// Persists the zone. Returns `true` on success so the View dismisses only
  /// when the save actually succeeded.
  ///
  /// In `.circle` mode, behaves exactly as before: the geocoded coordinate
  /// and (clamped) radius drive a circular zone with no boundary. In
  /// `.custom` mode, ``boundaryVertices`` is closed and validated into a
  /// `WatchZoneBoundary`, and the zone's `centre`/`radiusMetres` are derived
  /// from the boundary's centroid/enclosing radius (per `WatchZone.boundary`'s
  /// doc comment) so every existing circle-shaped read path keeps working.
  ///
  /// On a quota breach (`DomainError.insufficientEntitlement`) the editor routes
  /// to the subscription paywall via `onUpgradeRequired` and leaves `error`
  /// unset — the coordinator closes the sheet. All other failures (including an
  /// invalid custom shape) set `error` so the inline error section is shown and
  /// the editor stays open.
  @discardableResult
  public func save() async -> Bool {
    error = nil

    let boundary: WatchZoneBoundary?
    let centre: Coordinate
    let radius: Double

    switch shapeMode {
    case .circle:
      guard let coordinate = geocodedCoordinate else { return false }
      boundary = nil
      centre = coordinate
      radius = limits.clampRadius(selectedRadiusMetres)
    case .custom:
      guard canFinishCustomShape else { return false }
      do {
        let builtBoundary = try WatchZoneBoundary(vertices: boundaryVertices)
        let centroid = builtBoundary.centroid
        centre = try Coordinate(latitude: centroid.latitude, longitude: centroid.longitude)
        radius = builtBoundary.enclosingRadiusMetres
        boundary = builtBoundary
      } catch {
        handleError(error)
        return false
      }
    }

    do {
      let zone = try WatchZone(
        id: existingId ?? WatchZoneId(),
        name: nameInput,
        centre: centre,
        radiusMetres: radius,
        pushEnabled: pushEnabled,
        emailInstantEnabled: emailInstantEnabled,
        boundary: boundary,
        filterKey: selectedFilterKey
      )
      if isEditing {
        try await repository.update(zone)
      } else {
        try await repository.save(zone)
      }
      onSave?(zone)
      return true
    } catch DomainError.insufficientEntitlement {
      onUpgradeRequired?()
      return false
    } catch {
      handleError(error)
      return false
    }
  }
}
