import Foundation
import TownCrierDomain

/// Drives the post-signup "Add your other areas" conversion sheet (GH#879
/// Phase 5): offers the device-local zones the onboarding wizard did NOT
/// already convert (that conversion happens separately, via the wizard's own
/// `WatchZoneRepository.save` call) for server-side creation.
///
/// Presented once immediately after `completeOnboarding()` when unconverted
/// zones remain, and again from the authenticated Zones tab's dismissible
/// row for as long as any remain
/// (see `AppCoordinator+DeviceLocalZoneConversion`).
@MainActor
public final class DeviceLocalZoneConversionViewModel: ObservableObject {
  @Published public private(set) var zones: [DeviceLocalZone]
  @Published public private(set) var isConverting = false
  @Published public internal(set) var error: DomainError?

  private let watchZoneRepository: WatchZoneRepository
  private let deviceLocalZoneRepository: DeviceLocalZoneRepository

  /// Fired when a save hits the tier's watch-zone quota
  /// (`DomainError.insufficientEntitlement`). Conversion stops here — the
  /// triggering zone and everything after it in `zones` stay in local
  /// storage, untouched.
  var onInsufficientEntitlement: (() -> Void)?
  /// Fired when a pass over `zones` finishes — either every zone converted,
  /// or the user dismissed without converting the rest. Either way the sheet
  /// should close.
  var onFinished: (() -> Void)?

  public init(
    zones: [DeviceLocalZone],
    watchZoneRepository: WatchZoneRepository,
    deviceLocalZoneRepository: DeviceLocalZoneRepository
  ) {
    self.zones = zones
    self.watchZoneRepository = watchZoneRepository
    self.deviceLocalZoneRepository = deviceLocalZoneRepository
  }

  /// Converts every remaining zone sequentially, in list order — never in
  /// parallel, so a mid-list quota breach leaves everything after it
  /// untouched. Each success is deleted from local storage immediately.
  /// Reuses exactly the wizard/authed-editor path for zone creation: no
  /// `authorityId` is set, so the server resolves it from the coordinate.
  public func convertAll() async {
    isConverting = true
    error = nil
    for zone in zones {
      do {
        let watchZone = try WatchZone(
          name: zone.name, centre: zone.centre, radiusMetres: zone.radiusMetres)
        try await watchZoneRepository.save(watchZone)
        deviceLocalZoneRepository.delete(zone.id)
        zones.removeAll { $0.id == zone.id }
      } catch DomainError.insufficientEntitlement {
        isConverting = false
        onInsufficientEntitlement?()
        return
      } catch let domainError as DomainError {
        isConverting = false
        error = domainError
        return
      } catch {
        isConverting = false
        self.error = .unexpected(error.localizedDescription)
        return
      }
    }
    isConverting = false
    onFinished?()
  }

  /// The user declined to convert the remaining zones this time — they stay
  /// in local storage and the dismissible Zones-tab row offers them again
  /// (never silently discard user-created data).
  public func dismiss() {
    onFinished?()
  }
}
