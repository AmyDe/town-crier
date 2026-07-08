import Foundation
import TownCrierDomain

/// Persists the anonymous browse session to UserDefaults as a single
/// JSON-encoded blob (GH#868 Phase 3). Device-local, like
/// ``UserDefaultsOnboardingRepository`` — reinstalling the app resets it.
public final class UserDefaultsAnonymousBrowseStateRepository: AnonymousBrowseStateRepository,
  @unchecked Sendable {
  private let defaults: UserDefaults
  private let key = "anonymousBrowseState"
  private let decoder = JSONDecoder()
  private let encoder = JSONEncoder()

  public init(defaults: UserDefaults = .standard) {
    self.defaults = defaults
  }

  public func load() -> AnonymousBrowseState? {
    guard let data = defaults.data(forKey: key) else { return nil }
    guard let stored = try? decoder.decode(StoredState.self, from: data) else { return nil }
    guard let postcode = try? Postcode(stored.postcode) else { return nil }
    guard let coordinate = try? Coordinate(latitude: stored.latitude, longitude: stored.longitude)
    else { return nil }
    return AnonymousBrowseState(
      postcode: postcode,
      coordinate: coordinate,
      radiusMetres: stored.radiusMetres,
      createdAt: Date(timeIntervalSince1970: stored.createdAt)
    )
  }

  public func save(_ state: AnonymousBrowseState) {
    let stored = StoredState(
      postcode: state.postcode.value,
      latitude: state.coordinate.latitude,
      longitude: state.coordinate.longitude,
      radiusMetres: state.radiusMetres,
      createdAt: state.createdAt.timeIntervalSince1970
    )
    guard let data = try? encoder.encode(stored) else { return }
    defaults.set(data, forKey: key)
  }

  public func clear() {
    defaults.removeObject(forKey: key)
  }

  /// Flat, versionless wire shape for the persisted blob — deliberately not
  /// `AnonymousBrowseState` itself, so the domain type never needs to conform
  /// to `Codable` (Domain stays free of persistence concerns).
  private struct StoredState: Codable {
    let postcode: String
    let latitude: Double
    let longitude: Double
    let radiusMetres: Double
    let createdAt: TimeInterval

    private enum CodingKeys: String, CodingKey {
      case postcode, latitude, longitude, radiusMetres, createdAt
    }

    init(postcode: String, latitude: Double, longitude: Double, radiusMetres: Double, createdAt: TimeInterval) {
      self.postcode = postcode
      self.latitude = latitude
      self.longitude = longitude
      self.radiusMetres = radiusMetres
      self.createdAt = createdAt
    }

    /// Custom decode so a blob saved before `radiusMetres` existed (GH#868
    /// Phase 3, pre-refinement) still loads — a live TestFlight tester's
    /// already-persisted anonymous session shouldn't be silently dropped by
    /// this change. Falls back to the same 2000m default
    /// `AnonymousBrowseState` itself seeds.
    init(from decoder: Decoder) throws {
      let container = try decoder.container(keyedBy: CodingKeys.self)
      postcode = try container.decode(String.self, forKey: .postcode)
      latitude = try container.decode(Double.self, forKey: .latitude)
      longitude = try container.decode(Double.self, forKey: .longitude)
      createdAt = try container.decode(TimeInterval.self, forKey: .createdAt)
      radiusMetres = try container.decodeIfPresent(Double.self, forKey: .radiusMetres) ?? 2000
    }
  }
}
