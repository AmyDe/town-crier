import TownCrierDomain

/// A lightweight value representing a single pin on the map.
public struct MapAnnotationItem: Identifiable, Sendable {
  public let id: String
  public let applicationId: PlanningApplicationId
  public let latitude: Double
  public let longitude: Double
  public let status: ApplicationStatus
  public let title: String
  public let address: String

  public init(application: PlanningApplication, coordinate: Coordinate) {
    self.id = application.id.value
    self.applicationId = application.id
    self.latitude = coordinate.latitude
    self.longitude = coordinate.longitude
    self.status = application.status
    self.title = application.description
    self.address = application.address
  }
}
